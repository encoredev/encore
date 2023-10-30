package azure

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
	"encore.dev/types/uuid"
)

const RetryCountAttribute = "encore-retry-count"
const TargetSubAttribute = "encore-target-sub"

type Manager struct {
	ctxs *utils.Contexts

	clientMu sync.RWMutex
	_clients map[string]*azservicebus.Client // access via getClient()
}

func NewManager(ctxs *utils.Contexts) *Manager {
	return &Manager{ctxs: ctxs, _clients: map[string]*azservicebus.Client{}}
}

func (mgr *Manager) ProviderName() string { return "azure" }

func (mgr *Manager) Matches(cfg *config.PubsubProvider) bool {
	return cfg.Azure != nil
}

type topic struct {
	mgr        *Manager
	client     *azservicebus.Client
	topicCfg   *config.PubsubTopic
	senderOnce sync.Once
	_sender    *azservicebus.Sender
}

var _ types.TopicImplementation = (*topic)(nil)

func (mgr *Manager) NewTopic(providerCfg *config.PubsubProvider, _ types.TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	client := mgr.getClient(providerCfg.Azure)
	return &topic{mgr: mgr, client: client, topicCfg: runtimeCfg}
}

func (t *topic) sender() *azservicebus.Sender {
	t.senderOnce.Do(func() {
		sender, err := t.client.NewSender(t.topicCfg.ProviderName, nil)
		if err != nil {
			panic(fmt.Sprintf("failed to create pubsub sender for topic %s: %s", t.topicCfg.EncoreName, err))
		}
		t._sender = sender
	})
	return t._sender
}

func (t *topic) PublishMessage(ctx context.Context, groupingKey string, attrs map[string]string, data []byte) (id string, err error) {

	messageID, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("failed to generate message ID: %v", err.Error())
	}
	msg := &azservicebus.Message{
		MessageID:             to.Ptr(messageID.String()),
		Body:                  data,
		ApplicationProperties: map[string]interface{}{},
	}
	for k, v := range attrs {
		msg.ApplicationProperties[k] = v
	}

	// Attempt to publish the message
	err = t.sender().SendMessage(ctx, msg, nil)
	return *msg.MessageID, err
}

func (t *topic) scheduleRetry(subName string, msg *azservicebus.ReceivedMessage, backoff time.Duration) error {
	retryCount, _ := strconv.ParseInt(fmt.Sprintf("%v", msg.ApplicationProperties[RetryCountAttribute]), 10, 64)
	msg.ApplicationProperties[RetryCountAttribute] = retryCount + 1
	msg.ApplicationProperties[TargetSubAttribute] = subName

	reMsg := &azservicebus.Message{
		ApplicationProperties: msg.ApplicationProperties,
		Body:                  msg.Body,
		ContentType:           msg.ContentType,
		CorrelationID:         msg.CorrelationID,
		MessageID:             &msg.MessageID,
		PartitionKey:          msg.PartitionKey,
		ReplyTo:               msg.ReplyTo,
		ReplyToSessionID:      msg.ReplyToSessionID,
		SessionID:             msg.SessionID,
		Subject:               msg.Subject,
		TimeToLive:            msg.TimeToLive,
		To:                    msg.To,
	}
	_, err := t.sender().ScheduleMessages(
		t.mgr.ctxs.Connection, []*azservicebus.Message{reMsg}, time.Now().Add(backoff), nil)
	return err
}

func (t *topic) processMessage(
	ctx context.Context,
	logger *zerolog.Logger, receiver *azservicebus.Receiver, ackDeadline time.Duration, subCfg *config.PubsubSubscription,
	msg *azservicebus.ReceivedMessage, rp *types.RetryPolicy, f types.RawSubscriptionCallback) (err error) {

	ctx, cancel := context.WithTimeout(ctx, ackDeadline)
	defer cancel()

	attrs := make(map[string]string, len(msg.ApplicationProperties))
	for k, v := range msg.ApplicationProperties {
		attrs[k] = fmt.Sprintf("%v", v)
	}
	retryCount, _ := strconv.ParseInt(fmt.Sprintf("%v", msg.ApplicationProperties[RetryCountAttribute]), 10, 64)
	deliveryAttempt := retryCount + 1
	err = f(ctx, msg.MessageID, *msg.EnqueuedTime, int(deliveryAttempt), attrs, msg.Body)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to process messsage")
		shouldRetry, backoff := utils.GetDelay(
			rp.MaxRetries, rp.MinBackoff, rp.MaxBackoff, uint16(deliveryAttempt))
		if !shouldRetry {
			logger.Warn().Msg("deadlettering msg")
			err = receiver.DeadLetterMessage(t.mgr.ctxs.Connection, msg, &azservicebus.DeadLetterOptions{
				ErrorDescription:   to.Ptr(fmt.Sprintf("failed to process message after %v retries", deliveryAttempt)),
				Reason:             to.Ptr("ExhaustedRetries"),
				PropertiesToModify: map[string]interface{}{RetryCountAttribute: 0},
			})
		} else {
			logger.Warn().Msgf("scheduling msg retry in %v (attempt %v)", backoff, deliveryAttempt)
			err = t.scheduleRetry(subCfg.ProviderName, msg, backoff)
		}
	}
	// if err == nil we have either successfully processed the message or we have scheduled/deadlettered it
	if err == nil {
		err = receiver.CompleteMessage(t.mgr.ctxs.Connection, msg, nil)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to complete message")
		}
	}
	return err
}

func (t *topic) Subscribe(logger *zerolog.Logger, maxConcurrency int, ackDeadline time.Duration, retryPolicy *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	receiver, err := t.client.NewReceiverForSubscription(t.topicCfg.ProviderName, subCfg.ProviderName, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create pubsub receiver for subscription %s: %s", subCfg.EncoreName, err))
	}

	if maxConcurrency == 0 {
		maxConcurrency = 1 // FIXME(domblack): This retains the old behaviour, but allows user customisation - in a future release we should remove this
	}

	// Start the subscription
	go func() {
		for t.mgr.ctxs.Fetch.Err() == nil {
			err := utils.WorkConcurrently(
				t.mgr.ctxs, maxConcurrency, 0,
				func(ctx context.Context, maxToFetch int) ([]*azservicebus.ReceivedMessage, error) {
					// Subscribe to the topic to receive messages
					messages, err := receiver.ReceiveMessages(ctx, maxToFetch, nil)
					if err != nil {
						return nil, err
					}

					return messages, nil
				},
				func(ctx context.Context, work *azservicebus.ReceivedMessage) error {
					return t.processMessage(ctx, logger, receiver, ackDeadline, subCfg, work, retryPolicy, f)
				},
			)

			// If there was an error and we're not shutting down, log it and then sleep for a bit before trying again
			if err != nil && t.mgr.ctxs.Fetch.Err() == nil {
				logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		}

	}()

}
