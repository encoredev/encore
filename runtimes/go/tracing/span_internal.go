package tracing

import (
	"errors"
	"sync/atomic"
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
)

// Span represents a span that can be finished.
type Span interface {
	// WithAttributes adds the given key/value pairs as attributes to the span, when the span is finished these attributes
	// will be added to the span result.
	//
	// The keysAndValues must be pairs of string keys and arbitrary data
	//
	// The returned span is still the same span, it is returned for chaining purposes.
	WithAttributes(keysAndValues ...any) Span

	// Finish completes the span, and reports the error if provided, if nil the span is reported as successful.
	//
	// If you call it more than once it will panic.
	Finish(err error)
}

type userSpan struct {
	req        *model.Request
	tracer     trace2.Logger
	attributes []trace2.LogField
	isRoot     bool
	finished   atomic.Bool
}

func userSpanFinalizer(s *userSpan) {
	if !s.finished.Load() {
		s.Finish(errors.New("span not finished before being garbage collected"))
	}
}

func (u *userSpan) WithAttributes(keysAndValues ...any) Span {
	for i := 0; i < len(keysAndValues); i += 2 {
		u.attributes = append(u.attributes, trace2.LogField{
			Key:   keysAndValues[i].(string),
			Value: keysAndValues[i+1],
		})
	}

	return u
}

func (u *userSpan) Finish(err error) {
	if !u.finished.CompareAndSwap(false, true) {
		panic("span already finished")
	}

	rt := reqtrack.Singleton
	if u.req == nil || rt == nil {
		return
	}

	// End the span if we're tracing
	curr := rt.Current()
	if u.tracer != nil {
		u.tracer.GenericSpanEnd(u.req, trace2.GenericSpanEndParams{
			EventParams: trace2.EventParams{
				TraceID: u.req.TraceID,
				SpanID:  u.req.SpanID,
				Goid:    curr.Goctr,
			},
			Time:       time.Now(),
			Error:      err,
			Attributes: u.attributes,
			StackDepth: 1,
		})
	}

	// End the request
	rt.FinishRequest(false)

	if u.isRoot {
		rt.FinishOperation()
	}
}

type spanConfig struct {
	spanType   trace2.GenericSpanKind
	attributes []trace2.LogField
}
type SpanConfig func(*spanConfig)

// WithAttributes is a SpanConfig that adds the given key/value pairs as attributes to the span.
func WithAttributes(keysAndValues ...any) SpanConfig {
	return func(c *spanConfig) {
		for i := 0; i < len(keysAndValues); i += 2 {
			c.attributes = append(c.attributes, trace2.LogField{
				Key:   keysAndValues[i].(string),
				Value: keysAndValues[i+1],
			})
		}
	}
}

// AsRequestHandler is a SpanConfig that sets the span type to be a request handler,
// this is useful for spans that are created as a direct response to an external request.
func AsRequestHandler() SpanConfig {
	return func(c *spanConfig) {
		c.spanType = trace2.GenericSpanKindRequest
	}
}

// AsCall is a SpanConfig that sets the span type to be a call, indicating the code within
// the span is making a call to another service or system.
func AsCall() SpanConfig {
	return func(c *spanConfig) {
		c.spanType = trace2.GenericSpanKindCall
	}
}

// AsProducer is a SpanConfig that sets the span type to be a producer, indicating the code within
// the span is producing a message to be consumed by another service or system at a later time.
func AsProducer() SpanConfig {
	return func(c *spanConfig) {
		c.spanType = trace2.GenericSpanKindProducer
	}
}

// AsConsumer is a SpanConfig that sets the span type to be a consumer, indicating the code within
// the span is consuming a message produced by another service or system.
func AsConsumer() SpanConfig {
	return func(c *spanConfig) {
		c.spanType = trace2.GenericSpanKindConsumer
	}
}
