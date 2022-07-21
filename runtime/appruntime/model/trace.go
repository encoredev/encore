package model

import (
	"crypto/rand"
	_ "unsafe"
)

type (
	TraceID [16]byte
	SpanID  [8]byte
)

// GenTraceID generates a new trace id.
func GenTraceID() (TraceID, error) {
	var traceID TraceID
	_, err := rand.Read(traceID[:])
	return traceID, err
}

// GenSpanID generates a span id.
func GenSpanID() (SpanID, error) {
	var span SpanID
	_, err := rand.Read(span[:])
	return span, err
}
