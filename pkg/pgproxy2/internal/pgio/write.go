package pgio

import (
	"encoding/binary"
	"fmt"
	"io"
)

func AppendUint16(buf []byte, n uint16) []byte {
	wp := len(buf)
	buf = append(buf, 0, 0)
	binary.BigEndian.PutUint16(buf[wp:], n)
	return buf
}

func AppendUint32(buf []byte, n uint32) []byte {
	wp := len(buf)
	buf = append(buf, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(buf[wp:], n)
	return buf
}

func AppendUint64(buf []byte, n uint64) []byte {
	wp := len(buf)
	buf = append(buf, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint64(buf[wp:], n)
	return buf
}

func AppendInt16(buf []byte, n int16) []byte {
	return AppendUint16(buf, uint16(n))
}

func AppendInt32(buf []byte, n int32) []byte {
	return AppendUint32(buf, uint32(n))
}

func AppendInt64(buf []byte, n int64) []byte {
	return AppendUint64(buf, uint64(n))
}

func SetInt32(buf []byte, n int32) {
	binary.BigEndian.PutUint32(buf, uint32(n))
}

type PgMsg interface {
	Encode(dst []byte) []byte
	Decode(src []byte) error
}

func WriteMsg(conn io.Writer, msg PgMsg) error {
	_, err := conn.Write(msg.Encode(nil))
	return err
}

type RawMsg struct {
	Typ byte
	Buf []byte
}

func (r *RawMsg) Decode(src []byte) error {
	return fmt.Errorf("pgio: cannot decode into RawMsg")
}

func (r *RawMsg) Encode(dst []byte) []byte {
	dst = append(dst, r.Typ)
	sp := len(dst)
	dst = AppendInt32(dst, -1)
	dst = append(dst, r.Buf...)
	SetInt32(dst[sp:], int32(len(dst[sp:])))
	return dst
}

func ReadMsg(conn io.Reader) (*RawMsg, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, err
	}
	bodyLen := int(binary.BigEndian.Uint32(hdr[1:5])) - 4
	if bodyLen > (1 << 20) {
		// don't support messages from frontend longer than 1MiB.
		return nil, fmt.Errorf("too long message: %d bytes", bodyLen)
	}
	buf := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return &RawMsg{Typ: hdr[0], Buf: buf}, nil
}
