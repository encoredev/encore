package reqtrack

import (
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/traceprovider"
)

// New creates a new RequestTracker.
//
// If traceProvider is nil no traces are generated.
// If streamer is nil no traces are streamed to the platform.
func New(rootLogger zerolog.Logger, streamer TraceStreamer, traceProvider traceprovider.Factory) *RequestTracker {
	if streamer == nil {
		streamer = &noopTraceStreamer{}
	}

	return &RequestTracker{
		streamer:   streamer,
		impl:       newImpl(),
		trace:      traceProvider,
		rootLogger: rootLogger,
	}
}

type RequestTracker struct {
	streamer   TraceStreamer
	impl       reqTrackImpl
	trace      traceprovider.Factory // nil if tracing is not enabled
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

	if !prev.TraceID.IsZero() {
		next.TraceID = prev.TraceID
	}
	if next.ParentSpanID.IsZero() {
		next.ParentSpanID = prev.SpanID
	}
	if next.ParentTraceID.IsZero() {
		next.ParentTraceID = prev.ParentTraceID
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
	Trace  trace2.Logger  // can be nil
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

func (t *RequestTracker) TracingEnabled() bool {
	return t.trace != nil
}
