package pubsub

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"encore.dev/beta/errs"
	"encore.dev/internal/logging"
	"encore.dev/pubsub/internal/gcp"
	"encore.dev/pubsub/internal/nsq"
	"encore.dev/pubsub/internal/types"
	"encore.dev/pubsub/internal/utils"
	"encore.dev/runtime"
	"encore.dev/runtime/config"
)

// NewTopic is used to declare a Topic. Encore will use static
// analysis to identify Topics and automatically provision them
// for you.
//
// The value passed to cfg will be used at compile time to configure the
// topic. As such is not used directly by this code.
func NewTopic[T any](name string, cfg *TopicConfig) *Topic[T] {
	// Fetch the topic configuration
	topic, ok := config.Cfg.Runtime.PubsubTopics[name]
	if !ok {
		logging.RootLogger.Fatal().Msgf("unregistered/unknown topic: %v", name)
	}

	// Fetch the server config
	if topic.ServerID >= len(config.Cfg.Runtime.PubsubServers) {
		logging.RootLogger.Fatal().Msgf("invalid PubsubServer idx: %v", topic.ServerID)
	}
	server := config.Cfg.Runtime.PubsubServers[topic.ServerID]

	switch {
	case server.NSQServer != nil:
		return &Topic[T]{topicCfg: topic, topic: nsq.NewTopic(server.NSQServer, topic)}
	case server.GCP != nil:
		return &Topic[T]{topicCfg: topic, topic: gcp.NewTopic(server.GCP, topic)}

	default:
		logging.RootLogger.Fatal().Msgf("unsupported PubsubServer type for server idx: %v", topic.ServerID)
		panic("unsupported pubsub server type")
	}
}

// topicAdapter allows us to adapt from the types.TopicImplementation type to our public API
//
// This adapter also contains unified logic for publishing and subscribing to messages on any type of backing topic,
// including:
// - error handling and panic recovery
// - message serialization to attributes and body
//
type Topic[T any] struct {
	topicCfg *config.PubsubTopic
	topic    types.TopicImplementation
}

func (t *Topic[T]) NewSubscription(name string, sub Subscriber[T], cfg *SubscriptionConfig) *Subscription[T] {
	// Fetch the subscription configuration
	subscription, ok := t.topicCfg.Subscriptions[name]
	if !ok {
		logging.RootLogger.Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	staticCfg, ok := config.Cfg.Static.PubsubTopics[t.topicCfg.EncoreName].Subscriptions[name]
	if !ok {
		logging.RootLogger.Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	panicCatchWrapper := func(ctx context.Context, msg T) (err error) {
		defer func() {
			if err2 := recover(); err2 != nil {
				err = errs.B().Code(errs.Internal).Msgf("subscriber paniced: %s", err2).Err()
			}
		}()

		return sub(ctx, msg)
	}

	log := logging.RootLogger.With().
		Str("service", staticCfg.Service.Name).
		Str("topic", t.topicCfg.EncoreName).
		Str("subscription", name).
		Logger()

	// Subscribe to the topic
	t.topic.Subscribe(&log, cfg, subscription, func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) (err error) {
		runtime.BeginOperation()
		defer runtime.FinishOperation()

		var msg T

		if err = json.Unmarshal(data, &msg); err != nil {
			err = errs.B().Cause(err).Code(errs.InvalidArgument).Msg("failed to unmarshal message").Err()
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to unmarshal message")
			return err
		}

		if err = utils.UnmarshalFields(attrs, &msg, utils.AttrTag); err != nil {
			err = errs.B().Cause(err).Code(errs.InvalidArgument).Msg("failed to unmarshal attributes").Err()
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to unmarshal message attributes")
			return err
		}

		// Start the request tracing span
		err = runtime.BeginRequest(ctx, runtime.RequestData{
			Type:    runtime.PubSubMessage,
			Service: staticCfg.Service.Name,
			MsgData: runtime.PubSubMsgData{
				Topic:        t.topicCfg.EncoreName,
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

	log.Info().Msg("registered subscription")

	return &Subscription[T]{}
}

func (t *Topic[T]) Publish(ctx context.Context, msg T) (id string, err error) {
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
