package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
	metav1 "encr.dev/proto/encore/parser/meta/v1"
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

type waitParams struct {
	AppID   string
	Topic   string
	Sub     string // "" means any subscription on the topic
	Since   time.Time
	Match   map[string]any
	EventCh <-chan trace2.NewSpanEvent
	Getter  eventGetter
	Timeout time.Duration
}

type waitResult struct {
	Matched      bool
	Timeout      bool
	Span         *tracepb2.SpanSummary
	Details      *spanDetails
	MessagesSeen int // count of messages seen during wait that didn't match
}

// waitForMatch reads from EventCh until a span matching the filters arrives,
// the context is canceled, or Timeout elapses. It returns the matched span
// and full event details, or a timeout result.
func waitForMatch(ctx context.Context, p waitParams) (*waitResult, error) {
	deadline := time.NewTimer(p.Timeout)
	defer deadline.Stop()

	res := &waitResult{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			res.Timeout = true
			return res, nil
		case ev, ok := <-p.EventCh:
			if !ok {
				res.Timeout = true
				return res, nil
			}
			if !spanMatches(ev, p) {
				res.MessagesSeen++
				continue
			}
			details, err := loadSpanDetails(ctx, p.Getter, ev.AppID, ev.Span.TraceId, ev.Span.SpanId)
			if err != nil {
				return nil, err
			}
			if !matchPayload(details.Payload, p.Match) {
				res.MessagesSeen++
				continue
			}
			res.Matched = true
			res.Span = ev.Span
			res.Details = details
			return res, nil
		}
	}
}

func validateTopicSub(topics []*metav1.PubSubTopic, topic, sub string) error {
	for _, t := range topics {
		if t.Name != topic {
			continue
		}
		if sub == "" {
			return nil
		}
		for _, s := range t.Subscriptions {
			if s.Name == sub {
				return nil
			}
		}
		return fmt.Errorf("subscription %q not found on topic %q", sub, topic)
	}
	return fmt.Errorf("topic %q not found in current app", topic)
}

func spanMatches(ev trace2.NewSpanEvent, p waitParams) bool {
	if ev.AppID != p.AppID {
		return false
	}
	if ev.Span == nil || ev.Span.Type != tracepb2.SpanSummary_PUBSUB_MESSAGE {
		return false
	}
	if ev.Span.GetTopicName() != p.Topic {
		return false
	}
	if p.Sub != "" && ev.Span.GetSubscriptionName() != p.Sub {
		return false
	}
	if !p.Since.IsZero() && ev.Span.GetStartedAt() != nil {
		if ev.Span.GetStartedAt().AsTime().Before(p.Since) {
			return false
		}
	}
	return true
}
