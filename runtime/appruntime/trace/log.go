package trace

import (
	"math"
	"time"
	_ "unsafe" // for go:linkname

	"encore.dev/internal/stack"
)

type Factory interface {
	NewLogger() Logger
}

// DefaultFactory is a Factory that creates regular trace logs.
var DefaultFactory = &defaultFactory{}

type defaultFactory struct{}

func (*defaultFactory) NewLogger() Logger { return &Log{} }

type Log struct {
	// mu must be the runtime mutex and not a regular sync.Mutex,
	// as certain events (Go{Start,Clear,End}) are sometimes executed by system goroutines,
	// which do not support sync.Mutex.
	mu mutex

	data []byte
}

// Ensure Log implements Logger.
var _ Logger = (*Log)(nil)

// Add adds a new event in the trace log.
// If l is nil, it does nothing.
func (l *Log) Add(event EventType, data []byte) {
	if l == nil {
		return
	}
	ln := len(data)
	if ln > (1<<32 - 1) {
		println("encore.traceEvent: event too large, dropping")
		return
	}

	mutexLock(&l.mu)
	defer mutexUnlock(&l.mu)

	// Do this in the critical section to ensure we don't get
	// out-of-order timestamps.
	t := nanotime()
	var b [13]byte
	b[0] = byte(event)
	b[1] = byte(t)
	b[2] = byte(t >> 8)
	b[3] = byte(t >> 16)
	b[4] = byte(t >> 24)
	b[5] = byte(t >> 32)
	b[6] = byte(t >> 40)
	b[7] = byte(t >> 48)
	b[8] = byte(t >> 56)
	b[9] = byte(ln)
	b[10] = byte(ln >> 8)
	b[11] = byte(ln >> 16)
	b[12] = byte(ln >> 24)
	l.data = append(l.data, append(b[:], data...)...)
}

// GetAndClear gets the data and clears the buffer.
func (l *Log) GetAndClear() []byte {
	mutexLock(&l.mu)
	data := l.data
	l.data = l.data[len(l.data):]
	mutexUnlock(&l.mu)
	return data
}

// Buffer is a performant, low-overhead, growable buffer
// for buffering trace data in a compact way.
//
// The zero value is ready to be used, but NewBuffer
// can be used to provide an initial size hint.
type Buffer struct {
	scratch [10]byte
	buf     []byte
}

func NewBuffer(size int) Buffer {
	return Buffer{buf: make([]byte, 0, size)}
}

func (tb *Buffer) Buf() []byte {
	return tb.buf
}

func (tb *Buffer) Byte(b byte) {
	tb.buf = append(tb.buf, b)
}

func (tb *Buffer) Bytes(b []byte) {
	tb.buf = append(tb.buf, b...)
}

func (tb *Buffer) String(s string) {
	tb.UVarint(uint64(len(s)))
	tb.Bytes([]byte(s))
}

func (tb *Buffer) ByteString(b []byte) {
	tb.UVarint(uint64(len(b)))
	tb.Bytes(b)
}

// TruncatedByteString is like ByteString except it truncates b to maximum of maxLen.
// If truncationSuffix is provided, it is appended after truncating, leading to
// the final length being maxLen+len(truncationSuffix).
func (tb *Buffer) TruncatedByteString(b []byte, maxLen int, truncationSuffix []byte) {
	if size := len(b); size > maxLen {
		tb.UVarint(uint64(maxLen + len(truncationSuffix)))
		tb.Bytes(b[:maxLen])
		tb.Bytes(truncationSuffix)
	} else {
		tb.ByteString(b)
	}
}

func (tb *Buffer) Now() {
	now := time.Now()
	tb.Time(now)
}

func (tb *Buffer) Bool(b bool) {
	if b {
		tb.Bytes([]byte{1})
	} else {
		tb.Bytes([]byte{0})
	}
}

func (tb *Buffer) Err(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
		if msg == "" {
			msg = "unknown error"
		}
	}
	tb.String(msg)
}

func (tb *Buffer) Time(t time.Time) {
	tb.Int64(t.Unix())
	tb.Int32(int32(t.Nanosecond()))
}

func (tb *Buffer) Int32(x int32) {
	var u uint32
	if x < 0 {
		u = (^uint32(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint32(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.Uint32(u)
}

func (tb *Buffer) Uint32(x uint32) {
	tb.buf = append(tb.buf,
		byte(x),
		byte(x>>8),
		byte(x>>16),
		byte(x>>24),
	)
}

func (tb *Buffer) Int64(x int64) {
	var u uint64
	if x < 0 {
		u = (^uint64(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint64(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.Uint64(u)
}

func (tb *Buffer) Uint64(x uint64) {
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

func (tb *Buffer) Varint(x int64) {
	var u uint64
	if x < 0 {
		u = (^uint64(x) << 1) | 1 // complement i, bit 0 is 1
	} else {
		u = (uint64(x) << 1) // do not complement i, bit 0 is 0
	}
	tb.UVarint(u)
}

func (tb *Buffer) UVarint(u uint64) {
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

func (tb *Buffer) Float32(f float32) {
	tb.Uint32(math.Float32bits(f))
}

func (tb *Buffer) Float64(f float64) {
	tb.Uint64(math.Float64bits(f))
}

func (tb *Buffer) Stack(s stack.Stack) {
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

//go:linkname nanotime runtime.nanotime
func nanotime() int64
