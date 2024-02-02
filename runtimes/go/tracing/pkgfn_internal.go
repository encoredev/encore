package tracing

import (
	"runtime"
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
)

// StartSpan starts a new span with the given name, once started you must call Finish on the returned span to complete
// the span.
//
// For Example:
//
//	func myFunc() {
//	   span := tracing.StartSpan("my-func")
//	   defer span.Finish(nil)
//	   // do stuff
//	}
//
// You can add attributes to the span by calling WithAttributes on the returned span, for example:
//
//	func myFunc(userID int) {
//	   span := tracing.StartSpan("my-func").WithAttributes("user-id", userID)
//	   defer span.Finish(nil)
//	   // do stuff
//	}
//
// ## Span Types
//
// By default the span is created as an internal span, indicating that it is not part of the user's request, but something
// that your app is doing internally to handle the request and you want to track it separately to any other span that
// Encore creates automatically. However if you want to change the span type you can do so by passing a cfg here.
//
// The supported span types are:
// - [AsRequestHandler] - Indicates this span is around code handling a request from an external system.
// - [AsCall] - Indicates this span is wrapped around code making a call to an external system.
// - [AsProducer] - Indicates this span is creating a message which will be handled asynchronously by another system.
// - [AsConsumer] - Indicates this span is handling a message which was created by a producer previously.
func StartSpan(name string, cfg ...SpanConfig) Span {
	rt := reqtrack.Singleton
	if rt == nil {
		// If we're not running inside an encore app the req tracking singleton will not be initialized
		return &userSpan{}
	}

	traceID, err := model.GenTraceID()
	if err != nil {
		rt.Logger().Err(err).Msg("failed to create TraceID for span; no span will be created")
		return &userSpan{}
	}
	spanID, err := model.GenSpanID()
	if err != nil {
		rt.Logger().Err(err).Msg("failed to create SpanID for span; no span will be created")
		return &userSpan{}
	}

	curr := rt.Current()
	isRoot := curr.Req == nil

	if isRoot {
		rt.BeginOperation()
	}

	// Begin the request
	req := &model.Request{
		Type:    model.Unknown,
		TraceID: traceID,
		SpanID:  spanID,
		Start:   time.Now(),
		Traced:  rt.TracingEnabled(),
	}
	rt.BeginRequest(req, true)

	sc := &spanConfig{
		spanType:   trace2.GenericSpanKindInternal,
		attributes: nil,
	}
	for _, opt := range cfg {
		opt(sc)
	}

	// Start the span if we're tracing
	curr = rt.Current()
	tracer := curr.Trace
	if tracer != nil {
		tracer.GenericSpanStart(req, trace2.GenericSpanStartParams{
			EventParams: trace2.EventParams{
				TraceID: req.TraceID,
				SpanID:  req.SpanID,
			},
			Name:       name,
			Kind:       sc.spanType,
			Time:       time.Now(),
			Attributes: sc.attributes,
			StackDepth: 1,
		}, curr.Goctr)
	}

	span := &userSpan{
		req:    req,
		tracer: tracer,
		isRoot: isRoot,
	}

	// If the user never calls Finish on the span we need to make sure we finish it when the span is garbage collected
	// otherwise we'll leave dangling requests in memory as we never call FinishRequest on the request tracker.
	runtime.SetFinalizer(span, userSpanFinalizer)

	return span
}
