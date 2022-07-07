package pubsub

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/gcp"
	"encore.dev/pubsub/internal/nsq"
	"encore.dev/pubsub/internal/test"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
	"encore.dev/runtime"
	"encore.dev/runtime/config"
)

// Topic presents a flow of events of type T from any number of publishers to
// any number of subscribers.
//
// Each subscription will receive a copy of each message published to the topic.
//
// See NewTopic for more information on how to declare a Topic.
type Topic[T any] struct {
	topicCfg *config.PubsubTopic
	topic    types.TopicImplementation
}

// NewTopic is used to declare a Topic. Encore will use static
// analysis to identify Topics and automatically provision them
// for you.
//
// A call to NewTopic can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The topic name must be unique within an Encore application. Topic names must be defined
// in kebab-case (lowercase alphanumerics and hyphen seperated). The topic name must start with a letter
// and end with either a letter or number. It cannot be longer than 63 characters. Once created and deployed never
// change the topic name. When refactoring the topic name must stay the same.
// This allows for messages already on the topic to continue to be received after the refactored
// code is deployed.
//
// Example:
//
//     import "encore.dev/pubsub"
//
//     type MyEvent struct {
//       Foo string
//     }
//
//     var MyTopic = pubsub.NewTopic[*MyEvent]("my-topic", pubsub.TopicConfig{
//       DeliveryGuarantee: pubsub.AtLeastOnce,
//     })
//
//    //encore:api public
//    func DoFoo(ctx context.Context) error {
//      msgID, err := MyTopic.Publish(ctx, &MyEvent{Foo: "bar"})
//      if err != nil { return err }
//      rlog.Info("foo published", "message_id", msgID)
//      return nil
//    }
func NewTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	if config.Cfg.Static.Testing {
		return &Topic[T]{
			topicCfg: &config.PubsubTopic{EncoreName: name},
			topic:    test.NewTopic[T](name),
		}
	}

	// Look up the topic configuration
	topic, ok := config.Cfg.Runtime.PubsubTopics[name]
	if !ok {
		runtime.Logger().Fatal().Msgf("unregistered/unknown topic: %v", name)
	}

	// Look up the server config
	provider := config.Cfg.Runtime.PubsubProviders[topic.ProviderID]

	switch {
	case provider.NSQ != nil:
		return &Topic[T]{topicCfg: topic, topic: nsq.NewTopic(provider.NSQ, topic)}
	case provider.GCP != nil:
		return &Topic[T]{topicCfg: topic, topic: gcp.NewTopic(provider.GCP, topic)}

	default:
		runtime.Logger().Fatal().Msgf("unsupported PubsubProvider type for server idx: %v", topic.ProviderID)
		panic("unsupported pubsub server type")
	}
}

// Publish will publish a message to the topic and returns a unique message ID for the message.
//
// This function will not return until the message has been successfully accepted by the topic.
//
// If an error is returned, it is probable that the message failed to be published, however it is possible
// that the message could still be received by subscriptions to the topic.
func (t *Topic[T]) Publish(ctx context.Context, msg T) (id string, err error) {
	if t.topicCfg == nil || t.topic == nil {
		return "", errs.B().Code(errs.Unimplemented).Msg("pubsub topic was not created using pubsub.NewTopic").Err()
	}

	// Extract the message attributes
	attrs, err := utils.MarshalFields(msg, utils.AttrTag)
	if err != nil {
		return "", errs.B().Cause(err).Code(errs.InvalidArgument).Msgf("failed to extract message attributes for topic %s", t.topicCfg.EncoreName).Err()
	}

	// Marshal the message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return "", errs.B().Cause(err).Code(errs.InvalidArgument).Msgf("failed to marshal message to JSON for topic %s", t.topicCfg.EncoreName).Err()
	}

	// Start the trace span
	publishTraceID := atomic.AddUint64(&publishCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		tracePublishStart(t.topicCfg.EncoreName, data, req.SpanID, uint64(goid), publishTraceID, 2)
	}

	// Publish to the clouds topic
	id, err = t.topic.PublishMessage(ctx, attrs, data)
	if err != nil {
		return "", errs.B().Cause(err).Code(errs.Unavailable).Msgf("failed to publish message to %s", t.topicCfg.EncoreName).Err()
	}

	// End the trace span
	if req != nil && req.Traced {
		tracePublishEnd(publishTraceID, id, err)
	}

	return id, nil
}
