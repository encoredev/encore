package gcp

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog"

	"encore.dev/internal/ctx"
	"encore.dev/pubsub/internal/types"
	"encore.dev/runtime/config"
)

type topic struct {
	client   *pubsub.Client
	gcpTopic *pubsub.Topic
	topicCfg *config.PubsubTopic
}

func NewTopic(_ *config.GCPPubsubProvider, cfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	client := getClient()
	gcpTopic := client.TopicInProject(cfg.ProviderName, cfg.GCP.ProjectID)

	// Enable message ordering if we have an ordering key set
	gcpTopic.EnableMessageOrdering = cfg.OrderingKey != ""

	// Check we have permissions to interact with the given topic
	// (note: the call to Topic() above only creates the object, it doesn't verify that we have permissions to interact with it)
	_, err := gcpTopic.Config(ctx.App)
	if err != nil {
		panic(fmt.Sprintf("pubsub topic %s status call failed: %s", cfg.EncoreName, err))
	}

	return &topic{client, gcpTopic, cfg}
}

func (t *topic) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {
	gcpMsg := &pubsub.Message{
		Data:        data,
		Attributes:  attrs,
		OrderingKey: t.topicCfg.OrderingKey, // FIXME(domblack): this should be the ordering VALUE not column name
	}

	// Attempt to publish the message
	return t.gcpTopic.Publish(ctx, gcpMsg).Get(ctx)
}

func (t *topic) Subscribe(logger *zerolog.Logger, _ *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
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
			registerPushEndpoint(subCfg, f)
		} else if subCfg.PushOnly {
			panic("push-only subscriptions require a push service account to be configured for the PubSub server config")
		}
	}

	// If we're not push only, then also set up the subscription
	if !subCfg.PushOnly {
		// Create the subscription object (and then check it exists on GCP's side)
		subscription := t.client.SubscriptionInProject(subCfg.ProviderName, gcpCfg.ProjectID)
		exists, err := subscription.Exists(ctx.App)
		if err != nil {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s status call failed: %s", subCfg.EncoreName, t.topicCfg.EncoreName, err))
		}
		if !exists {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s does not exist in GCP", subCfg.EncoreName, t.topicCfg.EncoreName))
		}

		// Start the subscription
		go func() {
			for ctx.App.Err() == nil {
				// Subscribe to the topic to receive messages
				err := subscription.Receive(ctx.App, func(ctx context.Context, msg *pubsub.Message) {
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
				if err != nil && ctx.App.Err() == nil {
					logger.Warn().Err(err).Msg("pubsub subscription failed, retrying in 5 seconds")
					time.Sleep(5 * time.Second)
				}
			}
		}()
	}
}
