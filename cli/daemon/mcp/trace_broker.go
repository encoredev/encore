package mcp

import (
	"sync"

	"encr.dev/cli/daemon/engine/trace2"
)

// subscriberBufSize is the per-subscriber channel buffer.
// If a subscriber is slower than this, events are dropped for that subscriber
// rather than blocking the broker (and thus all other subscribers and the store).
const subscriberBufSize = 16

// traceBroker fans out trace2.NewSpanEvent events to per-call subscribers.
// trace2.Store.Listen only appends listeners and never removes them, so we
// register exactly one channel with the store and broker it ourselves.
type traceBroker struct {
	src <-chan trace2.NewSpanEvent

	mu   sync.Mutex
	subs map[chan trace2.NewSpanEvent]struct{}
	done chan struct{}
}

func newTraceBroker(src <-chan trace2.NewSpanEvent) *traceBroker {
	b := &traceBroker{
		src:  src,
		subs: make(map[chan trace2.NewSpanEvent]struct{}),
		done: make(chan struct{}),
	}
	go b.run()
	return b
}

func (b *traceBroker) run() {
	for {
		select {
		case <-b.done:
			return
		case ev, ok := <-b.src:
			if !ok {
				return
			}
			b.dispatch(ev)
		}
	}
}

func (b *traceBroker) dispatch(ev trace2.NewSpanEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Subscriber is slow; drop rather than block.
		}
	}
}

func (b *traceBroker) subscribe() (<-chan trace2.NewSpanEvent, func()) {
	ch := make(chan trace2.NewSpanEvent, subscriberBufSize)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

func (b *traceBroker) close() {
	close(b.done)
	b.mu.Lock()
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	b.mu.Unlock()
}
