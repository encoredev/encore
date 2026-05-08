package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

// matchPayload reports whether the JSON-encoded payload satisfies the match
// filter. A nil or empty filter always matches. The filter compares top-level
// keys for deep equality only — no nested paths, no wildcards.
func matchPayload(payload []byte, match map[string]any) bool {
	if len(match) == 0 {
		return true
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false
	}
	for k, want := range match {
		got, ok := decoded[k]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(got, want) {
			return false
		}
	}
	return true
}

// eventGetter abstracts trace2.Store for tests. trace2.Store.GetEvents matches
// this signature exactly.
type eventGetter interface {
	GetEvents(ctx context.Context, appID, traceID, spanID string) ([]*tracepb2.TraceEvent, error)
}

type spanDetails struct {
	MessageID    string
	Attempt      uint32
	PublishedAt  time.Time // zero if missing
	Payload      []byte
	DurationMS   int64
	HandlerError string // empty when handler succeeded
}

// loadSpanDetails fetches the SpanStart and SpanEnd events for the given span
// and synthesizes a spanDetails for the wait response.
func loadSpanDetails(ctx context.Context, store eventGetter, appID, traceID, spanID string) (*spanDetails, error) {
	events, err := store.GetEvents(ctx, appID, traceID, spanID)
	if err != nil {
		return nil, err
	}

	out := &spanDetails{}
	for _, ev := range events {
		switch e := ev.Event.(type) {
		case *tracepb2.TraceEvent_SpanStart:
			pm := e.SpanStart.GetPubsubMessage()
			if pm == nil {
				continue
			}
			out.MessageID = pm.GetMessageId()
			out.Attempt = pm.GetAttempt()
			if pt := pm.GetPublishTime(); pt != nil {
				out.PublishedAt = pt.AsTime()
			}
			out.Payload = pm.GetMessagePayload()
		case *tracepb2.TraceEvent_SpanEnd:
			out.DurationMS = int64(e.SpanEnd.GetDurationNanos() / 1_000_000)
			if errPB := e.SpanEnd.GetError(); errPB != nil {
				out.HandlerError = errPB.GetMsg()
			}
		}
	}
	return out, nil
}
