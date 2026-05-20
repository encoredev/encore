package mcp

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
	metav1 "encr.dev/proto/encore/parser/meta/v1"
)

func TestMatchPayload_TopLevelEquality(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		match   map[string]any
		want    bool
	}{
		{
			name:    "no match map matches anything",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   nil,
			want:    true,
		},
		{
			name:    "empty match map matches anything",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{},
			want:    true,
		},
		{
			name:    "single key matches",
			payload: []byte(`{"customerID":"cust_42","amount":10}`),
			match:   map[string]any{"customerID": "cust_42"},
			want:    true,
		},
		{
			name:    "single key mismatches",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{"customerID": "cust_99"},
			want:    false,
		},
		{
			name:    "missing key mismatches",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{"orderID": 7},
			want:    false,
		},
		{
			name:    "number equality with json.Number-like decoding",
			payload: []byte(`{"orderID":7}`),
			match:   map[string]any{"orderID": float64(7)},
			want:    true,
		},
		{
			name:    "all keys must match",
			payload: []byte(`{"a":1,"b":2}`),
			match:   map[string]any{"a": float64(1), "b": float64(3)},
			want:    false,
		},
		{
			name:    "non-JSON payload never matches a non-empty filter",
			payload: []byte("not json"),
			match:   map[string]any{"a": float64(1)},
			want:    false,
		},
		{
			name:    "non-JSON payload matches an empty filter",
			payload: []byte("not json"),
			match:   map[string]any{},
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchPayload(tc.payload, tc.match)
			if got != tc.want {
				t.Fatalf("matchPayload() = %v, want %v", got, tc.want)
			}
		})
	}
}

type fakeTraceStore struct {
	events []*tracepb2.TraceEvent
	// eventsBySpan, if non-nil, overrides events with a per-span lookup.
	eventsBySpan map[string][]*tracepb2.TraceEvent
	// spans is the list returned by List, in store order (newest first).
	spans []*tracepb2.SpanSummary
}

func (f *fakeTraceStore) GetEvents(ctx context.Context, appID, traceID, spanID string) ([]*tracepb2.TraceEvent, error) {
	if f.eventsBySpan != nil {
		return f.eventsBySpan[spanID], nil
	}
	return f.events, nil
}

func (f *fakeTraceStore) List(ctx context.Context, q *trace2.Query, iter trace2.ListEntryIterator) error {
	for _, s := range f.spans {
		if !iter(s) {
			return nil
		}
	}
	return nil
}

func TestLoadSpanDetails_SuccessOutcome(t *testing.T) {
	store := &fakeTraceStore{
		events: []*tracepb2.TraceEvent{
			{
				EventTime: timestamppb.New(time.Unix(1700000000, 0)),
				Event: &tracepb2.TraceEvent_SpanStart{
					SpanStart: &tracepb2.SpanStart{
						Data: &tracepb2.SpanStart_PubsubMessage{
							PubsubMessage: &tracepb2.PubsubMessageSpanStart{
								MessageId:      "msg-1",
								Attempt:        1,
								PublishTime:    timestamppb.New(time.Unix(1699999999, 0)),
								MessagePayload: []byte(`{"orderID":7}`),
							},
						},
					},
				},
			},
			{
				EventTime: timestamppb.New(time.Unix(1700000001, 0)),
				Event: &tracepb2.TraceEvent_SpanEnd{
					SpanEnd: &tracepb2.SpanEnd{
						DurationNanos: 42_000_000,
						Error:         nil,
					},
				},
			},
		},
	}

	details, err := loadSpanDetails(context.Background(), store, "app-1", "trace-1", "span-1")
	if err != nil {
		t.Fatalf("loadSpanDetails returned error: %v", err)
	}
	if details.MessageID != "msg-1" {
		t.Errorf("MessageID = %q, want %q", details.MessageID, "msg-1")
	}
	if details.Attempt != 1 {
		t.Errorf("Attempt = %d, want 1", details.Attempt)
	}
	if string(details.Payload) != `{"orderID":7}` {
		t.Errorf("Payload = %q", string(details.Payload))
	}
	if details.HandlerError != "" {
		t.Errorf("HandlerError = %q, want empty", details.HandlerError)
	}
	if details.DurationMS != 42 {
		t.Errorf("DurationMS = %d, want 42", details.DurationMS)
	}
}

func TestLoadSpanDetails_ErrorOutcome(t *testing.T) {
	store := &fakeTraceStore{
		events: []*tracepb2.TraceEvent{
			{
				Event: &tracepb2.TraceEvent_SpanStart{
					SpanStart: &tracepb2.SpanStart{
						Data: &tracepb2.SpanStart_PubsubMessage{
							PubsubMessage: &tracepb2.PubsubMessageSpanStart{
								MessageId: "msg-2",
							},
						},
					},
				},
			},
			{
				Event: &tracepb2.TraceEvent_SpanEnd{
					SpanEnd: &tracepb2.SpanEnd{
						DurationNanos: 12_000_000,
						Error: &tracepb2.Error{
							Msg: "pq: violates foreign key",
						},
					},
				},
			},
		},
	}

	details, err := loadSpanDetails(context.Background(), store, "app-1", "trace-1", "span-1")
	if err != nil {
		t.Fatalf("loadSpanDetails returned error: %v", err)
	}
	if details.HandlerError == "" {
		t.Errorf("HandlerError should be populated")
	}
	if details.HandlerError != "pq: violates foreign key" {
		t.Errorf("HandlerError = %q", details.HandlerError)
	}
}

func TestWaitForMatch_HappyPath(t *testing.T) {
	ch := make(chan trace2.NewSpanEvent, 4)
	getter := &fakeTraceStore{
		events: []*tracepb2.TraceEvent{
			{
				Event: &tracepb2.TraceEvent_SpanStart{
					SpanStart: &tracepb2.SpanStart{
						Data: &tracepb2.SpanStart_PubsubMessage{
							PubsubMessage: &tracepb2.PubsubMessageSpanStart{
								MessageId:      "msg-1",
								MessagePayload: []byte(`{"customerID":"cust_42"}`),
							},
						},
					},
				},
			},
			{
				Event: &tracepb2.TraceEvent_SpanEnd{
					SpanEnd: &tracepb2.SpanEnd{DurationNanos: 5_000_000},
				},
			},
		},
	}

	go func() {
		ch <- trace2.NewSpanEvent{
			AppID: "app-1",
			Span: &tracepb2.SpanSummary{
				Type:             tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TraceId:          "trace-1",
				SpanId:           "span-1",
				TopicName:        ptr("order-created"),
				SubscriptionName: ptr("audit"),
				StartedAt:        timestamppb.Now(),
			},
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := waitForMatch(ctx, waitParams{
		AppID:   "app-1",
		Topic:   "order-created",
		Sub:     "audit",
		Since:   time.Now().Add(-time.Hour),
		Match:   map[string]any{"customerID": "cust_42"},
		EventCh: ch,
		Getter:  getter,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("waitForMatch returned error: %v", err)
	}
	if !res.Matched {
		t.Fatal("expected matched")
	}
	if res.Details.MessageID != "msg-1" {
		t.Errorf("MessageID = %q", res.Details.MessageID)
	}
}

func ptr[T any](v T) *T { return &v }

func TestValidateTopicAndSub(t *testing.T) {
	cases := []struct {
		name    string
		topics  []*metav1.PubSubTopic
		topic   string
		sub     string
		wantErr string
	}{
		{
			name:   "topic exists, no sub specified",
			topics: []*metav1.PubSubTopic{{Name: "order-created"}},
			topic:  "order-created",
		},
		{
			name: "topic + sub both exist",
			topics: []*metav1.PubSubTopic{
				{Name: "order-created", Subscriptions: []*metav1.PubSubTopic_Subscription{{Name: "audit"}}},
			},
			topic: "order-created",
			sub:   "audit",
		},
		{
			name:    "topic missing",
			topics:  []*metav1.PubSubTopic{{Name: "other"}},
			topic:   "order-created",
			wantErr: `topic "order-created" not found in current app`,
		},
		{
			name: "topic exists, sub missing",
			topics: []*metav1.PubSubTopic{
				{Name: "order-created", Subscriptions: []*metav1.PubSubTopic_Subscription{{Name: "other"}}},
			},
			topic:   "order-created",
			sub:     "audit",
			wantErr: `subscription "audit" not found on topic "order-created"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTopicSub(tc.topics, tc.topic, tc.sub)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestWaitForMatch_Timeout(t *testing.T) {
	ch := make(chan trace2.NewSpanEvent)
	res, err := waitForMatch(context.Background(), waitParams{
		AppID:   "app-1",
		Topic:   "order-created",
		EventCh: ch,
		Getter:  &fakeTraceStore{},
		Timeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Timeout {
		t.Fatal("expected timeout")
	}
	if res.Matched {
		t.Fatal("matched should be false")
	}
}

func TestFindRecentMatch_HitFromLookback(t *testing.T) {
	store := &fakeTraceStore{
		spans: []*tracepb2.SpanSummary{
			{
				Type:             tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TraceId:          "trace-recent",
				SpanId:           "span-recent",
				TopicName:        ptr("site.added"),
				SubscriptionName: ptr("audit-site-added"),
				StartedAt:        timestamppb.New(time.Now().Add(-1 * time.Second)),
			},
		},
		eventsBySpan: map[string][]*tracepb2.TraceEvent{
			"span-recent": {
				{
					Event: &tracepb2.TraceEvent_SpanStart{
						SpanStart: &tracepb2.SpanStart{
							Data: &tracepb2.SpanStart_PubsubMessage{
								PubsubMessage: &tracepb2.PubsubMessageSpanStart{
									MessageId:      "msg-7",
									MessagePayload: []byte(`{"siteID":"s1"}`),
								},
							},
						},
					},
				},
				{
					Event: &tracepb2.TraceEvent_SpanEnd{
						SpanEnd: &tracepb2.SpanEnd{DurationNanos: 3_000_000},
					},
				},
			},
		},
	}

	hit, err := findRecentMatch(context.Background(), store, store, "app-1", "site.added", "audit-site-added", time.Time{}, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit == nil {
		t.Fatal("expected lookback hit")
	}
	if !hit.Matched {
		t.Fatal("expected Matched=true")
	}
	if hit.Details.MessageID != "msg-7" {
		t.Errorf("MessageID = %q", hit.Details.MessageID)
	}
}

func TestFindRecentMatch_LookbackZeroDisables(t *testing.T) {
	store := &fakeTraceStore{
		spans: []*tracepb2.SpanSummary{
			{
				Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TopicName: ptr("site.added"),
				StartedAt: timestamppb.New(time.Now()),
			},
		},
	}
	hit, err := findRecentMatch(context.Background(), store, store, "app-1", "site.added", "", time.Time{}, 0, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit != nil {
		t.Fatal("expected no hit when lookback is zero")
	}
}

func TestFindRecentMatch_OldSpansOutsideWindowIgnored(t *testing.T) {
	store := &fakeTraceStore{
		spans: []*tracepb2.SpanSummary{
			{
				Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TraceId:   "trace-old",
				SpanId:    "span-old",
				TopicName: ptr("site.added"),
				StartedAt: timestamppb.New(time.Now().Add(-30 * time.Second)),
			},
		},
		eventsBySpan: map[string][]*tracepb2.TraceEvent{
			"span-old": {
				{Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					Data: &tracepb2.SpanStart_PubsubMessage{PubsubMessage: &tracepb2.PubsubMessageSpanStart{}},
				}}},
				{Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{}}},
			},
		},
	}

	hit, err := findRecentMatch(context.Background(), store, store, "app-1", "site.added", "", time.Time{}, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit != nil {
		t.Fatal("expected no hit when span is outside the lookback window")
	}
}

func TestFindRecentMatch_TopicMismatchIgnored(t *testing.T) {
	store := &fakeTraceStore{
		spans: []*tracepb2.SpanSummary{
			{
				Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TopicName: ptr("other.topic"),
				StartedAt: timestamppb.New(time.Now()),
			},
		},
	}
	hit, err := findRecentMatch(context.Background(), store, store, "app-1", "site.added", "", time.Time{}, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit != nil {
		t.Fatal("expected no hit on topic mismatch")
	}
}

func TestFindRecentMatch_MatchFilterAppliesToPayload(t *testing.T) {
	store := &fakeTraceStore{
		spans: []*tracepb2.SpanSummary{
			{
				Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TraceId:   "t1",
				SpanId:    "s1",
				TopicName: ptr("site.added"),
				StartedAt: timestamppb.New(time.Now()),
			},
			{
				Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
				TraceId:   "t2",
				SpanId:    "s2",
				TopicName: ptr("site.added"),
				StartedAt: timestamppb.New(time.Now()),
			},
		},
		eventsBySpan: map[string][]*tracepb2.TraceEvent{
			"s1": {
				{Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					Data: &tracepb2.SpanStart_PubsubMessage{PubsubMessage: &tracepb2.PubsubMessageSpanStart{
						MessagePayload: []byte(`{"siteID":"other"}`),
					}},
				}}},
				{Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{}}},
			},
			"s2": {
				{Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					Data: &tracepb2.SpanStart_PubsubMessage{PubsubMessage: &tracepb2.PubsubMessageSpanStart{
						MessageId:      "msg-2",
						MessagePayload: []byte(`{"siteID":"wanted"}`),
					}},
				}}},
				{Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{}}},
			},
		},
	}

	hit, err := findRecentMatch(context.Background(), store, store, "app-1", "site.added", "", time.Time{}, 5*time.Second, map[string]any{"siteID": "wanted"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit == nil {
		t.Fatal("expected hit on the matching payload")
	}
	if hit.Details.MessageID != "msg-2" {
		t.Errorf("MessageID = %q, want msg-2", hit.Details.MessageID)
	}
}

func TestSubscriptionsOnTopic(t *testing.T) {
	topics := []*metav1.PubSubTopic{
		{Name: "site.added", Subscriptions: []*metav1.PubSubTopic_Subscription{
			{Name: "audit-site-added"},
			{Name: "check-site"},
		}},
		{Name: "other"},
	}
	got := subscriptionsOnTopic(topics, "site.added")
	want := []string{"audit-site-added", "check-site"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := subscriptionsOnTopic(topics, "nope"); got != nil {
		t.Errorf("expected nil for unknown topic, got %v", got)
	}
}

func TestFormatWaitResult_TimeoutIncludesSubscriptionsRegistry(t *testing.T) {
	topics := []*metav1.PubSubTopic{
		{Name: "site.added", Subscriptions: []*metav1.PubSubTopic_Subscription{
			{Name: "audit-site-added"},
			{Name: "check-site"},
		}},
	}
	res := &waitResult{Timeout: true, MessagesSeen: 0}
	out := formatWaitResult("site.added", "audit", 10000, res, topics)
	subs, ok := out["subscriptions_on_topic"].([]string)
	if !ok {
		t.Fatalf("expected subscriptions_on_topic []string, got %T (%v)", out["subscriptions_on_topic"], out)
	}
	if len(subs) != 2 || subs[0] != "audit-site-added" || subs[1] != "check-site" {
		t.Errorf("got %v", subs)
	}
}

func TestFormatWaitResult_TimeoutOmitsRegistryWhenTopicUnknown(t *testing.T) {
	res := &waitResult{Timeout: true}
	out := formatWaitResult("site.added", "", 10000, res, nil)
	if _, present := out["subscriptions_on_topic"]; present {
		t.Errorf("subscriptions_on_topic should be absent when topic isn't registered, got %v", out)
	}
}

func TestFormatWaitResult_MatchedDoesNotIncludeRegistry(t *testing.T) {
	topics := []*metav1.PubSubTopic{
		{Name: "site.added", Subscriptions: []*metav1.PubSubTopic_Subscription{{Name: "audit-site-added"}}},
	}
	res := &waitResult{
		Matched: true,
		Span: &tracepb2.SpanSummary{
			TraceId:          "trace-1",
			TopicName:        ptr("site.added"),
			SubscriptionName: ptr("audit-site-added"),
		},
		Details: &spanDetails{MessageID: "m1", PublishedAt: time.Unix(0, 0)},
	}
	out := formatWaitResult("site.added", "audit-site-added", 10000, res, topics)
	if _, present := out["subscriptions_on_topic"]; present {
		t.Errorf("subscriptions_on_topic should not be in the matched-success envelope, got %v", out)
	}
}

func TestWaitForMatch_StaleEventsBeforeSince_Skipped(t *testing.T) {
	ch := make(chan trace2.NewSpanEvent, 4)
	since := time.Now()

	// Stale event from before Since.
	ch <- trace2.NewSpanEvent{
		AppID: "app-1",
		Span: &tracepb2.SpanSummary{
			Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
			TopicName: ptr("order-created"),
			StartedAt: timestamppb.New(since.Add(-time.Hour)),
		},
	}
	// Then a fresh event after Since.
	ch <- trace2.NewSpanEvent{
		AppID: "app-1",
		Span: &tracepb2.SpanSummary{
			Type:      tracepb2.SpanSummary_PUBSUB_MESSAGE,
			TraceId:   "trace-2",
			SpanId:    "span-2",
			TopicName: ptr("order-created"),
			StartedAt: timestamppb.New(since.Add(time.Second)),
		},
	}

	res, err := waitForMatch(context.Background(), waitParams{
		AppID:   "app-1",
		Topic:   "order-created",
		Since:   since,
		EventCh: ch,
		Getter:  &fakeTraceStore{events: []*tracepb2.TraceEvent{}},
		Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Matched {
		t.Fatal("expected matched on the fresh event")
	}
	if res.Span.TraceId != "trace-2" {
		t.Errorf("expected trace-2, got %s", res.Span.TraceId)
	}
	if res.MessagesSeen != 1 {
		t.Errorf("expected 1 stale message seen, got %d", res.MessagesSeen)
	}
}
