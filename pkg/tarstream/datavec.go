package tarstream

import (
	"bytes"
	"io"
	"os"
)

// MemVec is a buffer vec type
type MemVec struct {
	Data []byte
}

// PathVec is a filename vec type
type PathVec struct {
	Path string
	Info os.FileInfo
}

// PadVec is a padding (0s) vec type
type PadVec struct {
	Size int64
}

type DataReader interface {
	io.ReaderAt
	io.Closer
}

func nopCloser(r io.ReaderAt) DataReader {
	return noopCloser{r}
}

type noopCloser struct {
	io.ReaderAt
}

func (n noopCloser) Close() error { return nil }

// Datavec is an interface for all vector types
type Datavec interface {
	Clone() Datavec
	GetSize() int64
	Open() (DataReader, error)
}

// GetSize gets the size of the memory vec
func (m MemVec) GetSize() int64 {
	return int64(len(m.Data))
}

func (m MemVec) Clone() Datavec {
	return m
}

// Open opens a memory vec
func (m MemVec) Open() (DataReader, error) {
	return nopCloser(bytes.NewReader(m.Data)), nil
}

// GetSize gets the file size of the path vec
func (p PathVec) GetSize() int64 {
	return p.Info.Size()
}

// Open opens a file represented by a path vec
func (p *PathVec) Open() (DataReader, error) {
	return os.Open(p.Path)
}

func (p *PathVec) Clone() Datavec {
	return p
}

// GetSize gets the size of the padding vec
func (p PadVec) GetSize() int64 {
	return p.Size
}

// Open opens the padding vec
func (p PadVec) Open() (DataReader, error) {
	return padReader{p.Size}, nil
}

func (p PadVec) Clone() Datavec {
	return p
}

type padReader struct {
	size int64
}

func (r padReader) ReadAt(b []byte, off int64) (int, error) {
	rem := int(r.size - off)
	if rem == 0 {
		return 0, io.EOF
	}

	n := min(rem, len(b))
	for i := range n {
		b[i] = 0
	}
	return n, nil
}

func (r padReader) Close() error {
	return nil
}
