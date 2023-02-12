package pkginfo

import (
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/fns"
)

// ParseRelPath parses the package at rp relative to the module root.
// It returns (nil, false) if the directory contains no Go files.
func (m *Module) ParseRelPath(rp RelPath) (pkg *Package, exists bool) {
	dir := filepath.Join(m.rootDir, rp.toFilePath())
	importPath := path.Join(m.Path, string(rp))
	entries, err := os.ReadDir(dir)
	m.l.c.Errs.AssertFile(err, dir)

	s := loadPkgSpec{
		m:          m,
		dir:        dir,
		importPath: importPath,
		filePaths: fns.Map(entries, func(e fs.DirEntry) string {
			return filepath.Join(dir, e.Name())
		}),
	}

	return m.l.parsePkg(s)
}

// ParsePkgPath parses the package with the given import path.
// It throws an error if the package cannot be found.
func (m *Module) ParsePkgPath(importPath string) *Package {
	tr := m.l.c.Trace("pkginfo.Module.ParsePkgPath", "importPath", importPath)
	defer tr.Done()

	cfg := m.pkgsConfig()
	pkgs, err := packages.Load(cfg, "pattern="+importPath)
	tr.Emit("loaded packages", "pkgs", pkgs, "err", err)
	m.l.c.Errs.AssertStd(err)

	var found *packages.Package
	for _, pkg := range pkgs {
		if pkg.PkgPath == importPath {
			found = pkg
			break
		}
	}
	if found == nil {
		m.l.c.Errs.Fatalf(token.NoPos, "cannot find package %q", importPath)
	} else if len(found.GoFiles) == 0 {
		m.l.c.Errs.Fatalf(token.NoPos, "package %q has no Go files", importPath)
	}

	// We've found the package.
	// Determine which pkginfo module it belongs to so we can
	pkgMod := m.l.resolveModule(found.Module)
	result, ok := m.l.parsePkg(loadPkgSpec{
		m:          pkgMod,
		dir:        filepath.Dir(found.GoFiles[0]),
		importPath: found.PkgPath,
		filePaths:  found.GoFiles,
	})
	if !ok {
		m.l.c.Errs.Fatalf(token.NoPos, "package %q has no Go files matching build parameters", importPath)
	}
	return result
}

func (l *Loader) resolveModule(in *packages.Module) *Module {
	l.modulesMu.Lock()
	defer l.modulesMu.Unlock()
	m, ok := l.modules[in.Path]
	if ok {
		return m
	}

	if in.Dir == "" {
		l.c.Errs.Fatalf(token.NoPos, "module %q has no filesystem directory information", in.Path)
	}

	m = &Module{
		l:       l,
		rootDir: in.Dir,
		Path:    in.Path,
		Version: in.Version,
		Main:    in.Main,
	}
	l.modules[in.Path] = m
	return m
}

func (m *Module) pkgsConfig() *packages.Config {
	m.pkgsConfigOnce.Do(func() {
		cgoEnabled := "0"
		if m.l.c.Build.CgoEnabled {
			cgoEnabled = "1"
		}
		m.cachedPkgsConfig = &packages.Config{
			Mode:    packages.NeedName | packages.NeedFiles | packages.NeedModule,
			Context: m.l.c.Ctx,
			Dir:     m.rootDir,
			Env: append(os.Environ(),
				"GOOS="+m.l.c.Build.GOOS,
				"GOARCH="+m.l.c.Build.GOARCH,
				"GOROOT="+m.l.c.Build.GOROOT,
				"CGO_ENABLED="+cgoEnabled,
			),
			Fset:    m.l.c.FS,
			Tests:   m.l.c.ParseTests,
			Overlay: nil,
			Logf: func(format string, args ...any) {
				m.l.c.Log.Debug().Str("component", "pkginfo").Msgf("go/packages: "+format, args...)
			},
		}
	})
	return m.cachedPkgsConfig
}
