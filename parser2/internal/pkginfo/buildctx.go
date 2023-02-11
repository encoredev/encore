package pkginfo

import (
	"go/build"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/fns"
)

func (m *Module) buildCtx() *build.Context {
	m.buildCtxOnce.Do(func() {
		i := m.l.c.Build
		d := &build.Default
		m.cachedBuildCtx = &build.Context{
			GOARCH: i.GOARCH,
			GOOS:   i.GOOS,
			GOROOT: i.GOROOT,

			Dir:         "", // we use fs.FS
			CgoEnabled:  i.CgoEnabled,
			UseAllFiles: false,
			Compiler:    d.Compiler,
			BuildTags:   append(slices.Clone(d.BuildTags), i.BuildTags...),
			ToolTags:    slices.Clone(d.ToolTags),
			ReleaseTags: slices.Clone(d.ReleaseTags),

			// Tweak filesystem access to work with io/fs.
			JoinPath: path.Join,
			IsAbsPath: func(p string) bool {
				return path.IsAbs(p)
			},
			IsDir: func(path string) bool {
				if info, err := fs.Stat(m.fsys, path); err == nil {
					return info.IsDir()
				}
				return false
			},
			ReadDir: func(dir string) ([]fs.FileInfo, error) {
				entries, err := fs.ReadDir(m.fsys, dir)
				if err != nil {
					return nil, err
				}
				return fns.Map(entries, func(e fs.DirEntry) fs.FileInfo {
					return &fileInfoWrapper{
						l:        m.l,
						DirEntry: e,
					}
				}), nil
			},
			OpenFile: func(path string) (io.ReadCloser, error) {
				return m.fsys.Open(path)
			},
			HasSubdir: func(root, dir string) (rel string, ok bool) {
				// Copied from go/build.hasSubdir and adjusted for io/fs.
				const sep = "/"
				root = path.Clean(root)
				if !strings.HasSuffix(root, sep) {
					root += sep
				}
				dir = path.Clean(dir)
				after, found := strings.CutPrefix(dir, root)
				if !found {
					return "", false
				}
				return after, true
			},
		}
	})
	return m.cachedBuildCtx
}

type fileInfoWrapper struct {
	l *Loader
	fs.DirEntry

	once     sync.Once
	statInfo fs.FileInfo
	statErr  error
}

func (f *fileInfoWrapper) Size() int64 {
	return f.info().Size()
}

func (f *fileInfoWrapper) Mode() fs.FileMode {
	return f.info().Mode()
}

func (f *fileInfoWrapper) ModTime() time.Time {
	return f.info().ModTime()
}

func (f *fileInfoWrapper) Sys() any {
	return f.info().Sys()
}

func (i *fileInfoWrapper) info() fs.FileInfo {
	i.once.Do(func() {
		i.statInfo, i.statErr = i.DirEntry.Info()
		if i.statErr != nil {
			i.l.c.Errs.AddForFile(i.statErr, i.DirEntry.Name())
		}
	})
	if i.statErr != nil {
		i.l.c.Errs.Bailout()
	}
	return i.statInfo
}

var _ fs.FileInfo = (*fileInfoWrapper)(nil)
