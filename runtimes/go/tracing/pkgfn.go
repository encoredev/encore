//go:build encore_app

package tracing

import (
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
var Singleton = NewManager(reqtrack.Singleton)

// StartSpan starts a new custom trace span with the given name.
// The span is automatically registered as a child of the currently active span
// (either the request span or another custom span).
//
// While this span is active, all traced operations (DB queries, PubSub publishes,
// RPC calls, etc.) will be recorded under this span.
//
// The span must be ended by calling [Span.End] or [Span.EndErr]. A common pattern is:
//
//	span := tracing.StartSpan("processOrder")
//	defer span.End()
//
// For spans where you want to capture errors:
//
//	func processOrder(id string) (err error) {
//	    span := tracing.StartSpan("processOrder")
//	    defer func() { span.EndErr(err) }()
//	    // ... your logic here
//	}
func StartSpan(name string, opts ...SpanOption) *Span {
	return Singleton.StartSpan(name, opts...)
}
