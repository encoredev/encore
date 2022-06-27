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
	client    *pubsub.Client
	gcpTopic  *pubsub.Topic
	serverCfg *config.GCPPubSubServer
	topicCfg  *config.PubsubTopic
}

func NewTopic(cfg *config.GCPPubSubServer, topicCfg *config.PubsubTopic) types.TopicImplementation {
	// Create the topic
	client := getClient(cfg)
	gcpTopic := client.Topic(topicCfg.CloudName)

	// Enable message ordering if we have an ordering key set
	gcpTopic.EnableMessageOrdering = topicCfg.OrderingKey != ""

	// Check we have permissions to interact with the given topic
	// (note: the call to Topic() above only creates the object, it doesn't verify that we have permissions to interact with it)
	_, err := gcpTopic.Config(ctx.App)
	if err != nil {
		panic(fmt.Sprintf("pubsub topic %s status call failed: %s", topicCfg.CloudName, err))
	}

	return &topic{client, gcpTopic, cfg, topicCfg}
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

func (t *topic) Subscribe(logger *zerolog.Logger, _ *types.SubscriptionConfig, subscriptionCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	if subscriptionCfg.PushOnly && subscriptionCfg.ResourceID == "" {
		panic("push-only subscriptions must have a resource ID")
	}

	// If we have a resource ID, let's register a push endpoint for it
	if subscriptionCfg.ResourceID != "" {
		if t.serverCfg.PushServiceAccount != "" {
			registerPushEndpoint(t.serverCfg, subscriptionCfg, f)
		} else if subscriptionCfg.PushOnly {
			panic("push-only subscriptions require a push service account to be configured for the PubSub server config")
		}
	}

	// If we're not push only, then let's also setup the subscription
	if !subscriptionCfg.PushOnly {
		// Create the subscription object (and then check it exists on GCP's side)
		subscription := t.client.Subscription(subscriptionCfg.CloudName)
		exists, err := subscription.Exists(ctx.App)
		if err != nil {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s status call failed: %s", subscriptionCfg.EncoreName, t.topicCfg.EncoreName, err))
		}
		if !exists {
			panic(fmt.Sprintf("pubsub subscription %s for topic %s does not exist in GCP", subscriptionCfg.EncoreName, t.topicCfg.EncoreName))
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
