package reqtrack

import (
	"sync/atomic"
	_ "unsafe" // for go:linkname

	model2 "encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
)

type reqTrackImpl interface {
	get() *encoreG
	set(e *encoreG)
}

// encoreG tracks per-goroutine Encore-specific data.
// This must match the definition in the encore-go runtime.
type encoreG struct {
	// op is the current operation the goroutine is a part of.
	op *encoreOp

	// req is request-specific data defined in the Encore runtime.
	req *encoreReq

	// goctr is the per-op goroutine counter.
	goctr uint32
}

// encoreOp represents an Encore operation.
// This must match the definition in the encore-go runtime.
type encoreOp struct {
	// t is the RequestTracker this is part of.
	t *RequestTracker

	// start is the start time of the operation
	start int64 // start time of trace from nanotime()

	// trace is the trace log; it is nil if the op is not traced.
	trace *lazyTraceInit

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
	spanID model2.SpanID
	// data is request-specific data defined in the Encore runtime.
	data *model2.Request
}

// beginOp begins a new Encore operation.
// The trace parameter determines if the op is traced.
//
// It tags the current goroutine with the op.
// It panics if the goroutine is already part of an op.
func (t *RequestTracker) beginOp(trace bool) *encoreOp {
	op := t.newOp(trace)
	t.tagG(op, nil)
	return op
}

// newOp creates a new encoreOp.
func (t *RequestTracker) newOp(trace bool) *encoreOp {
	op := &encoreOp{
		t:     t,
		start: nanotime(),
		refs:  1,
	}
	if trace && t.trace != nil {
		op.trace = newLazyTrace(t)
	}
	return op
}

// encoreTagG tags the g as participating in op, and with req
// as its request data.
// It does not increment the ref count, which means req
// must already be an active request.
// g must not already be part of an op.
func (t *RequestTracker) tagG(op *encoreOp, req *encoreReq) (goctr uint32) {
	if t.impl.get() != nil {
		panic("encore.tagG: goroutine already part of another operation")
	}
	goctr = atomic.AddUint32(&op.goidCtr, 1)
	t.impl.set(&encoreG{
		op:    op,
		req:   req,
		goctr: goctr,
	})
	return goctr
}

// finishOp marks an operation as finished
// and unsets the operation tag on the g.
// It must be part of an operation.
func (t *RequestTracker) finishOp() {
	e := t.impl.get()
	if e == nil {
		panic("encore.finishOp: goroutine not in an operation")
	}
	e.op.decRef(false)
	t.impl.set(nil)
}

// incRef increases the op's refcount by one.
func (op *encoreOp) incRef() int32 {
	return atomic.AddInt32(&op.refs, 1)
}

// decRef decreases the op's refcount by one.
// If it reaches zero and the op is traced, it sends off the trace.
func (op *encoreOp) decRef(blockOnTraceSend bool) int32 {
	n := atomic.AddInt32(&op.refs, -1)
	if n == 0 && op.trace != nil {
		op.trace.MarkDone()

		if blockOnTraceSend {
			op.trace.WaitForStreamSent()
		}
	}
	return n
}

// beginReq sets the request data for the current g,
// and increases the ref count on the operation.
// If the g is not part of an op, it creates a new op
// that is bound to the request lifetime.
func (t *RequestTracker) beginReq(data *model2.Request, trace bool) {
	e := t.impl.get()
	req := &encoreReq{spanID: data.SpanID, data: data}
	if e == nil {
		op := t.newOp(trace)
		t.tagG(op, req)
		// Don't increment the op refcount since it starts at one,
		// and this is not a standalone op.
	} else {
		if e.req != nil {
			panic("encore.beginReq: request already running")
		}
		e.op.incRef()
		e.req = req
	}
}

// finishReq completes the request and decreases the
// ref count on the operation.
// The g must be processing a request.
func (t *RequestTracker) finishReq(blockOnTraceSend bool) {
	e := t.impl.get()
	if e == nil {
		panic("encore.finishReq: goroutine not in an operation")
	} else if e.req == nil {
		panic("encore.finishReq: no current request")
	}
	e.op.decRef(blockOnTraceSend)
	e.req = nil
}

func (t *RequestTracker) currentReq() (req *model2.Request, tr trace2.Logger, goctr uint32, svcNum uint16) {
	if g := t.impl.get(); g != nil {
		var tr trace2.Logger
		if g.op != nil && g.op.trace != nil {
			tr = g.op.trace.Logger()
		}
		if g.req != nil {
			req = g.req.data
			svcNum = req.SvcNum
		}
		return req, tr, g.goctr, svcNum
	}
	return nil, nil, 0, 0
}

// encoreClearReq clears request data from the running g
// without decrementing the ref count.
// The g must be processing a request.
func (t *RequestTracker) clearReq() {
	e := t.impl.get()
	if e == nil {
		panic("encore.replaceReq: goroutine not in an operation")
	} else if e.req == nil {
		panic("encore.replaceReq: no current request")
	}
	e.req = nil
}

//go:linkname nanotime runtime.nanotime
func nanotime() int64
