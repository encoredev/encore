package pkgload

import (
	"fmt"
	"os"

	"golang.org/x/exp/slices"
	"golang.org/x/mod/modfile"

	"encr.dev/parser2/internal/paths"
)

// File modresolve contains tools for resolving an import path
// into information about the module it belongs to.

// loadModuleFromDisk loads the module information from the given directory.
// It does not consult the module cache; use resolveModule for that.
func (l *Loader) loadModuleFromDisk(rootDir paths.FS) (m *Module) {
	tr := l.c.Trace("pkgload.loadModuleFromDisk", "dir", rootDir)
	defer tr.Done("result", m)

	// Load the go.mod file from disk and validate it.
	gomodFilePath := rootDir.Join("go.mod").ToIO()
	data, err := os.ReadFile(gomodFilePath)
	l.c.Errs.AssertFile(err, gomodFilePath)
	modFile, err := modfile.Parse(gomodFilePath, data, nil)
	l.c.Errs.AssertFile(err, gomodFilePath)
	if !paths.ValidModPath(modFile.Module.Mod.Path) {
		l.c.Errs.AssertFile(fmt.Errorf("invalid module path: %q", modFile.Module.Mod.Path), gomodFilePath)
	}

	m = &Module{
		RootDir: rootDir,
		Path:    paths.ModPath(modFile.Module.Mod.Path),
		Version: modFile.Module.Mod.Version,
		file:    modFile,
	}

	// Parse the dependencies.
	// We ignore other directives (like replace) because they don't impact
	// how package paths are resolved to modules.
	for _, dep := range modFile.Require {
		depModPath := dep.Mod.Path
		// ignore invalid module paths. We could raise an error,
		//but the build step catches dependency issues anyway.
		if !paths.ValidModPath(depModPath) {
			continue
		}
		if m.Path.LexicallyContains(paths.PkgPath(depModPath)) {
			m.sortedNestedDeps = append(m.sortedNestedDeps, paths.ModPath(depModPath))
		} else {
			m.sortedOtherDeps = append(m.sortedOtherDeps, paths.ModPath(depModPath))
		}
	}
	slices.Sort(m.sortedNestedDeps)
	slices.Sort(m.sortedOtherDeps)

	return m
}

// moduleForPkgPath resolves the module path that contains
// the given import path, based on the module information.
// It consults the module path and the module's require directives.
func (m *Module) moduleForPkgPath(pkgPath paths.Pkg) (modPath paths.Mod, found bool) {
	if m.Path.LexicallyContains(pkgPath) {
		// The package is rooted within this module.
		// It's possible it's a nested module.
		if nested, ok := findModule(m.sortedNestedDeps, pkgPath); ok {
			return nested, true
		}
		// It belongs to this module.
		return m.Path, true
	}

	return findModule(m.sortedOtherDeps, pkgPath)
}

// findModule finds the module that contains pkg given a
// sorted list of modules to consult.
func findModule(sortedMods []paths.Mod, pkg paths.Pkg) (modPath paths.Mod, found bool) {
	idx, exactMatch := slices.BinarySearch(sortedMods, paths.Mod(pkg))
	// Two cases to consider: an exact match (unlikely) or a prefix match.
	if exactMatch {
		return sortedMods[idx], true
	}

	// idx represents where the path would be inserted in the list.
	// Since we're interested in prefix matches, we expect the module the
	// package path is contained within to be at idx-1.
	// If idx == 0 the module wasn't found.
	if idx == 0 {
		return "", false
	} else if candidate := sortedMods[idx-1]; candidate.LexicallyContains(pkg) {
		return candidate, true
	} else {
		return "", false
	}
}

/*
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
	tr := m.l.c.Trace("pkgload.Module.ParsePkgPath", "importPath", importPath)
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
	// Determine which module it belongs to so we can
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
				m.l.c.Log.Debug().Str("component", "pkgload").Msgf("go/packages: "+format, args...)
			},
		}
	})
	return m.cachedPkgsConfig
}

*/
