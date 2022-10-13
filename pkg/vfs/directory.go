package vfs

import (
	"errors"
	"io"
	"io/fs"
	"sort"
	"time"
)

type directoryContents struct {
	node
	childDirs map[string]*directoryContents
	files     map[string]*fileContents
}

// Directory is a wrapper around directoryContents which stores the state
// needed to implement ReadDirFile
type Directory struct {
	*directoryContents
	readDirCount int
	readEntries  []fs.DirEntry
	closed       bool
}

var (
	_ fs.FileInfo    = (*Directory)(nil)
	_ fs.DirEntry    = (*Directory)(nil)
	_ fs.ReadDirFile = (*Directory)(nil)
)

func newDirectoryContents(name string) *directoryContents {
	return &directoryContents{
		node: node{
			name:    name,
			parent:  nil,
			modTime: time.Now(),
		},
		childDirs: make(map[string]*directoryContents),
		files:     make(map[string]*fileContents),
	}
}

func (d *Directory) createEntries() []fs.DirEntry {
	rtn := make([]fs.DirEntry, 0, len(d.childDirs)+len(d.files))
	for _, child := range d.childDirs {
		rtn = append(rtn, &Directory{directoryContents: child})
	}

	for _, child := range d.files {
		rtn = append(rtn, &File{fileContents: child})
	}

	sort.SliceStable(rtn, func(i, j int) bool {
		return rtn[i].Name() < rtn[j].Name()
	})
	return rtn
}

// ReadDir reads the contents of the directory and returns
// a slice of up to n DirEntry values in directory order.
// Subsequent calls on the same file will yield further DirEntry values.
//
// If n > 0, ReadDir returns at most n DirEntry structures.
// In this case, if ReadDir returns an empty slice, it will return
// a non-nil error explaining why.
// At the end of a directory, the error is io.EOF.
//
// If n <= 0, ReadDir returns all the DirEntry values from the directory
// in a single slice. In this case, if ReadDir succeeds (reads all the way
// to the end of the directory), it returns the slice and a nil error.
// If it encounters an error before the end of the directory,
// ReadDir returns the DirEntry list read until that point and a non-nil error.
func (d *Directory) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.closed {
		return nil, fs.ErrClosed
	}

	// Create the directory listing if it doesn't exist already
	if d.readEntries == nil {
		d.readEntries = d.createEntries()
	}

	// detect an EOF
	if d.readDirCount == len(d.readEntries) {
		if n <= 0 { // special case return
			return nil, nil
		}

		return nil, io.EOF
	}

	// Create the return data with the number of requested entries
	readNum := len(d.readEntries) - d.readDirCount
	if readNum > n && n > 0 {
		readNum = n
	}
	rtn := make([]fs.DirEntry, readNum)
	for i := 0; i < readNum; i++ {
		rtn[i] = d.readEntries[d.readDirCount]
		d.readDirCount++
	}

	return rtn, nil
}

func (d *Directory) Read(_ []byte) (int, error) {
	return 0, errors.New("cannot read directory")
}

func (d *Directory) Name() string {
	return d.node.name
}

func (d *Directory) IsDir() bool {
	return true
}

func (d *Directory) Type() fs.FileMode {
	return fs.ModeDir
}

func (d *Directory) Info() (fs.FileInfo, error) {
	return d, nil
}

func (d *Directory) Size() int64 {
	return 0
}

func (d *Directory) Mode() fs.FileMode {
	return d.Type()
}

func (d *Directory) ModTime() time.Time {
	return d.node.modTime
}

func (d *Directory) Stat() (fs.FileInfo, error) {
	return d.Info()
}

func (d *Directory) Close() error {
	d.closed = true
	return nil
}

func (d *Directory) Sys() any {
	return nil
}
