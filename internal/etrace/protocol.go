package etrace

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
)

// See https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU/preview
// for documentation on the Perfetto JSON trace format.

func WithTracer(ctx context.Context, w io.Writer) (context.Context, *Tracer) {
	tr := &Tracer{
		start: time.Now(),
		bw:    bufio.NewWriter(w),
	}
	tr.bw.WriteString("[")
	ctx = context.WithValue(ctx, tracerKey, tr)
	return ctx, tr
}

func WithFileTracer(ctx context.Context, dst string) (context.Context, *Tracer, error) {
	f, err := os.Create(dst)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create file tracer")
	}

	tr := &Tracer{
		start:  time.Now(),
		bw:     bufio.NewWriter(f),
		closer: f,
	}
	tr.bw.WriteString("[")
	ctx = context.WithValue(ctx, tracerKey, tr)
	return ctx, tr, nil
}

func fromCtx(ctx context.Context) *Tracer {
	tr, _ := ctx.Value(tracerKey).(*Tracer)
	return tr
}

type key string

const tracerKey key = "etrace.tracer"

type Tracer struct {
	start  time.Time
	closer io.Closer

	mu sync.Mutex
	bw *bufio.Writer
}

func (tr *Tracer) Close() error {
	if tr == nil {
		return nil
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	err := tr.bw.Flush()

	// Close the file, if any.
	if tr.closer != nil {
		if err2 := tr.closer.Close(); err == nil {
			err = err2
		}
	}

	return err
}

func (tr *Tracer) Emit(typ eventType, name, category string, args map[string]any, gid int64, asyncID int64) {
	if tr == nil {
		return
	}
	ts := time.Since(tr.start).Microseconds()
	data, err := json.Marshal(event{
		Type:      typ,
		Name:      name,
		Category:  category,
		Args:      args,
		Timestamp: ts,
		ProcessID: 1, // TODO
		ThreadID:  gid,
		AsyncID:   asyncID,
	})
	if err != nil {
		panic(err)
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.bw.Write(data)
	tr.bw.WriteByte(',')
}

type eventType string

const (
	beginSync  eventType = "B"
	endSync    eventType = "E"
	beginAsync eventType = "b"
	endAsync   eventType = "e"
)

type event struct {
	// Name is the user-specified name of the traced operation.
	Name string `json:"name"`

	// Category is the user-specified category of the traced operation.
	// There should be a small number of categories.
	Category string `json:"cat"`

	// Type is the type of event.
	Type eventType `json:"ph"`

	// Timestamp is when the event occurred.
	Timestamp int64 `json:"ts"`

	// ProcessID is the id of the process generating the event.
	ProcessID uint64 `json:"pid"`

	// ThreadID is the id of the thread generating the event.
	ThreadID int64 `json:"tid"`

	// Args is the set of arguments for the event.
	Args map[string]any `json:"args,omitempty"`

	// AsyncID is the ID for asynchronous events.
	AsyncID int64 `json:"id,omitempty"`
}
