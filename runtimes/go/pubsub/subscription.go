package pubsub

import (
	"context"
	"fmt"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/cfgutil"
	"encore.dev/beta/errs"
	"encore.dev/pubsub/internal/noop"
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
// in kebab-case (lowercase alphanumerics and hyphen separated). The subscription name must start with a letter
// and end with either a letter or number. It cannot be longer than 63 characters.
//
// Once created and deployed never change the subscription name, or the topic name otherwise messages will be lost which
// could be in flight.
//
// Example:
//
//		import "encore.dev/pubsub"
//
//		type MyEvent struct {
//		  Foo string
//		}
//
//		var MyTopic = pubsub.NewTopic[*MyEvent]("my-topic", pubsub.TopicConfig{
//		  DeliveryGuarantee: pubsub.AtLeastOnce,
//		})
//
//		var Subscription = pubsub.NewSubscription(MyTopic, "my-subscription", pubsub.SubscriptionConfig[*MyEvent]{
//		  Handler:     HandleEvent,
//		  RetryPolicy: &pubsub.RetryPolicy{MaxRetries: 10},
//	      MaxConcurrency: 5,
//		})
//
//		func HandleEvent(ctx context.Context, event *MyEvent) error {
//		  rlog.Info("received foo")
//		  return nil
//		}
func NewSubscription[T any](topic *Topic[T], name string, cfg SubscriptionConfig[T]) *Subscription[T] {
	if topic.runtimeCfg == nil || topic.topic == nil || topic.mgr == nil {
		panic("pubsub topic was not created using pubsub.NewTopic")
	}

	mgr := topic.mgr
	if _, isNoop := topic.topic.(*noop.Topic); isNoop {
		// no-op means no-op!
		return &Subscription[T]{topic: topic, name: name, cfg: cfg, mgr: mgr}
	}

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

	subscription, staticCfg, exists := topic.getSubscriptionConfig(name)
	if !exists {
		// Noop subscription
		return &Subscription[T]{topic: topic, name: name, cfg: cfg, mgr: mgr}
	}

	// If the service isn't hosted, return a noop subscription.
	if !cfgutil.IsHostedService(mgr.runtime, staticCfg.Service) {
		return &Subscription[T]{topic: topic, name: name, cfg: cfg, mgr: mgr}
	}

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
		Str("topic", topic.runtimeCfg.EncoreName).
		Str("subscription", name).
		Logger()

	// Subscribe to the topic
	topic.topic.Subscribe(&log, cfg.MaxConcurrency, cfg.AckDeadline, cfg.RetryPolicy, subscription, func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) (err error) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		mgr.runningHandlers.Add(1)
		defer mgr.runningHandlers.Done()

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

		traceID, err := model.GenTraceID()
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to generate trace id")
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to generate trace id").Err()
		} else if traceID != (model.TraceID{}) {
			logCtx = logCtx.Str("trace_id", traceID.String())
		}

		spanID, err := model.GenSpanID()
		if err != nil {
			log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to generate span id")
			return errs.B().Code(errs.Internal).Cause(err).Msg("failed to generate span id").Err()
		}

		var parentTraceID model.TraceID
		if parentTraceIDStr := attrs[parentTraceIDAttribute]; parentTraceIDStr != "" {
			parentTraceID, err = model.ParseTraceID(parentTraceIDStr)
			if err != nil {
				log.Err(err).Str("msg_id", msgID).Int("delivery_attempt", deliveryAttempt).Msg("failed to parse parent trace id")
			}
		}

		// Default to logging with the external correlation id if present
		extCorrelationID := attrs[extCorrelationIDAttribute]
		if extCorrelationID != "" {
			logCtx = logCtx.Str("x_correlation_id", extCorrelationID)
		} else if parentTraceID != (model.TraceID{}) {
			logCtx = logCtx.Str("x_correlation_id", parentTraceID.String())
		}

		traced := false
		if val, ok := attrs[parentSampledAttribute]; ok {
			traced, _ = strconv.ParseBool(val)
		} else {
			traced = mgr.rt.SampleTrace()
		}

		// Start the request tracing span
		req := &model.Request{
			Type:             model.PubSubMessage,
			TraceID:          traceID,
			SpanID:           spanID,
			ParentTraceID:    parentTraceID,
			ExtCorrelationID: extCorrelationID,
			Start:            time.Now(),
			MsgData: &model.PubSubMsgData{
				Service:        staticCfg.Service,
				Topic:          topic.runtimeCfg.EncoreName,
				Subscription:   subscription.EncoreName,
				MessageID:      msgID,
				Attempt:        deliveryAttempt,
				Published:      publishTime,
				DecodedPayload: msg,
				Payload:        marshalParams(mgr.json, msg),
			},
			DefLoc: staticCfg.TraceIdx,
			SvcNum: staticCfg.SvcNum,
			Traced: traced,
		}
		reqLogger := logCtx.Logger()
		req.Logger = &reqLogger

		// Copy the previous request information over, if any
		{
			prev := mgr.rt.Current()
			if prevReq := prev.Req; prevReq != nil {
				// TODO(andre) is this correct, or should it be prevReq.SpanID?
				// Maybe it doesn't matter since subscriptions are always root spans anyway.
				req.ParentSpanID = prevReq.ParentSpanID

				req.Test = prevReq.Test
			}
		}

		mgr.rt.BeginRequest(req)
		curr := mgr.rt.Current()
		if curr.Trace != nil {
			curr.Trace.PubsubMessageSpanStart(req, curr.Goctr)
		}

		err = panicCatchWrapper(ctx, msg)

		if curr.Trace != nil {
			resp := &model.Response{
				Duration:   time.Since(req.Start),
				Err:        err,
				HTTPStatus: errs.HTTPStatus(err),
			}
			curr.Trace.PubsubMessageSpanEnd(trace2.PubsubMessageSpanEndParams{
				EventParams: trace2.EventParams{
					TraceID: req.TraceID,
					SpanID:  req.SpanID,
				},
				Req:  req,
				Resp: resp,
			})
		}
		mgr.rt.FinishRequest(false)

		return err
	})

	if !mgr.static.Testing {
		// Log the subscription registration - unless we're in unit tests
		log.Trace().Msg("registered subscription")
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

func (t *Topic[T]) getSubscriptionConfig(name string) (cfg *config.PubsubSubscription, staticCfg *config.StaticPubsubSubscription, ok bool) {
	if t.mgr.static.Testing {
		// No subscriptions occur in testing
		svcName, svcNum := t.mgr.ts.TestService()
		return &config.PubsubSubscription{EncoreName: name}, &config.StaticPubsubSubscription{
			Service: svcName,
			SvcNum:  svcNum,
		}, true
	}

	// Fetch the subscription configuration
	subscription, ok := t.runtimeCfg.Subscriptions[name]
	if !ok {
		return nil, nil, false
	}

	staticCfg, ok = t.mgr.static.PubsubTopics[t.runtimeCfg.EncoreName].Subscriptions[name]
	if !ok {
		return nil, nil, false
	}

	return subscription, staticCfg, true
}

func marshalParams[Resp any](json jsoniter.API, resp Resp) []byte {
	data, _ := json.Marshal(resp)
	return data
}

// MethodHandler is used to define a subscription Handler that references a service struct method.
//
// Example Usage:
//
//	//encore:service
//	type Service struct {}
//
//	func (s *Service) Method(ctx context.Context, msg *Event) error { /* ... */ }
//
//	var _ = pubsub.NewSubscription(Topic, "subscription-name", pubsub.SubscriptionConfig[*Event]{
//		Handler: pubsub.MethodHandler((*MyService).MyMethod),
//		// ...
//	})
func MethodHandler[T, SvcStruct any](handler func(s SvcStruct, ctx context.Context, msg T) error) func(ctx context.Context, msg T) error {
	// The use of MethodHandler acts as a sentinel for the code generator,
	// which replaces the call with some generated code to initialize the service struct.
	// As such this function should never be called in practice.
	return func(ctx context.Context, msg T) error {
		return fmt.Errorf("pubsub.MethodHandler is not usable in this context")
	}
}
