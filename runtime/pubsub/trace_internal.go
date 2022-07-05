package pubsub

import (
	"encore.dev/internal/stack"
	"encore.dev/runtime"
	"encore.dev/runtime/trace"
)

var (
	publishCounter uint64
)

func tracePublishStart(topic string, msg []byte, spanID trace.SpanID, goid, publishID uint64, skipFrames int) {
	var tb trace.TraceBuf
	tb.UVarint(publishID)
	tb.Bytes(spanID[:])
	tb.UVarint(goid)
	tb.String(topic)
	tb.ByteString(msg)
	tb.Stack(stack.Build(skipFrames))
	runtime.TraceLog(trace.PublishStart, tb.Buf())
}

func tracePublishEnd(publishID uint64, messageID string, err error) {
	var tb trace.TraceBuf

	tb.UVarint(publishID)
	tb.String(messageID)
	tb.Err(err)

	runtime.TraceLog(trace.PublishEnd, tb.Buf())
}
