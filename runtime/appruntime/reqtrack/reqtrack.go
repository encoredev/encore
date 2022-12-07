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
//
// If traceProvider is nil no traces are generated.
//
// If platform is nil no traces are sent (but are still generated if traceProvider is non-nil).
func New(rootLogger zerolog.Logger, platform *platform.Client, traceProvider trace.Factory) *RequestTracker {
	return &RequestTracker{
		platform:   platform,
		impl:       newImpl(),
		trace:      traceProvider,
		rootLogger: rootLogger,
	}
}

type RequestTracker struct {
	platform   *platform.Client
	impl       reqTrackImpl
	trace      trace.Factory // nil if tracing is not enabled
	rootLogger zerolog.Logger
}

func (t *RequestTracker) BeginOperation() {
	t.beginOp(true /* always trace by default */)
}

func (t *RequestTracker) FinishOperation() {
	t.finishOp()
}

func (t *RequestTracker) BeginRequest(req *model.Request) {
	if prev, _, _, _ := t.currentReq(); prev != nil {
		copyReqInfoFromParent(req, prev)
		t.clearReq()
	}
	t.beginReq(req, req.Traced)
}

// copyReqInfoFromParent copies over relevant request from the parent request.
// If the relevant fields are already set on next they are not copied over.
func copyReqInfoFromParent(next, prev *model.Request) {
	if prevData, nextData := prev.RPCData, next.RPCData; prevData != nil && nextData != nil {
		if nextData.UserID == "" {
			nextData.UserID = prevData.UserID
		}
		if nextData.AuthData == nil {
			nextData.AuthData = prevData.AuthData
		}
	} else if nextData != nil && prev.Test != nil {
		if nextData.UserID == "" {
			nextData.UserID = prev.Test.UserID
		}
		if nextData.AuthData == nil {
			nextData.AuthData = prev.Test.AuthData
		}
	}

	if next.TraceID == (model.TraceID{}) {
		next.TraceID = prev.TraceID
	}
	if next.ParentID == (model.SpanID{}) {
		next.ParentID = prev.SpanID
	}
	if next.CorrelationID == (model.TraceID{}) {
		next.CorrelationID = prev.CorrelationID
	}
	if next.ExtCorrelationID == "" {
		next.ExtCorrelationID = prev.ExtCorrelationID
	}
	if !next.Traced {
		next.Traced = prev.Traced
	}
	if next.Test == nil {
		next.Test = prev.Test
	}
}

func (t *RequestTracker) FinishRequest() {
	t.finishReq()
}

type Current struct {
	Req    *model.Request // can be nil
	Trace  trace.Logger   // can be nil
	Goctr  uint32         // zero if Req == nil && Trace == nil
	SvcNum uint16         // 0 if not in a service
}

func (t *RequestTracker) Current() Current {
	req, tr, goid, svc := t.currentReq()
	return Current{req, tr, goid, svc}
}

func (t *RequestTracker) Logger() *zerolog.Logger {
	if curr := t.Current(); curr.Req != nil && curr.Req.Logger != nil {
		return curr.Req.Logger
	}
	return &t.rootLogger
}

func (t *RequestTracker) sendTrace(tr trace.Logger) {
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
