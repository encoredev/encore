//go:build opentelemetry

package reqtrack

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
)

// configureOpenTelemetry is a no-op when OpenTelemetry is not enabled.
//
// Our implementations of the OpenTelemetry API embed the noop implementations
// from the OpenTelemetry API. This means if the user forcibly updates the
// OpenTelemetry API to a version which has new methods, we will default to the
// noop implementation for those methods, rather than a compilation error (which
// would happen if we embedded `embedded.TracerProvider`) or panicing at runtime
// (which would happen if we embedded `trace.TracerProvider`).
func configureOpenTelemetry(reqTracker *RequestTracker) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		reqTracker.Logger().Err(err).Msg("an error occurred in the OpenTelemetry SDK")
	}))
	otel.SetLogger(logr.New(&logrSink{reqTracker: reqTracker}))
	otel.SetTracerProvider(&otelTraceProvider{reqTracker: reqTracker})
}

// logrSink is a logr.LogSink that writes to our request logger. We need this because the OpenTelemetry library
// uses the logr library for logging.
type logrSink struct {
	reqTracker *RequestTracker
	name       string
	values     []any
	depth      int
}

var (
	_ logr.LogSink          = &logrSink{}
	_ logr.CallDepthLogSink = &logrSink{}
)

// Init receives runtime info about the logr library.
func (ls *logrSink) Init(info logr.RuntimeInfo) {
	ls.depth = info.CallDepth + 2
}

// Enabled tests whether this LogSink is enabled at the specified V-level.
func (ls *logrSink) Enabled(level int) bool {
	zlvl := zerolog.Level(1 - level)
	return zlvl >= ls.reqTracker.Logger().GetLevel() && zlvl >= zerolog.GlobalLevel()
}

// Info logs a non-error message at specified V-level with the given key/value pairs as context.
func (ls *logrSink) Info(level int, msg string, keysAndValues ...any) {
	logger := ls.reqTracker.Logger()
	ev := logger.WithLevel(zerolog.Level(1 - level))
	ls.msg(ev, msg, keysAndValues)
}

// Error logs an error, with the given message and key/value pairs as context.
func (ls *logrSink) Error(err error, msg string, keysAndValues ...any) {
	logger := ls.reqTracker.Logger()
	ev := logger.Err(err)
	ls.msg(ev, msg, keysAndValues)
}

// WithValues returns a new LogSink with additional key/value pairs.
func (ls *logrSink) WithValues(keysAndValues ...any) logr.LogSink {
	return &logrSink{
		reqTracker: ls.reqTracker,
		name:       ls.name,
		values:     append(ls.values, keysAndValues...),
		depth:      ls.depth,
	}
}

// WithName returns a new LogSink with the specified name appended in NameFieldName.
// Name elements are separated by NameSeparator.
func (ls *logrSink) WithName(name string) logr.LogSink {
	return &logrSink{
		reqTracker: ls.reqTracker,
		name:       name,
		values:     ls.values,
		depth:      ls.depth,
	}
}

// WithCallDepth returns a new LogSink that offsets the call stack by adding specified depths.
func (ls *logrSink) WithCallDepth(depth int) logr.LogSink {
	return &logrSink{
		reqTracker: ls.reqTracker,
		name:       ls.name,
		values:     ls.values,
		depth:      ls.depth + depth,
	}
}

// msg is a helper function to log a message with the given key/value pairs as context.
func (ls *logrSink) msg(ev *zerolog.Event, msg string, keysAndValues []interface{}) {
	if ev == nil {
		return
	}
	if ls.name != "" {
		ev.Str("logger", ls.name)
	}

	for i, n := 1, len(keysAndValues); i < n; i += 2 {
		value := keysAndValues[i]
		switch v := value.(type) {
		case logr.Marshaler:
			keysAndValues[i] = v.MarshalLog()
		case fmt.Stringer:
			keysAndValues[i] = v.String()
		}
	}

	ev = ev.Fields(keysAndValues)
	ev.CallerSkipFrame(ls.depth)

	ev.Msg(msg)
}

type otelTraceProvider struct {
	noop.TracerProvider

	// Our request tracker
	reqTracker *RequestTracker
}

func (o *otelTraceProvider) Tracer(name string, _ ...trace.TracerOption) trace.Tracer {
	return &otelTracer{
		name:     name,
		provider: o,
	}
}

type otelTracer struct {
	noop.Tracer

	name     string
	provider *otelTraceProvider
}

func (o *otelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	rt := o.provider.reqTracker

	var root bool
	traceID, err := model.GenTraceID()
	if err != nil {
		rt.Logger().Err(err).Msg("failed to create TraceID for OpenTelemetry span; no span will be created")
		return ctx, noop.Span{}
	}
	spanID, err := model.GenSpanID()
	if err != nil {
		rt.Logger().Err(err).Msg("failed to create SpanID for OpenTelemetry span; no span will be created")
		return ctx, noop.Span{}
	}

	// We need to know if this span is the root span or not.
	//
	// This would happen if the user calls `Start` directly on the tracer from a background goroutine.
	// which isn't associated with a request.
	if curr := rt.Current(); curr.Req == nil {
		root = true
	}

	otelData := &otelSpanData{}
	req := &model.Request{
		Type:    model.Unknown,
		TraceID: traceID,
		SpanID:  spanID,
		Start:   time.Now(),
		Logger:  nil,
		Traced:  rt.TracingEnabled(),
	}

	// Now apply the span start options to the request.
	cfg := trace.NewSpanStartConfig(opts...)

	// NewRoot identifies a Span as the root Span for a new trace. This is
	// commonly used when an existing trace crosses trust boundaries and the
	// remote parent span context should be ignored for security.
	if cfg.NewRoot() && !root {
		rt.clearReq()
	}

	// Note: beging request will mutate the request object and copy over
	// the traceID and spanID from the current request if there is one.
	rt.BeginRequest(req, true)

	// Now record this span starting
	var traceLogger trace2.Logger
	if curr := rt.Current(); curr.Trace != nil {
		traceLogger = curr.Trace

		kind := trace2.GenericSpanKindUnknown
		switch otelData.SpanKind {
		case trace.SpanKindInternal:
			kind = trace2.GenericSpanKindInternal
		case trace.SpanKindServer:
			kind = trace2.GenericSpanKindRequest
		case trace.SpanKindClient:
			kind = trace2.GenericSpanKindCall
		case trace.SpanKindProducer:
			kind = trace2.GenericSpanKindProducer
		case trace.SpanKindConsumer:
			kind = trace2.GenericSpanKindConsumer
		}

		params := trace2.GenericSpanStartParams{
			EventParams: trace2.EventParams{
				TraceID: req.TraceID,
				SpanID:  req.SpanID,
			},
			Name:       spanName,
			Time:       time.Now(),
			Kind:       kind,
			Attributes: attributesToLogFields(cfg.Attributes()),
			StackDepth: -1,
		}

		if ts := cfg.Timestamp(); !ts.IsZero() {
			params.Time = ts
		}

		if cfg.StackTrace() {
			params.StackDepth = 1
		}

		traceLogger.GenericSpanStart(req, params, curr.Goctr)
	}

	// Finally return the Open Telemetry span
	var traceFlags trace.TraceFlags
	if req.Traced {
		traceFlags = trace.FlagsSampled
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(req.TraceID),
		SpanID:     trace.SpanID(req.SpanID),
		TraceFlags: traceFlags,
		TraceState: trace.TraceState{},
		Remote:     false,
	})

	span := &otelSpan{
		tracer:            o,
		name:              spanName,
		root:              root,
		sc:                sc,
		traceLogger:       traceLogger,
		resultCode:        codes.Unset,
		resultDescription: "",
	}

	return trace.ContextWithSpan(ctx, span), span
}

type otelSpanData struct {
	SpanKind   trace.SpanKind
	Attributes []attribute.KeyValue
}

type otelSpan struct {
	noop.Span
	tracer *otelTracer

	name string
	root bool // is this span a root span?
	sc   trace.SpanContext
	req  *model.Request

	// The trace logger for this request.
	// Note we track it within the otel span, so we can access it from any thread
	// even if that thread is (according to Encore's tracing) not related to the span.
	//
	// This can happen in otel due to the fact Spans are simply user-defined and the user
	// could in theory send one over a channel to another goroutine.
	traceLogger trace2.Logger

	mu sync.Mutex

	// attributes recorded by the user using [SetAttributes]
	attributes        []attribute.KeyValue
	lastError         error
	resultCode        codes.Code
	resultDescription string
}

func (o *otelSpan) End(options ...trace.SpanEndOption) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.sc.TraceFlags().IsSampled() {
		return
	}

	rt := o.tracer.provider.reqTracker

	cfg := trace.NewSpanEndConfig(options...)

	// If we're tracing, record the span end
	if o.traceLogger != nil {
		attributes := append(o.attributes, cfg.Attributes()...)

		params := trace2.GenericSpanEndParams{
			EventParams: trace2.EventParams{
				TraceID: model.TraceID(o.sc.TraceID()),
				SpanID:  model.SpanID(o.sc.SpanID()),
			},
			Time:       time.Now(),
			Error:      o.lastError,
			Attributes: attributesToLogFields(attributes),
			StackDepth: -1,
		}

		if ts := cfg.Timestamp(); !ts.IsZero() {
			params.Time = ts
		}

		if cfg.StackTrace() {
			params.StackDepth = 1
		}

		o.traceLogger.GenericSpanEnd(o.req, params)
	}

	rt.FinishRequest(false)
}

func (o *otelSpan) AddEvent(name string, options ...trace.EventOption) {
	if o.traceLogger == nil {
		return
	}

	cfg := trace.NewEventConfig(options...)

	params := trace2.GenericEventParams{
		EventParams: trace2.EventParams{
			TraceID: model.TraceID(o.sc.TraceID()),
			SpanID:  model.SpanID(o.sc.SpanID()),
		},
		Name:       name,
		Time:       time.Now(),
		StackDepth: -1,
		Attributes: attributesToLogFields(cfg.Attributes()),
	}

	if ts := cfg.Timestamp(); !ts.IsZero() {
		params.Time = ts
	}

	if cfg.StackTrace() {
		params.StackDepth = 1
	}

	o.traceLogger.GenericEvent(params)
}

func (o *otelSpan) IsRecording() bool {
	return o.sc.IsSampled()
}

func (o *otelSpan) RecordError(err error, options ...trace.EventOption) {
	o.mu.Lock()
	o.lastError = err
	o.mu.Unlock()

	if o.traceLogger == nil {
		return
	}

	o.resultCode = codes.Error
	o.resultDescription = err.Error()

	cfg := trace.NewEventConfig(options...)

	params := trace2.GenericEventParams{
		EventParams: trace2.EventParams{
			TraceID: model.TraceID(o.sc.TraceID()),
			SpanID:  model.SpanID(o.sc.SpanID()),
		},
		Name:       "Error",
		Error:      err,
		Time:       time.Now(),
		StackDepth: -1,
		Attributes: attributesToLogFields(cfg.Attributes()),
	}

	if ts := cfg.Timestamp(); !ts.IsZero() {
		params.Time = ts
	}

	if cfg.StackTrace() {
		params.StackDepth = 1
	}

	o.traceLogger.GenericEvent(params)
}

func (o *otelSpan) SpanContext() trace.SpanContext {
	return o.sc
}

func (o *otelSpan) SetStatus(code codes.Code, description string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.resultCode = code
	o.resultDescription = description
}

func (o *otelSpan) SetName(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.name = name
}

func (o *otelSpan) SetAttributes(kv ...attribute.KeyValue) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.attributes = append(o.attributes, kv...)
}

func (o *otelSpan) TracerProvider() trace.TracerProvider {
	return o.tracer.provider
}

// attributesToLogFields converts OpenTelemetry attributes to Encore log fields.
// which we already can log.
func attributesToLogFields(attrs []attribute.KeyValue) []trace2.LogField {
	fields := make([]trace2.LogField, 0, len(attrs))

	for i, attr := range attrs {
		var values []any
		switch value := attr.Value.AsInterface().(type) {
		case []bool:
			values = make([]any, len(value))
			for i, v := range value {
				values[i] = v
			}
		case []int64:
			values = make([]any, len(value))
			for i, v := range value {
				values[i] = v
			}
		case []float64:
			values = make([]any, len(value))
			for i, v := range value {
				values[i] = v
			}
		case []string:
			values = make([]any, len(value))
			for i, v := range value {
				values[i] = v
			}
		case bool, int64, float64, string:
			values = []any{value}
		default:
			panic(fmt.Sprintf("unknown attribute value type %T", value))
		}

		for _, v := range values {
			fields[i] = trace2.LogField{
				Key:   string(attr.Key),
				Value: v,
			}
		}
	}

	return fields
}
