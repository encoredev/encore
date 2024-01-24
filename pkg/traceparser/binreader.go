package traceparser

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/exported/trace2"
)

var bin = binary.LittleEndian

type traceReader struct {
	buf        *bufio.Reader
	version    trace2.Version
	bytesRead  int
	timeAnchor int64
	err        error // any error encountered during reading
}

func (tr *traceReader) setErr(err error) {
	if tr.err == nil {
		tr.err = err
	}
}

// Err reports any error encountered during reading.
func (tr *traceReader) Err() error {
	return tr.err
}

func (tr *traceReader) Bytes(b []byte) {
	n, err := io.ReadFull(tr.buf, b)
	tr.bytesRead += n
	tr.setErr(err)
}

func (tr *traceReader) Skip(n int) {
	discarded, err := tr.buf.Discard(n)
	tr.bytesRead += discarded
	tr.setErr(err)
}

func (tr *traceReader) Byte() byte {
	b, err := tr.buf.ReadByte()
	tr.setErr(err)
	if err == nil {
		tr.bytesRead++
	}
	return b
}

func (tr *traceReader) Bool() bool {
	return tr.Byte() != 0
}

func (tr *traceReader) String() string {
	return string(tr.ByteString())
}

func (tr *traceReader) ByteString() []byte {
	size := tr.UVarint()
	if (size) == 0 {
		return nil
	}
	b := make([]byte, int(size))
	tr.Bytes(b)
	return b
}

func (tr *traceReader) Time() *timestamppb.Timestamp {
	sec := tr.Int64()
	nsec := tr.Int32()
	t := time.Unix(sec, int64(nsec)).UTC()
	return timestamppb.New(t)
}

func (tr *traceReader) Nanotime() int64 {
	return tr.Int64()
}

func (tr *traceReader) Int32() int32 {
	u := tr.Uint32()
	var v int32
	if u&1 == 0 {
		v = int32(u >> 1)
	} else {
		v = ^int32(u >> 1)
	}
	return v
}

func (tr *traceReader) Uint32() uint32 {
	var buf [4]byte
	tr.Bytes(buf[:])
	return bin.Uint32(buf[:])
}

func (tr *traceReader) Int64() int64 {
	return unsignedToSigned(tr.Uint64())
}

func (tr *traceReader) Uint64() uint64 {
	var buf [8]byte
	tr.Bytes(buf[:])
	return bin.Uint64(buf[:])
}

func (tr *traceReader) Varint() int64 {
	u := tr.UVarint()
	var v int64
	if u&1 == 0 {
		v = int64(u >> 1)
	} else {
		v = ^int64(u >> 1)
	}
	return v
}

func (tr *traceReader) UVarint() uint64 {
	i := 0
	var u uint64
	for {
		b, err := tr.buf.ReadByte()
		tr.setErr(err)
		if err != nil {
			return 0
		}
		tr.bytesRead++
		u |= uint64(b&^0x80) << i
		if b&0x80 == 0 {
			return u
		}
		i += 7
	}
}

func (tr *traceReader) Float32() float32 {
	b := tr.Uint32()
	return math.Float32frombits(b)
}

func (tr *traceReader) Float64() float64 {
	b := tr.Uint64()
	return math.Float64frombits(b)
}

func (tr *traceReader) EventID() trace2.EventID {
	return trace2.EventID(tr.UVarint())
}

func (tr *traceReader) Duration() time.Duration {
	return time.Duration(tr.Varint())
}

func ptrOrNil[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}
	return &val
}

func unsignedToSigned(u uint64) int64 {
	var v int64
	if u&1 == 0 {
		v = int64(u >> 1)
	} else {
		v = ^int64(u >> 1)
	}
	return v
}

type versionFilterReader struct {
	traceReader *traceReader
	filtered    bool
}

func (tr *traceReader) FromVer(version trace2.Version) versionFilterReader {
	return versionFilterReader{traceReader: tr, filtered: tr.version < version}
}

func (tr versionFilterReader) Bool(defaultForOlderVersions bool) bool {
	if tr.filtered {
		return defaultForOlderVersions
	}
	return tr.traceReader.Bool()
}
