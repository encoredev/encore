package reqtrack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/model"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/trace"
)

// New creates a new RequestTracker.
// If platform is nil no traces are sent.
func New(rootLogger zerolog.Logger, platform *platform.Client, trace bool) *RequestTracker {
	return &RequestTracker{
		platform:   platform,
		impl:       newImpl(),
		trace:      trace,
		rootLogger: rootLogger,
	}
}

type RequestTracker struct {
	platform   *platform.Client
	impl       reqTrackImpl
	trace      bool // whether tracing is enabled
	rootLogger zerolog.Logger
}

func (t *RequestTracker) BeginOperation() {
	t.beginOp(t.trace)
}

func (t *RequestTracker) FinishOperation() {
	t.finishOp()
}

func (t *RequestTracker) BeginRequest(req *model.Request) {
	if prev, _, _ := t.currentReq(); prev != nil {
		t.clearReq()
	}
	t.beginReq(req, req.Traced)
}

func (t *RequestTracker) FinishRequest() {
	t.finishReq()
}

type Current struct {
	Req   *model.Request // can be nil
	Trace *trace.Log     // can be nil
	Goctr uint32         // zero if Req == nil && Trace == nil
}

func (t *RequestTracker) Current() Current {
	req, tr, goid := t.currentReq()
	return Current{req, tr, goid}
}

func (t *RequestTracker) Logger() *zerolog.Logger {
	if curr := t.Current(); curr.Req != nil && curr.Req.Logger != nil {
		return curr.Req.Logger
	}
	return &t.rootLogger
}

func (t *RequestTracker) sendTrace(tr *trace.Log) {
	// Do this first so we clear the buffer even if t.platform == nil
	data := tr.GetAndClear()

	if t.platform == nil {
		// If we don't have a platform client we can't send traces.
		// This is the case if the app is ejected.
		return
	}

	go func() {
		traceID, err := model.GenTraceID()
		if err != nil {
			fmt.Fprintln(os.Stderr, "encore: could not generate trace id:", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = t.platform.SendTrace(ctx, traceID, bytes.NewReader(data))
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, "encore: could not record trace:", err)
		}
	}()
}
