package trace2

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
	_ "unsafe" // for go:linkname

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/beta/errs"
)

type EventID = model.TraceEventID

// nextEventID is a atomic counter for event IDs.
var nextEventID atomic.Uint64

type Log struct {
	mu   sync.Mutex
	data []byte
}

// Ensure Log implements Logger.
var _ Logger = (*Log)(nil)

type Event struct {
	Type    EventType
	TraceID model.TraceID
	SpanID  model.SpanID
	Data    EventBuffer
}

// Add adds a new event in the trace log.
// If l is nil, it does nothing.
func (l *Log) Add(e Event) EventID {
	if l == nil {
		return 0
	}

	eventData := e.Data.Buf()
	ln := len(eventData)
	if ln > (1<<32 - 1) {
		println("encore.traceEvent: event too large, dropping")
		return 0
	}

	eventID := nextEventID.Add(1)
	if eventID == 0 {
		// We use 0 to indicate "no event" in several places,
		// so don't use that value.
		eventID = nextEventID.Add(1)
	}

	ts := signedToUnsigned(nanotime())
	header := [...]byte{
		// Event type, 1 byte
		byte(e.Type),

		// Event ID, 8 bytes
		byte(eventID),
		byte(eventID >> 8),
		byte(eventID >> 16),
		byte(eventID >> 24),
		byte(eventID >> 32),
		byte(eventID >> 40),
		byte(eventID >> 48),
		byte(eventID >> 56),

		// Timestamp, 8 bytes
		byte(ts),
		byte(ts >> 8),
		byte(ts >> 16),
		byte(ts >> 24),
		byte(ts >> 32),
		byte(ts >> 40),
		byte(ts >> 48),
		byte(ts >> 56),

		// Trace ID, 16 bytes
		e.TraceID[0], e.TraceID[1], e.TraceID[2], e.TraceID[3],
		e.TraceID[4], e.TraceID[5], e.TraceID[6], e.TraceID[7],
		e.TraceID[8], e.TraceID[9], e.TraceID[10], e.TraceID[11],
		e.TraceID[12], e.TraceID[13], e.TraceID[14], e.TraceID[15],

		// Span ID, 8 bytes
		e.SpanID[0], e.SpanID[1], e.SpanID[2], e.SpanID[3],
		e.SpanID[4], e.SpanID[5], e.SpanID[6], e.SpanID[7],

		// Event data length, 4 bytes
		byte(ln),
		byte(ln >> 8),
		byte(ln >> 16),
		byte(ln >> 24),
	}

	l.mu.Lock()
	l.data = append(l.data, append(header[:], eventData...)...)
	l.mu.Unlock()

	return EventID(eventID)
}

// GetAndClear gets the data and clears the buffer.
func (l *Log) GetAndClear() []byte {
	l.mu.Lock()
	data := l.data
	l.data = l.data[len(l.data):]
	l.mu.Unlock()
	return data
}

// EventBuffer is a performant, low-overhead, growable buffer
// for buffering trace data in a compact way.
//
// The zero value is ready to be used, but NewEventBuffer
// can be used to provide an initial size hint.
type EventBuffer struct {
	scratch [10]byte
	buf     []byte
}

func NewEventBuffer(size int) EventBuffer {
	return EventBuffer{buf: make([]byte, 0, size)}
}

func (tb *EventBuffer) Buf() []byte {
	return tb.buf
}

func (tb *EventBuffer) Byte(b byte) {
	tb.buf = append(tb.buf, b)
}

func (tb *EventBuffer) Bytes(b []byte) {
	tb.buf = append(tb.buf, b...)
}

func (tb *EventBuffer) String(s string) {
	tb.UVarint(uint64(len(s)))
	tb.Bytes([]byte(s))
}

func (tb *EventBuffer) ByteString(b []byte) {
	tb.UVarint(uint64(len(b)))
	tb.Bytes(b)
}

// TruncatedByteString is like ByteString except it truncates b to maximum of maxLen.
// If truncationSuffix is provided, it is appended after truncating, leading to
// the final length being maxLen+len(truncationSuffix).
func (tb *EventBuffer) TruncatedByteString(b []byte, maxLen int, truncationSuffix []byte) {
	if size := len(b); size > maxLen {
		tb.UVarint(uint64(maxLen + len(truncationSuffix)))
		tb.Bytes(b[:maxLen])
		tb.Bytes(truncationSuffix)
	} else {
		tb.ByteString(b)
	}
}

func (tb *EventBuffer) Now() {
	now := time.Now()
	tb.Time(now)
}

func (tb *EventBuffer) Bool(b bool) {
	if b {
		tb.Bytes([]byte{1})
	} else {
		tb.Bytes([]byte{0})
	}
}

func (tb *EventBuffer) Err(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
		if msg == "" {
			msg = "unknown error"
		}
	}
	tb.String(msg)
}

func (tb *EventBuffer) ErrWithStack(err error) {
	if err == nil {
		tb.String("")
		return
	}

	msg := err.Error()
	if msg == "" {
		msg = "unknown error"
	}
	tb.String(msg)
	tb.Stack(errs.Stack(err))
}

func (tb *EventBuffer) Time(t time.Time) {
	tb.Int64(t.Unix())
	tb.Int32(int32(t.Nanosecond()))
}

func (tb *EventBuffer) Int32(x int32) {
	var u uint32
	if x < 0 {
		u = (^uint32(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint32(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.Uint32(u)
}

func (tb *EventBuffer) Uint32(x uint32) {
	tb.buf = append(tb.buf,
		byte(x),
		byte(x>>8),
		byte(x>>16),
		byte(x>>24),
	)
}

func (tb *EventBuffer) Int64(i int64) {
	tb.Uint64(signedToUnsigned(i))
}

func (tb *EventBuffer) EventID(id EventID) {
	tb.UVarint(uint64(id))
}

func (tb *EventBuffer) Uint64(x uint64) {
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

func (tb *EventBuffer) Varint(i int64) {
	tb.UVarint(signedToUnsigned(i))
}

func (tb *EventBuffer) UVarint(u uint64) {
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

func (tb *EventBuffer) Float32(f float32) {
	tb.Uint32(math.Float32bits(f))
}

func (tb *EventBuffer) Float64(f float64) {
	tb.Uint64(math.Float64bits(f))
}

func (tb *EventBuffer) Duration(dur time.Duration) {
	tb.Varint(int64(dur))
}

func (tb *EventBuffer) Stack(s stack.Stack) {
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

func signedToUnsigned(i int64) uint64 {
	if i < 0 {
		return (^uint64(i) << 1) | 1 // complement i, bit 0 is 1
	} else {
		return (uint64(i) << 1) // do not complement i, bit 0 is 0
	}
}

//go:linkname nanotime runtime.nanotime
func nanotime() int64
