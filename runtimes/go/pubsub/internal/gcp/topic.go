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

	clientsMu sync.Mutex                // clientsMu protects access to the clients map
	clients   map[string]*pubsub.Client // A map of project ID to pubsub client
}

func NewManager(ctxs *utils.Contexts, runtime *config.Runtime, pushRegistry types.PushEndpointRegistry) *Manager {
	return &Manager{ctxs: ctxs, runtime: runtime, pushRegistry: pushRegistry, clients: make(map[string]*pubsub.Client)}
}

type topic struct {
	mgr      *Manager
	gcpTopic *pubsub.Topic
	topicCfg *config.PubsubTopic
}

func (mgr *Manager) ProviderName() string { return "gcp" }

func (mgr *Manager) Matches(cfg *config.PubsubProvider) bool {
	return cfg.GCP != nil
}

func (mgr *Manager) NewTopic(_ *config.PubsubProvider, staticCfg types.TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	gcpTopic := mgr.getClientForProject(runtimeCfg.GCP.ProjectID).Topic(runtimeCfg.ProviderName)

	// Enable message ordering if we have an ordering key set
	gcpTopic.EnableMessageOrdering = staticCfg.OrderingAttribute != ""

	// Check we have permissions to interact with the given topic
	// (note: the call to Topic() above only creates the object, it doesn't verify that we have permissions to interact with it)
	_, err := gcpTopic.Config(mgr.ctxs.Connection)
	if err != nil {
		panic(fmt.Sprintf("pubsub topic %s status call failed: %s", runtimeCfg.EncoreName, err))
	}

	return &topic{mgr, gcpTopic, runtimeCfg}
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
		subscription := t.mgr.getClientForProject(gcpCfg.ProjectID).Subscription(subCfg.ProviderName)

		// Set the concurrency
		if maxConcurrency == 0 {
			maxConcurrency = 1000 // FIXME(domblack): This retains the old behaviour, but allows user customisation - in a future release we should remove this
		}
		subscription.ReceiveSettings.MaxOutstandingMessages = maxConcurrency

		// Start the subscription with the GCP library
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
