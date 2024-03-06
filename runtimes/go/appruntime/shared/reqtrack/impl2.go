package reqtrack

import (
	"sync/atomic"

	"encore.dev/appruntime/exported/model"
)

// taggedGoroutine requests that a goroutine that we're tracking as part of an operation
//
// Note it is not thread safe and is only ever expected to be access by the goroutine it is associated to
type taggedGoroutine struct {
	// id is the identifier assigned to this goroutine;
	// it's only requirement is it must be unique within the associated operation
	id uint32

	// operation is the operation this goroutine is a part of
	operation *trackedOperation

	// request is the request this goroutine is associated to
	//
	// Once set this is immutable until the go routine tag is removed
	request *model.Request

	// spanID is the span ID of the span that this goroutine is associated to
	activeSpan *trackedSpan
}

type trackedOperation struct {
	// id is the Trace ID assigned to this operation
	id model.TraceID

	// nextGoroutineID is the next goroutine ID to assign when a Go routine is spawned
	nextGoroutineID atomic.Uint32
}

func (op *trackedOperation) NextGoroutineID() uint32 {
	return op.nextGoroutineID.Add(1)
}

// trackedSpan represents a span that is part of an operation
type trackedSpan struct {
	// id is the unique identifier assigned to this span
	id model.SpanID

	parent *trackedSpan

	finished atomic.Bool
}
