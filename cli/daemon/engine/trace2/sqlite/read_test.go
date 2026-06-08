package sqlite

import (
	"context"
	"database/sql"
	"slices"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

const testSchema = `
CREATE TABLE trace_event (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	app_id TEXT NOT NULL,
	trace_id TEXT NOT NULL,
	span_id TEXT NOT NULL,
	event_data TEXT NOT NULL
);
CREATE TABLE trace_span_index (
	trace_id TEXT NOT NULL,
	span_id TEXT NOT NULL,
	app_id TEXT NOT NULL,
	span_type INTEGER NOT NULL,
	started_at INTEGER NULL,
	is_root BOOLEAN NULL,
	service_name TEXT NULL,
	endpoint_name TEXT NULL,
	topic_name TEXT NULL,
	subscription_name TEXT NULL,
	message_id TEXT NULL,
	external_request_id TEXT NULL,
	has_response BOOLEAN NOT NULL,
	is_error BOOLEAN NULL,
	duration_nanos INTEGER NULL,
	user_id TEXT NULL,
	test_skipped BOOLEAN NOT NULL DEFAULT FALSE,
	src_file TEXT NULL,
	src_line INTEGER NULL,
	parent_span_id TEXT NULL,
	parent_trace_id TEXT NULL,
	caller_event_id INTEGER NULL,
	PRIMARY KEY (trace_id, span_id)
);
`

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return New(db)
}

// writeRootRequest writes a complete root request span to the store, optionally
// with a parent trace id, and returns the span's own trace id (encoded).
func writeRootRequest(t *testing.T, s *Store, traceID *tracepb2.TraceID, spanID uint64, service, endpoint string, dur time.Duration, isError bool, parent *tracepb2.TraceID) string {
	t.Helper()
	ctx := context.Background()
	meta := &trace2.Meta{AppID: "app"}

	start := &tracepb2.TraceEvent{
		TraceId:   traceID,
		SpanId:    spanID,
		EventTime: timestamppb.New(time.Unix(0, 0)),
		Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
			ParentTraceId: parent,
			Data: &tracepb2.SpanStart_Request{Request: &tracepb2.RequestSpanStart{
				ServiceName:  service,
				EndpointName: endpoint,
			}},
		}},
	}
	var errPb *tracepb2.Error
	if isError {
		errPb = &tracepb2.Error{Msg: "boom"}
	}
	end := &tracepb2.TraceEvent{
		TraceId:   traceID,
		SpanId:    spanID,
		EventTime: timestamppb.New(time.Unix(0, int64(dur))),
		Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{
			DurationNanos: uint64(dur),
			Error:         errPb,
			Data:          &tracepb2.SpanEnd_Request{Request: &tracepb2.RequestSpanEnd{}},
		}},
	}

	if err := s.WriteEvents(ctx, meta, []*tracepb2.TraceEvent{start, end}); err != nil {
		t.Fatalf("write events: %v", err)
	}
	return encodeTraceID(traceID)
}

func listIDs(t *testing.T, s *Store, q *trace2.Query) []string {
	t.Helper()
	q.AppID = "app"
	var ids []string
	err := s.List(context.Background(), q, func(span *tracepb2.SpanSummary) bool {
		ids = append(ids, span.TraceId)
		return true
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return ids
}

func TestList_Filters(t *testing.T) {
	s := newTestStore(t)

	parent := &tracepb2.TraceID{High: 11, Low: 22}
	parentID := encodeTraceID(parent)

	// Child trace triggered by `parent`.
	child := writeRootRequest(t, s, &tracepb2.TraceID{High: 1, Low: 2}, 100,
		"billing", "Charge", 50*time.Millisecond, false, parent)
	// Unrelated slow + errored trace, no parent.
	other := writeRootRequest(t, s, &tracepb2.TraceID{High: 3, Low: 4}, 200,
		"users", "Get", 500*time.Millisecond, true, nil)

	contains := slices.Contains[[]string, string]

	t.Run("service", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{Service: "billing"})
		if !contains(ids, child) || contains(ids, other) {
			t.Fatalf("service filter: got %v", ids)
		}
	})

	t.Run("endpoint", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{Endpoint: "Get"})
		if !contains(ids, other) || contains(ids, child) {
			t.Fatalf("endpoint filter: got %v", ids)
		}
	})

	t.Run("error", func(t *testing.T) {
		isErr := true
		ids := listIDs(t, s, &trace2.Query{IsError: &isErr})
		if !contains(ids, other) || contains(ids, child) {
			t.Fatalf("error filter: got %v", ids)
		}
	})

	t.Run("min_duration", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{MinDurNanos: uint64(100 * time.Millisecond)})
		if !contains(ids, other) || contains(ids, child) {
			t.Fatalf("min duration filter: got %v", ids)
		}
	})

	t.Run("max_duration", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{MaxDurNanos: uint64(100 * time.Millisecond)})
		if !contains(ids, child) || contains(ids, other) {
			t.Fatalf("max duration filter: got %v", ids)
		}
	})

	t.Run("parent_trace_id_match", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{ParentTraceID: parentID})
		if !contains(ids, child) || contains(ids, other) {
			t.Fatalf("parent trace filter: got %v, want only %v", ids, child)
		}
	})

	t.Run("parent_trace_id_no_match", func(t *testing.T) {
		ids := listIDs(t, s, &trace2.Query{ParentTraceID: encodeTraceID(&tracepb2.TraceID{High: 99, Low: 99})})
		if len(ids) != 0 {
			t.Fatalf("parent trace filter (no match): got %v, want none", ids)
		}
	})
}
