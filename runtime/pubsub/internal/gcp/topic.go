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
	ctx          context.Context
	runtime      *config.Runtime
	pushRegistry types.PushEndpointRegistry

	clientOnce sync.Once
	_client    *pubsub.Client // access via getClient()
}

func NewManager(ctx context.Context, runtime *config.Runtime, pushRegistry types.PushEndpointRegistry) *Manager {
	return &Manager{ctx: ctx, runtime: runtime, pushRegistry: pushRegistry}
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
	_, err := gcpTopic.Config(mgr.ctx)
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

		// Set the concurrency
		subscription.ReceiveSettings.MaxOutstandingMessages = maxConcurrency
		subscription.ReceiveSettings.NumGoroutines = utils.Clamp(maxConcurrency, 1, maxConcurrency)

		exists, err := subscription.Exists(t.mgr.ctx)
		if err != nil {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s status call failed: %s", subCfg.EncoreName, t.topicCfg.EncoreName, err))
		}
		if !exists {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s does not exist in GCP", subCfg.EncoreName, t.topicCfg.EncoreName))
		}

		// Start the subscription
		go func() {
			for t.mgr.ctx.Err() == nil {
				// Subscribe to the topic to receive messages
				err := subscription.Receive(t.mgr.ctx, func(ctx context.Context, msg *pubsub.Message) {
					deliveryAttempt := 1
					if msg.DeliveryAttempt != nil {
						deliveryAttempt = *msg.DeliveryAttempt
					}

					if err := f(ctx, msg.ID, msg.PublishTime, deliveryAttempt, msg.Attributes, msg.Data); err != nil {
						msg.Nack()
					} else {
						msg.Ack()
					}
				})

				// If there was an error and we're not shutting down, log it and then sleep for a bit before trying again
				if err != nil && t.mgr.ctx.Err() == nil {
					logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
					time.Sleep(5 * time.Second)
				}
			}
		}()
	}
}
