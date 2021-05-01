package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"time"

	_ "unsafe" // for go:linkname

	"encore.dev/internal/stack"
)

type TraceEvent byte

const (
	RequestStart       TraceEvent = 0x01
	RequestEnd         TraceEvent = 0x02
	GoStart            TraceEvent = 0x03
	GoEnd              TraceEvent = 0x04
	GoClear            TraceEvent = 0x05
	TxStart            TraceEvent = 0x06
	TxEnd              TraceEvent = 0x07
	QueryStart         TraceEvent = 0x08
	QueryEnd           TraceEvent = 0x09
	CallStart          TraceEvent = 0x0A
	CallEnd            TraceEvent = 0x0B
	AuthStart          TraceEvent = 0x0C
	AuthEnd            TraceEvent = 0x0D
	HTTPCallStart      TraceEvent = 0x0E
	HTTPCallEnd        TraceEvent = 0x0F
	HTTPCallBodyClosed TraceEvent = 0x10
	LogMessage         TraceEvent = 0x11
)

// genTraceID generates a new trace id and root span id.
func genTraceID() ([16]byte, error) {
	var traceID [16]byte
	_, err := rand.Read(traceID[:])
	return traceID, err
}

// genSpanID generates a span id.
func genSpanID() (span SpanID, err error) {
	_, err = rand.Read(span[:])
	return
}

func asyncSendTrace(data []byte) {
	if Config.Testing {
		// Don't send traces when running tests
		return
	}

	traceID, err := genTraceID()
	if err != nil {
		fmt.Fprintln(os.Stderr, "encore: could not generate trace id:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = RecordTrace(ctx, traceID, data)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "encore: could not record trace:", err)
	}
}

type TraceBuf struct {
	scratch [10]byte
	buf     []byte
}

func NewTraceBuf(size int) TraceBuf {
	return TraceBuf{buf: make([]byte, 0, size)}
}

func (tb *TraceBuf) Buf() []byte {
	return tb.buf
}

func (tb *TraceBuf) Byte(b byte) {
	tb.buf = append(tb.buf, b)
}

func (tb *TraceBuf) Bytes(b []byte) {
	tb.buf = append(tb.buf, b...)
}

func (tb *TraceBuf) String(s string) {
	tb.UVarint(uint64(len(s)))
	tb.Bytes([]byte(s))
}

func (tb *TraceBuf) ByteString(b []byte) {
	tb.UVarint(uint64(len(b)))
	tb.Bytes(b)
}

func (tb *TraceBuf) Now() {
	now := time.Now()
	tb.Time(now)
}

func (tb *TraceBuf) Bool(b bool) {
	if b {
		tb.Bytes([]byte{1})
	} else {
		tb.Bytes([]byte{0})
	}
}

func (tb *TraceBuf) Err(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
		if msg == "" {
			msg = "unknown error"
		}
	}
	tb.String(msg)
}

func (tb *TraceBuf) Time(t time.Time) {
	tb.Int64(t.Unix())
	tb.Int32(int32(t.Nanosecond()))
}

func (tb *TraceBuf) Int32(x int32) {
	var u uint32
	if x < 0 {
		u = (^uint32(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint32(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.Uint32(u)
}

func (tb *TraceBuf) Uint32(x uint32) {
	tb.buf = append(tb.buf,
		byte(x),
		byte(x>>8),
		byte(x>>16),
		byte(x>>24),
	)
}

func (tb *TraceBuf) Int64(x int64) {
	var u uint64
	if x < 0 {
		u = (^uint64(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint64(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.Uint64(u)
}

func (tb *TraceBuf) Uint64(x uint64) {
	tb.buf = append(tb.buf,
		byte(x),
		byte(x>>8),
		byte(x>>16),
		byte(x>>24),
		byte(x>>32),
		byte(x>>40),
		byte(x>>48),
		byte(x>>56),
	)
}

func (tb *TraceBuf) Varint(x int64) {
	var u uint64
	if x < 0 {
		u = (^uint64(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint64(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.UVarint(u)
}

func (tb *TraceBuf) UVarint(u uint64) {
	i := 0
	for u >= 0x80 {
		tb.scratch[i] = byte(u) | 0x80
		u >>= 7
		i++
	}
	tb.scratch[i] = byte(u)
	i++
	tb.Bytes(tb.scratch[:i])
}

func (tb *TraceBuf) Float32(f float32) {
	tb.Uint32(math.Float32bits(f))
}

func (tb *TraceBuf) Float64(f float64) {
	tb.Uint64(math.Float64bits(f))
}

func (tb *TraceBuf) Stack(s stack.Stack) {
	n := len(s.Frames)
	if n > 0xFF {
		panic("stack too large") // should never happen; it's capped at 100
	}
	tb.Byte(byte(n))
	if n == 0 {
		return
	}

	var prev int64 = 0
	for _, pc := range s.Frames {
		p := int64(pc - s.Off)
		diff := p - prev
		tb.Varint(diff)
		prev = p
	}
}
