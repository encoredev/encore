package azure

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
	"encore.dev/types/uuid"
)

const RetryCountAttribute = "encore-retry-count"
const TargetSubAttribute = "encore-target-sub"

type Manager struct {
	ctx context.Context
	cfg *config.Config

	clientMu sync.RWMutex
	_clients map[string]*azservicebus.Client // access via getClient()
}

func NewManager(ctx context.Context, cfg *config.Config) *Manager {
	return &Manager{ctx: ctx, cfg: cfg, _clients: map[string]*azservicebus.Client{}}
}

type topic struct {
	mgr        *Manager
	client     *azservicebus.Client
	topicCfg   *config.PubsubTopic
	senderOnce sync.Once
	_sender    *azservicebus.Sender
}

var _ types.TopicImplementation = (*topic)(nil)

func (mgr *Manager) NewTopic(providerCfg *config.AzureServiceBusProvider, cfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	client := mgr.getClient(providerCfg)
	return &topic{mgr: mgr, client: client, topicCfg: cfg}
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

func (t *topic) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {

	messageID, err := uuid.NewV4()
	if err != nil {
		return "", errors.New("failed to generate message ID: %v" + err.Error())
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
		t.mgr.ctx, []*azservicebus.Message{reMsg}, time.Now().Add(backoff), nil)
	return err
}

func (t *topic) processMessage(
	logger *zerolog.Logger, receiver *azservicebus.Receiver, ackDeadline time.Duration, subCfg *config.PubsubSubscription,
	msg *azservicebus.ReceivedMessage, rp *types.RetryPolicy, f types.RawSubscriptionCallback) (err error) {

	ctx, cancel := context.WithTimeout(t.mgr.ctx, ackDeadline)
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
			err = receiver.DeadLetterMessage(t.mgr.ctx, msg, &azservicebus.DeadLetterOptions{
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
		err = receiver.CompleteMessage(t.mgr.ctx, msg, nil)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to complete message")
		}
	}
	return err
}

func (t *topic) Subscribe(logger *zerolog.Logger, ackDeadline time.Duration, rp *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	receiver, err := t.client.NewReceiverForSubscription(t.topicCfg.ProviderName, subCfg.ProviderName, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create pubsub receiver for subscription %s: %s", subCfg.EncoreName, err))
	}
	// Start the subscription
	go func() {
		for t.mgr.ctx.Err() == nil {
			// Subscribe to the topic to receive messages
			messages, err := receiver.ReceiveMessages(t.mgr.ctx, 1, nil)
			if len(messages) > 0 {
				err = t.processMessage(logger, receiver, ackDeadline, subCfg, messages[0], rp, f)
			}
			// If there was an error and we're not shutting down, log it and then sleep for a bit before trying again
			if err != nil && t.mgr.ctx.Err() == nil {
				logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
				time.Sleep(5 * time.Second)
			}
		}

	}()

}
