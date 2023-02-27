package pkginfo

import (
	"fmt"
	"go/token"
	"os"

	"golang.org/x/exp/slices"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"encr.dev/v2/internal/paths"
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
		Path:    paths.MustModPath(modFile.Module.Mod.Path),
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
		if m.Path.LexicallyContains(paths.MustPkgPath(depModPath)) {
			m.sortedNestedDeps = append(m.sortedNestedDeps, paths.MustModPath(depModPath))
		} else {
			m.sortedOtherDeps = append(m.sortedOtherDeps, paths.MustModPath(depModPath))
		}
	}
	slices.Sort(m.sortedNestedDeps)
	slices.Sort(m.sortedOtherDeps)

	return m
}

var stdModule = paths.StdlibMod()

// moduleForPkgPath resolves the module path that contains
// the given import path, based on the module information.
// It consults the module path and the module's require directives.
func (l *Loader) moduleForPkgPath(pkgPath paths.Pkg) (modPath paths.Mod, found bool) {
	// We resolve all packages by consulting the versions
	// in the main module, since only the main module's dependencies
	// track exactly which versions are used (due to MVS).
	m := l.mainModule

	// Fast path: first check if it's contained in the main module,
	// since that's what we scan the most frequently.
	if m.Path.LexicallyContains(pkgPath) {
		// The package is rooted within this module.
		// It's possible it's a nested module.
		if nested, ok := findModule(m.sortedNestedDeps, pkgPath); ok {
			return nested, true
		}
		// It belongs to this module.
		return m.Path, true
	}

	// Otherwise fall back to all the other dependencies.
	if modPath, found = findModule(m.sortedOtherDeps, pkgPath); found {
		return modPath, true
	}

	// We couldn't find it in any module the main module depends on.
	// See if it belongs to the standard library.
	if stdModule.LexicallyContains(pkgPath) {
		// The package is rooted within the standard library.
		return stdModule, true
	}

	// Couldn't find it. Give up.
	return "", false
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

// resolveModuleForPkg resolves information about the module that contains a package.
func (l *Loader) resolveModuleForPkg(cause token.Pos, pkgPath paths.Pkg) (result *Module) {
	tr := l.c.Trace("pkgload.resolveModuleForPkg", "pkgPath", pkgPath)
	defer tr.Done("result", result)

	// Which module does this package belong to?
	modPath, found := l.moduleForPkgPath(pkgPath)
	if !found {
		l.c.Errs.Addf(cause, "package %q not found belonging to any module", pkgPath)
		l.c.Errs.Bailout()
		return nil // unreachable
	}

	// Is the module already cached?
	l.modulesMu.Lock()
	cached, ok := l.modules[modPath]
	l.modulesMu.Unlock()
	if ok {
		tr.Emit("found cached module")
		return cached
	}

	pkgs, err := packages.Load(l.packagesConfig, "pattern="+string(pkgPath))
	tr.Emit("loaded packages", "pkgs", pkgs, "err", err)
	l.c.Errs.AssertStd(err)

	var pkg *packages.Package
	for _, candidate := range pkgs {
		if candidate.PkgPath == string(pkgPath) {
			pkg = candidate
			break
		}
	}
	if pkg == nil {
		l.c.Errs.Fatalf(cause, "cannot find package %q", pkgPath)
	} else if len(pkg.Errors) > 0 {
		for _, err := range pkg.Errors {
			l.c.Errs.AddStd(err)
		}
		l.c.Errs.Bailout()
	} else if len(pkg.GoFiles) == 0 {
		l.c.Errs.Fatalf(cause, "package %q has no Go files", pkgPath)
	}

	// Load the module from disk. We have some of the information
	// present in pkg.Module already, but not all of it.
	if modPath == stdModule {
		// If this is the standard library go/packages doesn't return
		// a Module object. Instead look it up from our GOROOT.
		goroot := l.c.Build.GOROOT
		rootPath := goroot.Join("src")

		// Construct a synthetic Module object for the standard library.
		result = &Module{
			l:       l,
			RootDir: rootPath,
			Path:    "std",
			Version: "",
		}
	} else {
		rootPath := paths.RootedFSPath(pkg.Module.Dir, ".")
		result = l.loadModuleFromDisk(rootPath)
	}

	// Add the module to the cache.
	l.modulesMu.Lock()
	defer l.modulesMu.Unlock()
	l.modules[modPath] = result
	return result
}

/*
// ParseRelPath parses the package at rp relative to the module root.
// It returns (nil, false) if the directory contains no Go files.
func (m *Module) ParseRelPath(rp RelPath) (pkg *Package, exists bool) {
	dir := filepath.Join(m.rootDir, rp.toFilePath())
	importPath := path.Join(m.Path, string(rp))
	entries, bailout := os.ReadDir(dir)
	m.l.c.Errs.AssertFile(bailout, dir)

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
	pkgs, bailout := packages.Load(cfg, "pattern="+importPath)
	tr.Emit("loaded packages", "pkgs", pkgs, "bailout", bailout)
	m.l.c.Errs.AssertStd(bailout)

	var found *packages.Package
	for _, pkg := range pkgs {
		if pkg.MustPkgPath == importPath {
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
		importPath: found.MustPkgPath,
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
