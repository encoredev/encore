package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

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

const defaultWaitTimeoutMS = 10000

// waitForSubscriptionMessage is the MCP tool handler.
func (m *Manager) waitForSubscriptionMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	topic, ok := request.Params.Arguments["topic"].(string)
	if !ok || topic == "" {
		return nil, fmt.Errorf("missing or invalid topic argument")
	}
	sub, _ := request.Params.Arguments["subscription"].(string)

	timeoutMS := defaultWaitTimeoutMS
	switch v := request.Params.Arguments["timeout_ms"].(type) {
	case float64:
		timeoutMS = int(v)
	case int:
		timeoutMS = v
	}
	if timeoutMS <= 0 {
		return nil, fmt.Errorf("timeout_ms must be a positive integer, got %d", timeoutMS)
	}

	since := time.Now()
	if sinceStr, ok := request.Params.Arguments["since"].(string); ok && sinceStr != "" {
		t, err := time.Parse(time.RFC3339Nano, sinceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid since timestamp: %w", err)
		}
		since = t
	}

	var match map[string]any
	switch v := request.Params.Arguments["match"].(type) {
	case map[string]any:
		match = v
	case string:
		if v != "" {
			if err := json.Unmarshal([]byte(v), &match); err != nil {
				return nil, fmt.Errorf("invalid match argument: %w", err)
			}
		}
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	if err := validateTopicSub(md.PubsubTopics, topic, sub); err != nil {
		return mcp.NewToolResultText(toJSON(map[string]any{"error": err.Error()})), nil
	}

	subCh, cancel := m.broker.subscribe()
	defer cancel()

	res, err := waitForMatch(ctx, waitParams{
		AppID:   inst.PlatformOrLocalID(),
		Topic:   topic,
		Sub:     sub,
		Since:   since,
		Match:   match,
		EventCh: subCh,
		Getter:  m.traces,
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(toJSON(formatWaitResult(topic, sub, timeoutMS, res))), nil
}

func formatWaitResult(topic, sub string, timeoutMS int, res *waitResult) map[string]any {
	if res.Timeout {
		return map[string]any{
			"matched":                   false,
			"timeout":                   true,
			"topic":                     topic,
			"waited_ms":                 timeoutMS,
			"messages_seen_during_wait": res.MessagesSeen,
		}
	}

	outcome := "success"
	if res.Details.HandlerError != "" {
		outcome = "error"
	}

	subscriptionName := res.Span.GetSubscriptionName()
	if sub != "" {
		subscriptionName = sub
	}
	return map[string]any{
		"matched":      true,
		"topic":        topic,
		"subscription": subscriptionName,
		"trace_id":     res.Span.GetTraceId(),
		"message": map[string]any{
			"id":               res.Details.MessageID,
			"published_at":     res.Details.PublishedAt.UTC().Format(time.RFC3339Nano),
			"delivery_attempt": res.Details.Attempt,
			"payload":          rawJSON(res.Details.Payload),
		},
		"handler": map[string]any{
			"outcome":     outcome,
			"duration_ms": res.Details.DurationMS,
			"error":       nilIfEmpty(res.Details.HandlerError),
		},
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// rawJSON returns the payload as decoded JSON when possible, otherwise a string.
func rawJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err == nil {
		return v
	}
	return string(b)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
