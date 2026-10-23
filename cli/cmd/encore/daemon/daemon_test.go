package daemon

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/engine/trace2/sqlite"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

// Apps upload traces from many requests at once, and every upload must be stored.
func TestEncoreDBStoresConcurrentTraceUploads(t *testing.T) {
	db, err := (&Daemon{}).openEncoreDB(filepath.Join(t.TempDir(), "encore.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := sqlite.New(db)

	const writers, uploads = 8, 50
	meta := &trace2.Meta{AppID: "app"}
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range uploads {
				traceID := &tracepb2.TraceID{High: uint64(w + 1), Low: uint64(u + 1)}
				if err := store.WriteEvents(context.Background(), meta, requestEvents(traceID)); err != nil {
					t.Errorf("upload %d/%d: %v", w, u, err)
				}
			}
		}()
	}
	wg.Wait()

	var stored int
	if err := db.QueryRow("SELECT COUNT(*) FROM trace_span_index WHERE has_response").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != writers*uploads {
		t.Errorf("%d complete spans stored, want %d", stored, writers*uploads)
	}
}

// requestEvents is the upload for one request: its span's start and end.
func requestEvents(traceID *tracepb2.TraceID) []*tracepb2.TraceEvent {
	return []*tracepb2.TraceEvent{
		{TraceId: traceID, SpanId: 1, EventTime: timestamppb.Now(), Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
			Data: &tracepb2.SpanStart_Request{Request: &tracepb2.RequestSpanStart{ServiceName: "users", EndpointName: "Get"}}}}},
		{TraceId: traceID, SpanId: 1, EventTime: timestamppb.Now(), Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{
			DurationNanos: 1000, Data: &tracepb2.SpanEnd_Request{Request: &tracepb2.RequestSpanEnd{}}}}},
	}
}
