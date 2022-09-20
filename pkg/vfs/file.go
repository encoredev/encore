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

// Read implements io.Reader
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
