package tarstream

import (
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
	file *os.File
}

// PadVec is a padding (0s) vec type
type PadVec struct {
	Size int64
}

// Datavec is an interface for all vector types
type Datavec interface {
	Clone() Datavec
	GetSize() int64
	Open() error
	Close()
	ReadAt(b []byte, off int64) (int, error)
}

// GetSize gets the size of the memory vec
func (m MemVec) GetSize() int64 {
	return int64(len(m.Data))
}

// Open opens a memory vec
func (m MemVec) Open() error {
	return nil
}

// Close closes the memory vec
func (m MemVec) Close() {
}

func (m MemVec) Clone() Datavec {
	return m
}

// ReadAt reads at an offset of a memory vec
func (m MemVec) ReadAt(b []byte, off int64) (int, error) {
	var end int64
	if int64(len(m.Data))-off > int64(len(b)) {
		end = off + int64(len(b))
	} else {
		end = off + int64(len(m.Data))
	}
	if end > int64(len(m.Data)) {
		end = int64(len(m.Data))
	}

	n := copy(b, m.Data[off:end])
	if n == 0 {
		return n, io.EOF
	}
	return n, nil
}

// GetSize gets the file size of the path vec
func (p PathVec) GetSize() int64 {
	return p.Info.Size()
}

// Open opens a file represented by a path vec
func (p *PathVec) Open() error {
	var err error
	p.file, err = os.Open(p.Path)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the file represented by the path vec
func (p *PathVec) Close() {
	p.file.Close()
}

// ReadAt reads the file represented by path vec at the given offset
func (p *PathVec) ReadAt(b []byte, off int64) (int, error) {
	n, err := p.file.ReadAt(b, off)
	if err == io.EOF {
		return n, nil
	}
	if n == 0 {
		return n, io.EOF
	}
	return n, err
}

func (p *PathVec) Clone() Datavec {
	// Clone the path vec by creating a new instance with the same path and info
	return &PathVec{
		Path: p.Path,
		Info: p.Info,
		file: nil,
	}
}

// GetSize gets the size of the padding vec
func (p PadVec) GetSize() int64 {
	return p.Size
}

// Open opens the padding vec
func (p PadVec) Open() error {
	return nil
}

// Close closes the padding vec
func (p PadVec) Close() {
}

// ReadAt read the padding vec at a given offset (which is always 0s)
func (p PadVec) ReadAt(b []byte, off int64) (int, error) {
	n := min(int(p.Size-off), len(b))

	if n == 0 {
		return 0, io.EOF
	}
	for i := range n {
		b[i] = 0
	}
	return n, nil
}

func (p PadVec) Clone() Datavec {
	return p
}
