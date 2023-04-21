package pubsub

import (
	"context"
	"time"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/config"
	model2 "encore.dev/appruntime/exported/model"
	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/utils"
)

// Subscription represents a subscription to a Topic.
type Subscription[T any] struct {
	topic *Topic[T]
	name  string
	cfg   SubscriptionConfig[T]
	mgr   *Manager
}

// NewSubscription is used to declare a Subscription to a topic. The passed in handler will be called
// for each message published to the topic.
//
// A call to NewSubscription can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The subscription name must be unique for that topic. Subscription names must be defined
// in kebab-case (lowercase alphanumerics and hyphen seperated). The subscription name must start with a letter
// and end with either a letter or number. It cannot be longer than 63 characters.
//
// Once created and deployed never change the subscription name, or the topic name otherwise messages will be lost which
// could be in flight.
//
// Example:
//
//	import "encore.dev/pubsub"
//
//	type MyEvent struct {
//	  Foo string
//	}
//
//	var MyTopic = pubsub.NewTopic[*MyEvent]("my-topic", pubsub.TopicConfig{
//	  DeliveryGuarantee: pubsub.AtLeastOnce,
//	})
//
//	var Subscription = pubsub.NewSubscription(MyTopic, "my-subscription", pubsub.SubscriptionConfig[*MyEvent]{
//	  Handler:     HandleEvent,
//	  RetryPolicy: &pubsub.RetryPolicy { MaxRetries: 10 },
//	})
//
//	func HandleEvent(ctx context.Context, event *MyEvent) error {
//	  rlog.Info("received foo")
//	  return nil
//	}
func NewSubscription[T any](topic *Topic[T], name string, cfg SubscriptionConfig[T]) *Subscription[T] {
	if topic.topicCfg == nil || topic.topic == nil || topic.mgr == nil {
		panic("pubsub topic was not created using pubsub.NewTopic")
	}
	mgr := topic.mgr

	// Set default config values for missing values
	if cfg.RetryPolicy == nil {
		cfg.RetryPolicy = &RetryPolicy{
			MaxRetries: 100,
		}
	}
	if cfg.RetryPolicy.MinBackoff < 0 {
		panic("MinRetryDelay cannot be negative")
	}
	if cfg.RetryPolicy.MaxBackoff < 0 {
		panic("MaxRetryDelay cannot be negative")
	}
	cfg.RetryPolicy.MinBackoff = utils.WithDefaultValue(cfg.RetryPolicy.MinBackoff, 10*time.Second)
	cfg.RetryPolicy.MaxBackoff = utils.WithDefaultValue(cfg.RetryPolicy.MaxBackoff, 10*time.Minute)

	if cfg.AckDeadline == 0 {
		cfg.AckDeadline = 30 * time.Second
	} else if cfg.AckDeadline < 0 {
		panic("AckDeadline cannot be negative")
	}

	subscription, staticCfg := topic.getSubscriptionConfig(name)
	panicCatchWrapper := func(ctx context.Context, msg T) (err error) {
		defer func() {
			if err2 := recover(); err2 != nil {
				err = errs.B().Code(errs.Internal).Msgf("subscriber panicked: %s", err2).Err()
			}
		}()

		return cfg.Handler(ctx, msg)
	}

	log := mgr.rootLogger.With().
		Str("service", staticCfg.Service).
		Str("topic", topic.topicCfg.EncoreName).
		Str("subscription", name).
		Logger()

	tracingEnabled := mgr.rt.TracingEnabled()

	// Subscribe to the topic
	topic.topic.Subscribe(&log, cfg.AckDeadline, cfg.RetryPolicy, subscription, func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) (err error) {
		mgr.outstanding.Inc()
		defer mgr.outstanding.Dec()

		if !mgr.static.Testing {
			// Under test we're already inside an operation
			mgr.rt.BeginOperation()
			defer mgr.rt.FinishOperation()
		}

		msg, err := utils.UnmarshalMessage[T](attrs, data)
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to unmarshal message")
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to unmarshal message").Err()
		}

		logCtx := log.With()

		traceID, err := model2.GenTraceID()
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to generate trace id")
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to generate trace id").Err()
		} else if traceID != (model2.TraceID{}) {
			logCtx = logCtx.Str("trace_id", traceID.String())
		}

		spanID, err := model2.GenSpanID()
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to generate span id")
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to generate span id").Err()
		}

		var parentTraceID model2.TraceID
		if parentTraceIDStr := attrs[parentTraceIDAttribute]; parentTraceIDStr != "" {
			parentTraceID, err = model2.ParseTraceID(parentTraceIDStr)
			if err != nil {
				log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to parse parent trace id")
			}
		}

		// Default to logging with the external correlation id if present
		extCorrelationID := attrs[extCorrelationIDAttribute]
		if extCorrelationID != "" {
			logCtx = logCtx.Str("x_correlation_id", extCorrelationID)
		} else if parentTraceID != (model2.TraceID{}) {
			logCtx = logCtx.Str("x_correlation_id", parentTraceID.String())
		}
		// Start the request tracing span
		req := &model2.Request{
			Type:             model2.PubSubMessage,
			TraceID:          traceID,
			SpanID:           spanID,
			ParentTraceID:    parentTraceID,
			ExtCorrelationID: extCorrelationID,
			Start:            time.Now(),
			MsgData: &model2.PubSubMsgData{
				Service:        staticCfg.Service,
				Topic:          topic.topicCfg.EncoreName,
				Subscription:   subscription.EncoreName,
				MessageID:      msgID,
				Attempt:        deliveryAttempt,
				Published:      publishTime,
				DecodedPayload: msg,
				Payload:        marshalParams(mgr.json, msg),
			},
			DefLoc: staticCfg.TraceIdx,
			SvcNum: staticCfg.SvcNum,
			Traced: tracingEnabled,
		}
		reqLogger := logCtx.Logger()
		req.Logger = &reqLogger

		// Copy the previous request information over, if any
		{
			prev := mgr.rt.Current()
			if prevReq := prev.Req; prevReq != nil {
				req.ParentID = prevReq.ParentID
				req.Traced = prevReq.Traced
				req.Test = prevReq.Test
			}
		}

		mgr.rt.BeginRequest(req)
		curr := mgr.rt.Current()
		if curr.Trace != nil {
			curr.Trace.BeginRequest(req, curr.Goctr)
		}

		err = panicCatchWrapper(ctx, msg)

		if curr.Trace != nil {
			resp := &model2.Response{
				Err:        err,
				HTTPStatus: errs.HTTPStatus(err),
			}
			curr.Trace.FinishRequest(req, resp)
		}
		mgr.rt.FinishRequest()

		return err
	})

	if !mgr.static.Testing {
		// Log the subscription registration - unless we're in unit tests
		log.Info().Msg("registered subscription")
	}

	return &Subscription[T]{topic: topic, name: name, cfg: cfg, mgr: mgr}
}

// SubscriptionMeta contains metadata about a subscription.
// The fields should not be modified by the caller.
// Additional fields may be added in the future.
type SubscriptionMeta[T any] struct {
	// Name is the name of the subscription, as provided in the constructor to NewSubscription.
	Name string

	// Config is the subscriptions's configuration.
	Config SubscriptionConfig[T]

	// Topic provides metadata about the topic it subscribes to.
	Topic TopicMeta
}

// Meta returns metadata about the topic.
func (t *Subscription[T]) Meta() SubscriptionMeta[T] {
	return SubscriptionMeta[T]{
		Name:   t.name,
		Config: t.cfg,
		Topic:  t.topic.Meta(),
	}
}

// Config returns the subscription's configuration.
// It must not be modified by the caller.
func (s *Subscription[T]) Config() SubscriptionConfig[T] {
	return s.cfg
}

func (t *Topic[T]) getSubscriptionConfig(name string) (*config.PubsubSubscription, *config.StaticPubsubSubscription) {
	if t.mgr.static.Testing {
		// No subscriptions occur in testing
		return &config.PubsubSubscription{EncoreName: name}, &config.StaticPubsubSubscription{
			Service: t.mgr.ts.TestService(),
		}
	}

	// Fetch the subscription configuration
	subscription, ok := t.topicCfg.Subscriptions[name]
	if !ok {
		t.mgr.rootLogger.Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	staticCfg, ok := t.mgr.static.PubsubTopics[t.topicCfg.EncoreName].Subscriptions[name]
	if !ok {
		t.mgr.rootLogger.Fatal().Msgf("unregistered/unknown subscription on topic %s: %s", t.topicCfg.EncoreName, name)
	}

	return subscription, staticCfg
}

func marshalParams[Resp any](json jsoniter.API, resp Resp) []byte {
	data, _ := json.Marshal(resp)
	return data
}
