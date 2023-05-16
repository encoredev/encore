package trace2

import (
	"context"
	"errors"
	"time"

	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

type Meta struct {
	AppID string
}

type Query struct {
	AppID        string
	Service      string
	Endpoint     string
	Topic        string
	Subscription string
	TraceID      string
	MessageID    string
	Tags         []Tag

	// StartTime and EndTime specify the time range to query.
	// If zero values they are not bounded.
	StartTime, EndTime time.Time

	IsError *bool // nil means both successes and failures are returned

	// Minimum and maximum duration (in nanoseconds) to filter requests for.
	// If MaxDurMicros is 0 it defaults to no limit.
	MinDurNanos, MaxDurNanos uint64

	Limit int // if 0 defaults to 100.
}

type Tag struct {
	Key   string
	Value string
}

// ErrNotFound is reported by Store.Get when a trace is not found.
var ErrNotFound = errors.New("trace not found")

// A ListEntryIterator is called once for each trace matching the query string,
// sequentially and in streaming fashion as traces are read from the store.
//
// If it returns false the listing operation is stopped and the function is
// not called again.
type ListEntryIterator func(*tracepb2.SpanSummary) bool

// An EventIterator is called once for each event in a trace,
// sequentially and in streaming fashion as events are read from the store.
//
// If it returns false the stream is aborted and the function is
// not called again.
type EventIterator func(*tracepb2.TraceEvent) bool

// Store is the interface for storing and retrieving traces.
type Store interface {
	// WriteEvents persists requests in the store.
	WriteEvents(ctx context.Context, meta *Meta, events []*tracepb2.TraceEvent) error

	// List lists traces that match the query.
	// It calls fn for each trace read; see ListEntryIterator.
	List(ctx context.Context, q *Query, iter ListEntryIterator) error

	// Get streams events matching the given trace id.
	// fn may be called with events out of order.
	// If the trace is not found it reports an error matching ErrNotFound.
	Get(ctx context.Context, appID, traceID string, iter EventIterator) error

	// Listen listens for new spans.
	Listen(ch <-chan *tracepb2.SpanSummary)
}
