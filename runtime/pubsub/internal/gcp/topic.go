package gcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
)

type Manager struct {
	ctxs         *utils.Contexts
	runtime      *config.Runtime
	pushRegistry types.PushEndpointRegistry

	clientOnce sync.Once
	_client    *pubsub.Client // access via getClient()
}

func NewManager(ctxs *utils.Contexts, runtime *config.Runtime, pushRegistry types.PushEndpointRegistry) *Manager {
	return &Manager{ctxs: ctxs, runtime: runtime, pushRegistry: pushRegistry}
}

type topic struct {
	mgr      *Manager
	client   *pubsub.Client
	gcpTopic *pubsub.Topic
	topicCfg *config.PubsubTopic
}

func (mgr *Manager) ProviderName() string { return "gcp" }

func (mgr *Manager) Matches(cfg *config.PubsubProvider) bool {
	return cfg.GCP != nil
}

func (mgr *Manager) NewTopic(_ *config.PubsubProvider, staticCfg types.TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	client := mgr.getClient()
	gcpTopic := client.TopicInProject(runtimeCfg.ProviderName, runtimeCfg.GCP.ProjectID)

	// Enable message ordering if we have an ordering key set
	gcpTopic.EnableMessageOrdering = staticCfg.OrderingAttribute != ""

	// Check we have permissions to interact with the given topic
	// (note: the call to Topic() above only creates the object, it doesn't verify that we have permissions to interact with it)
	_, err := gcpTopic.Config(mgr.ctxs.Connection)
	if err != nil {
		panic(fmt.Sprintf("pubsub topic %s status call failed: %s", runtimeCfg.EncoreName, err))
	}

	return &topic{mgr, client, gcpTopic, runtimeCfg}
}

func (t *topic) PublishMessage(ctx context.Context, orderingKey string, attrs map[string]string, data []byte) (id string, err error) {
	gcpMsg := &pubsub.Message{
		Data:        data,
		Attributes:  attrs,
		OrderingKey: orderingKey,
	}

	// Attempt to publish the message
	return t.gcpTopic.Publish(ctx, gcpMsg).Get(ctx)
}

func (t *topic) Subscribe(logger *zerolog.Logger, maxConcurrency int, ackDeadline time.Duration, retryPolicy *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	if subCfg.PushOnly && subCfg.ID == "" {
		panic("push-only subscriptions must have a subscription ID")
	}
	gcpCfg := subCfg.GCP
	if gcpCfg == nil {
		panic("GCP subscriptions must have GCP-specific configuration provided, got nil")
	}

	// If we have a subscription ID, register a push endpoint for it
	if subCfg.ID != "" {
		if gcpCfg.PushServiceAccount != "" {
			t.mgr.registerPushEndpoint(logger, subCfg, f)
		} else if subCfg.PushOnly {
			panic("push-only subscriptions require a push service account to be configured for the PubSub server config")
		}
	}

	// If we're not push only, then also set up the subscription
	if !subCfg.PushOnly {
		// Create the subscription object (and then check it exists on GCP's side)
		subscription := t.client.SubscriptionInProject(subCfg.ProviderName, gcpCfg.ProjectID)

		if maxConcurrency == 0 {
			maxConcurrency = 1000 // FIXME(domblack): This retains the old behaviour, but allows user customisation - in a future release we should remove this
		}

		// Set the concurrency
		subscription.ReceiveSettings.MaxOutstandingMessages = maxConcurrency
		subscription.ReceiveSettings.NumGoroutines = utils.Clamp(maxConcurrency, 1, maxConcurrency)

		// Double-check that the subscription exists at initialization
		// to guard against configuration issues.
		{
			// Use a separate context to check if the subscription exists,
			// since this is happening during initialization, and causes a race
			// with Cloud Run as it tries to determine if the new revision is healthy.
			existsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			exists, err := subscription.Exists(existsCtx)
			if err != nil {
				panic(fmt.Sprintf("pubsub subscription %s for topic %s status call failed: %s", subCfg.EncoreName, t.topicCfg.EncoreName, err))
			}
			if !exists {
				panic(fmt.Sprintf("pubsub subscription %s for topic %s does not exist in GCP", subCfg.EncoreName, t.topicCfg.EncoreName))
			}
		}

		// Start the subscription
		go func() {
			for t.mgr.ctxs.Fetch.Err() == nil {
				// Subscribe to the topic to receive messages
				err := subscription.Receive(t.mgr.ctxs.Fetch, func(_ context.Context, msg *pubsub.Message) {
					deliveryAttempt := 1
					if msg.DeliveryAttempt != nil {
						deliveryAttempt = *msg.DeliveryAttempt
					}

					// Create a context from the handler context with a deadline of the ackdeadline
					ctx, cancel := context.WithTimeout(t.mgr.ctxs.Handler, ackDeadline)
					defer cancel()

					var result *pubsub.AckResult
					if err := f(ctx, msg.ID, msg.PublishTime, deliveryAttempt, msg.Attributes, msg.Data); err != nil {
						result = msg.NackWithResult()
					} else {
						result = msg.AckWithResult()
					}

					res, err := result.Get(t.mgr.ctxs.Connection)
					if err != nil {
						logger.Warn().Err(err).Str("msg_id", msg.ID).Msg("failed to ack/nack message")
					} else {
						switch res {
						case pubsub.AcknowledgeStatusSuccess:
							return
						case pubsub.AcknowledgeStatusPermissionDenied:
							logger.Error().Str("msg_id", msg.ID).Msg("failed to ack/nack message due to permissions")
						case pubsub.AcknowledgeStatusFailedPrecondition:
							logger.Error().Str("msg_id", msg.ID).Msg("failed to ack/nack message due to precondition")
						case pubsub.AcknowledgeStatusInvalidAckID:
							logger.Error().Str("msg_id", msg.ID).Msg("failed to ack/nack message due to invalid ack ID")
						default:
							logger.Error().Str("msg_id", msg.ID).Msg("failed to ack/nack message due to unknown error")
						}
					}
				})

				// If there was an error and we're not shutting down, log it and then sleep for a bit before trying again
				if err != nil && t.mgr.ctxs.Fetch.Err() == nil {
					logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
					time.Sleep(5 * time.Second)
				}
			}
		}()
	}
}
