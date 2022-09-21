package vfs

import (
	"io/fs"
	"os"

	"encr.dev/pkg/eerror"
)

// FromDir creates a Virtual File System (VFS) from the workingDir on the local computer
//
// Only files or folders which match the predicate will be added into the file system
// A nil predicate will result in all files and folders being added into the file system
func FromDir(workingDir string, predicate func(fileName string, info fs.DirEntry) bool) (*VFS, error) {
	dirFS := os.DirFS(workingDir)
	rtn := New()

	if predicate == nil {
		predicate = func(fileName string, info fs.DirEntry) bool { return true }
	}

	err := fs.WalkDir(dirFS, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return eerror.Wrap(err, "vfs", "error walking directory", map[string]any{"path": path})
		}

		if predicate(path, info) {
			if info.IsDir() {
				_ = rtn.AddDir(path)
			} else {
				bytes, err := fs.ReadFile(dirFS, path)
				if err != nil {
					return eerror.Wrap(err, "vfs", "error reading file", map[string]any{"path": path})
				}

				stat, err := fs.Stat(dirFS, path)
				if err != nil {
					return eerror.Wrap(err, "vfs", "error stat file", map[string]any{"path": path})
				}

				if _, err := rtn.AddFile(path, bytes, stat.ModTime()); err != nil {
					return eerror.Wrap(err, "vfs", "unable to add file", map[string]any{"path": path})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return rtn, nil
}
