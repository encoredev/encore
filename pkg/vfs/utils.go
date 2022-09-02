package vfs

import (
	"io/fs"
	"os"

	"encr.dev/pkg/eerror"
)

func FromDir(workingDir string, predicate func(fileName string) bool) (*VFS, error) {
	dirFS := os.DirFS(workingDir)
	rtn := New()

	if predicate == nil {
		predicate = func(fileName string) bool { return true }
	}

	err := fs.WalkDir(dirFS, ".", func(path string, info fs.DirEntry, err error) error {
		if !info.IsDir() {
			if predicate(path) {
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
