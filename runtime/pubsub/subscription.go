package pubsub

import (
	"context"
	"time"

	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/utils"
	"encore.dev/runtime"
	"encore.dev/runtime/config"
)

// Subscription represents a subscription to a Topic.
type Subscription[T any] struct{}

// Subscriber is a function reference
// The signature must be `func(context.Context, msg M) error` where M is either the
// message type of the topic or RawMessage
type Subscriber[T any] func(ctx context.Context, msg T) error

// NewSubscription is used to declare a Subscription to a topic. The passed in handler will be called
// for each message published to the topic.
//
// A call to NewSubscription can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The subscription name must be unique for that topic. Subscription names must be defined
// in kebab-case (lowercase alphanumerics and hyphen seperated).Once created and deployed never
// change the subscription name, or the topic name otherwise messages will be lost which
// could be inflight.
//
// Example:
//     type MyEvent struct {
//       Foo string
//     }
//
//     var MyTopic = pubsub.NewTopic[*MyEvent]("my-topic", pubsub.TopicConfig{
//       DeliveryGuarantee: pubsub.AtLeastOnce,
//     })
//
//     var Subscription = pubsub.NewSubscription(MyTopic, "my-subscription", HandleEvent, pubsub.SubscriptionConfig{
//       RetryPolicy: &pubsub.RetryPolicy { MaxRetries: 10 },
//     })
//
//     func HandleEvent(ctx context.Context, event *MyEvent) error {
//       rlog.Info("received foo")
//       return nil
//     }
func NewSubscription[T any](topic *Topic[T], name string, handler Subscriber[T], subscriptionCfg SubscriptionConfig) *Subscription[T] {
	if topic.topicCfg == nil || topic.topic == nil {
		panic("pubsub topic was not created using pubsub.NewTopic")
	}

	// Set default config values for missing values
	if subscriptionCfg.RetryPolicy == nil {
		subscriptionCfg.RetryPolicy = &RetryPolicy{
			MaxRetries: 100,
		}
	}
	if subscriptionCfg.RetryPolicy.MinRetryDelay < 0 {
		panic("MinRetryDelay cannot be negative")
	}
	if subscriptionCfg.RetryPolicy.MaxRetryDelay < 0 {
		panic("MaxRetryDelay cannot be negative")
	}
	subscriptionCfg.RetryPolicy.MinRetryDelay = utils.WithDefaultValue(subscriptionCfg.RetryPolicy.MinRetryDelay, 10*time.Second)
	subscriptionCfg.RetryPolicy.MaxRetryDelay = utils.WithDefaultValue(subscriptionCfg.RetryPolicy.MaxRetryDelay, 10*time.Minute)

	subscription, staticCfg := topic.getSubscriptionConfig(name)
	panicCatchWrapper := func(ctx context.Context, msg T) (err error) {
		defer func() {
			if err2 := recover(); err2 != nil {
				err = errs.B().Code(errs.Internal).Msgf("subscriber paniced: %s", err2).Err()
			}
		}()

		return handler(ctx, msg)
	}

	log := runtime.Logger().With().
		Str("service", staticCfg.Service.Name).
		Str("topic", topic.topicCfg.EncoreName).
		Str("subscription", name).
		Logger()

	// Subscribe to the topic
	topic.topic.Subscribe(&log, &subscriptionCfg, subscription, func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) (err error) {
		if !config.Cfg.Static.Testing {
			// Under test we're already inside an operation
			runtime.BeginOperation()
			defer runtime.FinishOperation()
		}

		msg, err := utils.UnmarshalMessage[T](attrs, data)
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to unmarshal message")
		}

		// Start the request tracing span
		err = runtime.BeginRequest(ctx, runtime.RequestData{
			Type:    runtime.PubSubMessage,
			Service: staticCfg.Service.Name,
			MsgData: runtime.PubSubMsgData{
				Topic:        topic.topicCfg.EncoreName,
				Subscription: subscription.EncoreName,
				MessageID:    msgID,
				Attempt:      deliveryAttempt,
				Published:    publishTime,
			},
			CallExprIdx:     0,
			EndpointExprIdx: staticCfg.TraceIdx,
			Inputs:          [][]byte{data},
		})
		if err != nil {
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to begin request").Err()
		}

		err = panicCatchWrapper(ctx, msg)
		runtime.FinishRequest(nil, err)
		return err
	})

	if !config.Cfg.Static.Testing {
		// Log the subscription registration - unless we're in unit tests
		log.Info().Msg("registered subscription")
	}

	return &Subscription[T]{}
}

func (t *Topic[T]) getSubscriptionConfig(name string) (*config.PubsubSubscription, *config.StaticPubsubSubscription) {
	if config.Cfg.Static.Testing {
		// No subscriptions occur in testing
		return &config.PubsubSubscription{EncoreName: name}, &config.StaticPubsubSubscription{
			Service: &config.Service{Name: "test"},
		}
	}

	// Fetch the subscription configuration
	subscription, ok := t.topicCfg.Subscriptions[name]
	if !ok {
		runtime.Logger().Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	staticCfg, ok := config.Cfg.Static.PubsubTopics[t.topicCfg.EncoreName].Subscriptions[name]
	if !ok {
		runtime.Logger().Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	return subscription, staticCfg
}
