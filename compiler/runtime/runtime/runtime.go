package runtime

import (
	"context"
	"net/http"
	"unsafe"
)

// encoreG tracks per-goroutine Encore-specific data.
// This must match the definition in the encore-go runtime.
type encoreG struct {
	// op is the current operation the goroutine is a part of.
	op *encoreOp

	// req is request-specific data defined in the Encore runtime.
	req *encoreReq

	// goid is the per-op goroutine id.
	goid uint32
}

// encoreGetG gets the encore data for the current g, or nil.
//go:linkname encoreGetG runtime.encoreGetG
func encoreGetG() *encoreG

// encoreOp represents an Encore operation.
// This must match the definition in the encore-go runtime.
type encoreOp struct {
	// start is the start time of the operation
	start int64 // start time of trace from nanotime()

	// trace is the trace log; it is nil if the op is not traced.
	trace unsafe.Pointer

	// refs is the op refcount. It is 1 + number of requests
	// that reference this op (see doc comment above).
	// It is accessed atomically.
	refs int32

	// goidCtr is a per-operation goroutine counter, for telling
	// apart goroutines participating in the operation.
	goidCtr uint32
}

// encoreReq represents an Encore API request.
type encoreReq struct {
	// spanID is the request span id.
	spanID SpanID
	// data is request-specific data defined in the Encore runtime.
	data *Request
}

// encoreBeginOp begins a new Encore operation.
// The trace parameter determines whether tracing is enabled.
//
// It tags the current goroutine with the op.
// It panics if the goroutine is already part of an op.
//go:linkname encoreBeginOp runtime.encoreBeginOp
func encoreBeginOp(trace bool) *encoreOp

// encoreFinishOp marks an operation as finished.
// It must be part of an operation.
//go:linkname encoreFinishOp runtime.encoreFinishOp
func encoreFinishOp()

// encoreTraceEvent adds the event to the trace.
// The g must already be part of an operation.
//go:linkname encoreTraceEvent runtime.encoreTraceEvent
func encoreTraceEvent(event TraceEvent, data []byte)

// encoreBeginReq sets the request data for the current g,
// and increases the ref count on the operation.
// It must already be part of an operation.
//go:linkname encoreBeginReq runtime.encoreBeginReq
func encoreBeginReq(spanID SpanID, req *Request, trace bool)

// encoreCompleteReq completes the request and decreases the
// ref count on the operation.
// The g must be processing a request.
//go:linkname encoreCompleteReq runtime.encoreCompleteReq
func encoreCompleteReq()

// encoreClearReq clears request data from the running g
// without decrementing the ref count.
// The g must be processing a request.
//go:linkname encoreClearReq runtime.encoreClearReq
func encoreClearReq()

// encoreTraceID represents an Encore trace id.
type encoreTraceID [16]byte

// encoreSendTrace is called by Encore's go runtime to send a trace.
//go:linkname encoreSendTrace runtime.encoreSendTrace
func encoreSendTrace(data []byte) {
	go asyncSendTrace(data)
}

//go:linkname encoreBeginRoundTrip net/http.encoreBeginRoundTrip
func encoreBeginRoundTrip(req *http.Request) (context.Context, error) {
	return httpBeginRoundTrip(req)
}

//go:linkname encoreFinishRoundTrip net/http.encoreFinishRoundTrip
func encoreFinishRoundTrip(req *http.Request, resp *http.Response, err error) {
	httpCompleteRoundTrip(req, resp, err)
}
