package trace2

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/trace2"
	"encr.dev/pkg/traceparser"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

type Recorder struct {
	s Store
}

func NewRecorder(s Store) *Recorder {
	return &Recorder{s}
}

type RecordData struct {
	Meta         *Meta
	TraceVersion trace2.Version
	Buf          *bufio.Reader
	Anchor       trace2.TimeAnchor
}

func (h *Recorder) RecordTrace(data RecordData) error {
	eventCh := make(chan *tracepb2.TraceEvent, 100)
	go func() {
		defer close(eventCh)
		for {
			ev, err := traceparser.ParseEvent(data.Buf, data.Anchor, data.TraceVersion)
			if ev != nil {
				eventCh <- ev
			}
			if err == nil {
				continue
			}

			// We have an error.
			if !errors.Is(err, io.EOF) {
				log.Error().Err(err).Msg("unable to parse trace")
			}
			return
		}
	}()

	writeEvents := func(ctx context.Context, ev []*tracepb2.TraceEvent) error {
		if len(ev) == 0 {
			return nil
		}
		return h.s.WriteEvents(ctx, data.Meta, ev)
	}

	// pendingWrites are the accumulated events that we have parsed so far
	// that have not yet been written to the store.
	pendingWrites := make([]*tracepb2.TraceEvent, 0, 100)

	flushWrites := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := writeEvents(ctx, pendingWrites); err != nil {
			log.Error().Err(err).Msg("unable to write trace events")
			return
		}

		// Garbage collect the slice if it's too big.
		if cap(pendingWrites) > 1000 {
			pendingWrites = make([]*tracepb2.TraceEvent, 0, 100)
		} else {
			pendingWrites = pendingWrites[:0]
		}
	}

	debounce := time.NewTicker(500 * time.Millisecond)
	defer debounce.Stop()

	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				// No more events.
				flushWrites()
				return nil
			}
			debounce.Reset(500 * time.Millisecond)
			pendingWrites = append(pendingWrites, ev)

			// Flush immediately if we've accumulated a bunch of events
			// since the debounce may never run in a high throughput scenario.
			if len(pendingWrites) >= 100 {
				flushWrites()
			}

		case <-debounce.C:
			flushWrites()
		}
	}
}
