package mcp

import (
	"testing"
	"time"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

func TestTraceBroker_FanOut(t *testing.T) {
	src := make(chan trace2.NewSpanEvent, 4)
	b := newTraceBroker(src)
	defer b.close()

	subA, cancelA := b.subscribe()
	defer cancelA()
	subB, cancelB := b.subscribe()
	defer cancelB()

	ev := trace2.NewSpanEvent{
		AppID: "app-1",
		Span: &tracepb2.SpanSummary{
			TraceId: "trace-1",
			SpanId:  "span-1",
			Type:    tracepb2.SpanSummary_PUBSUB_MESSAGE,
		},
	}
	src <- ev

	for _, sub := range []<-chan trace2.NewSpanEvent{subA, subB} {
		select {
		case got := <-sub:
			if got.AppID != "app-1" || got.Span.TraceId != "trace-1" {
				t.Fatalf("unexpected event: %+v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
}

func TestTraceBroker_UnsubscribeStopsDelivery(t *testing.T) {
	src := make(chan trace2.NewSpanEvent, 4)
	b := newTraceBroker(src)
	defer b.close()

	sub, cancel := b.subscribe()
	cancel()

	src <- trace2.NewSpanEvent{AppID: "app-1", Span: &tracepb2.SpanSummary{}}

	select {
	case _, ok := <-sub:
		if ok {
			t.Fatal("received event after unsubscribe")
		}
		// channel closed is acceptable
	case <-time.After(100 * time.Millisecond):
		// no delivery is acceptable
	}
}

func TestTraceBroker_SlowSubscriberDoesNotBlock(t *testing.T) {
	src := make(chan trace2.NewSpanEvent, 1)
	b := newTraceBroker(src)
	defer b.close()

	// Subscribe but never read.
	_, cancel := b.subscribe()
	defer cancel()

	// Push more events than the subscriber's buffer.
	for i := 0; i < 100; i++ {
		select {
		case src <- trace2.NewSpanEvent{AppID: "app-1", Span: &tracepb2.SpanSummary{}}:
		case <-time.After(time.Second):
			t.Fatalf("broker blocked on slow subscriber at iteration %d", i)
		}
	}
}

func TestTraceBroker_SubscribeAfterCloseDoesNotPanic(t *testing.T) {
	src := make(chan trace2.NewSpanEvent)
	b := newTraceBroker(src)
	b.close()

	sub, cancel := b.subscribe()
	defer cancel() // must be safe to call

	// The returned channel must be closed (i.e. read returns ok=false immediately).
	select {
	case _, ok := <-sub:
		if ok {
			t.Fatal("expected closed channel, got value")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("read on returned channel did not complete (channel should be already closed)")
	}
}

func TestTraceBroker_CloseIsIdempotent(t *testing.T) {
	src := make(chan trace2.NewSpanEvent)
	b := newTraceBroker(src)

	// Calling close twice must not panic.
	b.close()
	b.close()
}
