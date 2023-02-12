package pkginfo

import (
	"go/build"

	"golang.org/x/exp/slices"
)

func (m *Module) buildCtx() *build.Context {
	m.buildCtxOnce.Do(func() {
		i := m.l.c.Build
		d := &build.Default
		m.cachedBuildCtx = &build.Context{
			GOARCH: i.GOARCH,
			GOOS:   i.GOOS,
			GOROOT: i.GOROOT,

			Dir:         m.rootDir,
			CgoEnabled:  i.CgoEnabled,
			UseAllFiles: false,
			Compiler:    d.Compiler,
			BuildTags:   append(slices.Clone(d.BuildTags), i.BuildTags...),
			ToolTags:    slices.Clone(d.ToolTags),
			ReleaseTags: slices.Clone(d.ReleaseTags),
		}
	})
	return m.cachedBuildCtx
}
