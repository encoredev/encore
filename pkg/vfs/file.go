package vfs

import (
	"io"
	"io/fs"
	"time"
)

type fileContents struct {
	node
	contents []byte
}

type File struct {
	*fileContents
	bytesRead int
	closed    bool
}

var (
	_ fs.File     = (*File)(nil)
	_ fs.FileInfo = (*File)(nil)
	_ fs.DirEntry = (*File)(nil)
)

func (f *File) Name() string {
	return f.node.name
}

// Reader is the interface that wraps the basic Read method.
//
// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered. Even if Read
// returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally
// returns what is available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after
// successfully reading n > 0 bytes, it returns the number of
// bytes read. It may return the (non-nil) error from the same call
// or return the error (and n == 0) from a subsequent call.
// An instance of this general case is that a Reader returning
// a non-zero number of bytes at the end of the input stream may
// return either err == EOF or err == nil. The next Read should
// return 0, EOF.
//
// Callers should always process the n > 0 bytes returned before
// considering the error err. Doing so correctly handles I/O errors
// that happen after reading some bytes and also both of the
// allowed EOF behaviors.
//
// Implementations of Read are discouraged from returning a
// zero byte count with a nil error, except when len(p) == 0.
// Callers should treat a return of 0 and nil as indicating that
// nothing happened; in particular it does not indicate EOF.
//
// Implementations must not retain p.
func (f *File) Read(bytes []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.bytesRead >= len(f.contents) {
		return 0, io.EOF
	}

	bytesRead := copy(bytes, f.contents[f.bytesRead:])
	f.bytesRead += bytesRead
	if f.bytesRead >= len(f.contents) {
		return bytesRead, io.EOF
	}

	return bytesRead, nil
}

func (f *File) Close() error {
	f.closed = true
	return nil
}

func (f *File) Size() int64 {
	return int64(len(f.contents))
}

func (f *File) Mode() fs.FileMode {
	return fs.ModePerm & 0444 // files in this file system are read only
}

func (f *File) ModTime() time.Time {
	return f.node.modTime
}

func (f *File) IsDir() bool {
	return false
}

func (f *File) Sys() any {
	return nil
}

func (f *File) Type() fs.FileMode {
	return f.Mode() & fs.ModeType
}

func (f *File) Info() (fs.FileInfo, error) {
	return f, nil
}

func (f *File) Stat() (fs.FileInfo, error) {
	return f, nil
}
