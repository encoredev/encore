// Package tracing provides support for custom trace spans within Encore applications.
//
// Custom spans let you group operations (DB queries, PubSub publishes, HTTP calls, etc.)
// under a named span in the trace viewer. When a custom span is active, all operations
// executed within it are recorded as children of that span.
//
// Custom spans can be nested: if you start a span while another custom span is active,
// the new span becomes a child of the existing custom span.
//
// For more information about tracing inside Encore applications see https://encore.dev/docs/observability/tracing.
package tracing

import (
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
type Manager struct {
	rt *reqtrack.RequestTracker
}

//publicapigen:drop
func NewManager(rt *reqtrack.RequestTracker) *Manager {
	return &Manager{rt: rt}
}

// SpanOption configures a custom span.
type SpanOption func(*spanConfig)

type spanConfig struct {
	attrs map[string]string
}

// WithAttributes adds key-value attributes to the span.
// Attributes are recorded at span start and can be used for filtering/searching.
func WithAttributes(attrs map[string]string) SpanOption {
	return func(cfg *spanConfig) {
		cfg.attrs = attrs
	}
}

// Span represents an active custom trace span.
// Use [Manager.StartSpan] to create a span and [Span.End] to end it.
type Span struct {
	mgr          *Manager
	traceID      model.TraceID
	spanID       model.SpanID
	parentSpanID model.SpanID
	trace        trace2.Logger
	start        time.Time
	active       bool
}

// StartSpan starts a new custom trace span with the given name.
// The span is automatically registered as a child of the currently active span
// (either the request span or another custom span).
//
// While this span is active, all traced operations (DB queries, PubSub publishes,
// RPC calls, etc.) will be recorded under this span.
//
// The span must be ended by calling [Span.End]. A common pattern is:
//
//	span := tracing.StartSpan("processOrder")
//	defer span.End()
func (m *Manager) StartSpan(name string, opts ...SpanOption) *Span {
	cfg := &spanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	curr := m.rt.Current()
	if curr.Req == nil || curr.Trace == nil || !curr.Req.Traced {
		return &Span{mgr: m}
	}

	spanID, err := model.GenSpanID()
	if err != nil {
		return &Span{mgr: m}
	}

	parentSpanID := curr.SpanID
	traceID := curr.Req.TraceID

	curr.Trace.CustomSpanStart(trace2.CustomSpanStartParams{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Goid:         curr.Goctr,
		Name:         name,
		Attributes:   cfg.attrs,
	})

	m.rt.PushSpan(spanID)

	return &Span{
		mgr:          m,
		traceID:      traceID,
		spanID:       spanID,
		parentSpanID: parentSpanID,
		trace:        curr.Trace,
		start:        time.Now(),
		active:       true,
	}
}

// End ends the span successfully (with no error).
// After End is called, operations are no longer recorded under this span.
// It is safe to call End multiple times; subsequent calls are no-ops.
func (s *Span) End() {
	s.end(nil)
}

// EndErr ends the span with the given error.
// If err is nil, the span is ended successfully.
// After EndErr is called, operations are no longer recorded under this span.
// It is safe to call EndErr multiple times; subsequent calls are no-ops.
func (s *Span) EndErr(err error) {
	s.end(err)
}

func (s *Span) end(err error) {
	if !s.active {
		return
	}
	s.active = false

	s.mgr.rt.PopSpan()

	if s.trace != nil {
		s.trace.CustomSpanEnd(trace2.CustomSpanEndParams{
			TraceID:      s.traceID,
			SpanID:       s.spanID,
			ParentSpanID: s.parentSpanID,
			Duration:     time.Since(s.start),
			Err:          err,
		})
	}
}
