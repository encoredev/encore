package vfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VFS struct {
	root *directoryContents
}

var (
	_ fs.FS         = (*VFS)(nil)
	_ fs.ReadDirFS  = (*VFS)(nil)
	_ fs.ReadFileFS = (*VFS)(nil)
	_ fs.SubFS      = (*VFS)(nil)
	_ fs.StatFS     = (*VFS)(nil)
)

func New() *VFS {
	return &VFS{root: newDirectoryContents("")}
}

// Open opens the named file.
//
// When Open returns an error, it should be of type *PathError
// with the Op field set to "open", the Path field set to name,
// and the Err field describing the problem.
//
// Open should reject attempts to open names that do not satisfy
// ValidPath(name), returning a *PathError with Err set to
// ErrInvalid or ErrNotExist.
func (v *VFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &Directory{directoryContents: v.root}, nil
	}

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	parts := strings.Split(name, "/")

	pathTravelled := make([]string, 0, len(parts))
	dir := v.root
	pathTravelled = append(pathTravelled, dir.name)
	if len(parts) > 1 {
		for _, subDir := range parts[:len(parts)-1] {

			pathTravelled = append(pathTravelled, subDir)
			dir = dir.childDirs[subDir]
			if dir == nil {
				return nil, &fs.PathError{
					Op:   "open",
					Path: strings.Join(pathTravelled, "/"),
					Err:  fs.ErrNotExist,
				}
			}
		}
	}

	// Return the child
	childName := parts[len(parts)-1]
	if childFile, found := dir.files[childName]; found {
		return &File{fileContents: childFile}, nil
	}
	if childDir, found := dir.childDirs[childName]; found {
		return &Directory{directoryContents: childDir}, nil
	}

	return nil, &fs.PathError{
		Op:   "open",
		Path: strings.Join(pathTravelled, "/"),
		Err:  fs.ErrNotExist,
	}
}

// ReadDir reads the named directory
// and returns a list of directory entries sorted by filename.
func (v *VFS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := v.Open(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "ReadDir",
			Path: name,
			Err:  err,
		}
	}

	switch file := file.(type) {
	case *Directory:
		return file.createEntries(), nil

	default:
		return nil, &fs.PathError{
			Op:   "ReadDir",
			Path: name,
			Err:  errors.New("not a directory"),
		}
	}
}

// ReadFile reads the named file and returns its contents.
// A successful call returns a nil error, not io.EOF.
// (Because ReadFile reads the whole file, the expected EOF
// from the final Read is not treated as an error to be reported.)
//
// The caller is permitted to modify the returned byte slice.
// This method should return a copy of the underlying data.
func (v *VFS) ReadFile(name string) ([]byte, error) {
	file, err := v.Open(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "ReadFile",
			Path: name,
			Err:  err,
		}
	}

	switch file := file.(type) {
	case *File:
		rtnBytes := make([]byte, len(file.contents))
		copy(rtnBytes, file.contents)
		return rtnBytes, nil

	default:
		return nil, &fs.PathError{
			Op:   "ReadFile",
			Path: name,
			Err:  errors.New("not a file"),
		}
	}
}

// Sub returns an FS corresponding to the subtree rooted at dir.
func (v *VFS) Sub(dir string) (fs.FS, error) {
	file, err := v.Open(dir)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "Sub",
			Path: dir,
			Err:  err,
		}
	}

	switch file := file.(type) {
	case *Directory:
		return &VFS{root: file.directoryContents}, nil

	default:
		return nil, &fs.PathError{
			Op:   "Sub",
			Path: dir,
			Err:  errors.New("not a directory"),
		}
	}
}

// Stat returns a FileInfo describing the file.
// If there is an error, it should be of type *PathError.
func (v *VFS) Stat(name string) (fs.FileInfo, error) {
	file, err := v.Open(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "Stat",
			Path: name,
			Err:  err,
		}
	}

	return file.Stat()
}

// AddFile records a file into the VFS
func (v *VFS) AddFile(path string, bytes []byte, time time.Time) (*fileContents, error) {
	dirPath, filename := filepath.Split(path)
	dirParts := strings.Split(dirPath, string(os.PathSeparator))

	dir := v.root
	for _, dirPart := range dirParts {
		// dirPart = strings.TrimSuffix(dirPart, string(os.PathSeparator))

		if dirPart == "." || dirPart == "" {
			continue
		}

		if dirPart == ".." {
			if dir.node.parent == nil {
				return nil, errors.New("cannot go above root of filesystem")
			}

			dir = dir.node.parent
			continue
		}

		child := dir.childDirs[dirPart]
		if child == nil {
			child = newDirectoryContents(dirPart)
			child.node.parent = dir
			dir.childDirs[dirPart] = child
		}
		dir = child
	}

	dir.files[filename] = &fileContents{
		node: node{
			name:    filename,
			parent:  dir,
			modTime: time,
		},
		contents: bytes,
	}

	return dir.files[filename], nil
}
