package pkginfo

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"slices"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
)

// File modresolve contains tools for resolving an import path
// into information about the module it belongs to.

// loadModuleFromDisk loads the module information from the given directory.
// It does not consult the module cache; use resolveModule for that.
func (l *Loader) loadModuleFromDisk(rootDir paths.FS, fallbackModPath paths.Mod) (m *Module) {
	tr := l.c.Trace("pkgload.loadModuleFromDisk", "dir", rootDir)
	defer tr.Done("result", m)

	// Load the go.mod file from disk and validate it.
	gomodFilePath := rootDir.Join("go.mod").ToIO()
	data, err := os.ReadFile(gomodFilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && fallbackModPath != "" {
			// The dependency predates Go Modules. Simulate an empty module.
			return &Module{
				RootDir: rootDir,
				Path:    fallbackModPath,
				Version: "v0.0.0-00010101000000-000000000000",
			}
		}

		l.c.Errs.Assert(errReadingGoMod.Wrapping(err).InFile(gomodFilePath))
	}
	modFile, err := modfile.Parse(gomodFilePath, data, nil)
	if err != nil {
		l.c.Errs.Assert(errReadingGoMod.Wrapping(err).InFile(gomodFilePath))
	}
	if !paths.ValidModPath(modFile.Module.Mod.Path) {
		l.c.Errs.Assert(errInvalidModulePath(modFile.Module.Mod.Path).InFile(gomodFilePath))
	}

	m = &Module{
		RootDir: rootDir,
		Path:    paths.MustModPath(modFile.Module.Mod.Path),
		Version: modFile.Module.Mod.Version,
		file:    option.Some(modFile),
	}

	// Parse the dependencies.
	// We ignore other directives (like replace) because they don't impact
	// how package paths are resolved to modules.
	for _, dep := range modFile.Require {
		depModPath := dep.Mod.Path
		// ignore invalid module paths. We could raise an error,
		// but the build step catches dependency issues anyway.
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

// buildListModuleDir returns the module that pkgPath belongs to, and that
// module's directory on disk, looked up from the batched module list.
func (l *Loader) buildListModuleDir(pkgPath paths.Pkg) (modPath paths.Mod, dir string, ok bool) {
	l.moduleDirsOnce.Do(l.loadModuleDirs)
	modPath, ok = findModule(l.sortedModulePaths, pkgPath)
	if !ok {
		return "", "", false
	}
	dir = l.moduleDirs[modPath]
	return modPath, dir, dir != ""
}

// loadModuleDirs finds the directory of every module the app depends on,
// using one `go list -m -e -json all` invocation.
func (l *Loader) loadModuleDirs() {
	l.moduleDirs = make(map[paths.Mod]string)

	// -e makes go list report broken modules in its output
	// instead of failing the whole command.
	goBin := l.c.Build.GOROOT.Join("bin", "go").ToIO()
	cmd := exec.CommandContext(l.c.Ctx, goBin, "list", "-m", "-e", "-json", "all")
	cmd.Dir = l.packagesConfig.Dir
	cmd.Env = l.packagesConfig.Env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// Non-fatal: resolveModuleForPkg falls back to per-module packages.Load.
		l.c.Log.Warn().Err(err).Str("stderr", stderr.String()).
			Msg("go list -m -e -json all failed; falling back to per-module resolution")
		return
	}

	type modError struct{ Err string }
	type modJSON struct {
		Path  string
		Dir   string
		Error *modError
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var m modJSON
		if err := dec.Decode(&m); err != nil {
			// Keep whatever decoded cleanly.
			l.c.Log.Warn().Err(err).Msg("failed to decode go list module output; using partial module list")
			break
		}
		// Skip errored modules and any without a resolved directory.
		if m.Error != nil || m.Path == "" || m.Dir == "" || !paths.ValidModPath(m.Path) {
			continue
		}
		l.moduleDirs[paths.MustModPath(m.Path)] = m.Dir
	}

	l.sortedModulePaths = make([]paths.Mod, 0, len(l.moduleDirs))
	for modPath := range l.moduleDirs {
		l.sortedModulePaths = append(l.sortedModulePaths, modPath)
	}
	slices.Sort(l.sortedModulePaths)
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
		// It's possible to end up here if there are multiple dependencies
		// with module paths that are prefixes of one another.
		//
		// Consider the deps: ["foo", "foo/bar", "foo/bar/baz"].
		// Doing a binary search for "foo/qux" would return (idx=3, exactMatch=false),
		// but the module that contains "foo/qux" is "foo" at idx=0.
		//
		// To handle this case, keep iterating backwards until we find a prefix match.
		for i := idx - 2; i >= 0; i-- {
			if candidate := sortedMods[i]; candidate.LexicallyContains(pkg) {
				return candidate, true
			}
		}

		return "", false
	}
}

// resolveModuleForPkg resolves information about the module that contains a package.
func (l *Loader) resolveModuleForPkg(cause token.Pos, pkgPath paths.Pkg) (result *Module) {
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
		return cached
	}

	tr := l.c.Trace("resolve module for package", "pkgPath", pkgPath)
	defer tr.Done("result", result)

	// Fast path: look up the module's directory in the batched module list
	// instead of spawning a `go list` subprocess per module.
	// realMod can differ from the modPath guess above: if modPath is "foo" but
	// the package actually lives in a separate nested module "foo/bar", realMod
	// is "foo/bar". Cache under realMod, the module the package belongs to.
	if realMod, dir, ok := l.buildListModuleDir(pkgPath); ok {
		l.modulesMu.Lock()
		if cached, ok := l.modules[realMod]; ok {
			l.modulesMu.Unlock()
			return cached
		}
		l.modulesMu.Unlock()

		result = l.loadModuleFromDisk(paths.RootedFSPath(dir, "."), realMod)
		l.modulesMu.Lock()
		defer l.modulesMu.Unlock()
		l.modules[realMod] = result
		return result
	}

	pkgs, err := packages.Load(l.packagesConfig, "pattern="+string(pkgPath))
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

	// Load the module from disk.
	if pkg.Module == nil || pkg.Module.Dir == "" {
		l.c.Errs.Fatalf(cause, "package %q has no module information", pkgPath)
	}
	rootPath := paths.RootedFSPath(pkg.Module.Dir, ".")
	result = l.loadModuleFromDisk(rootPath, modPath)

	// Add the module to the cache.
	l.modulesMu.Lock()
	defer l.modulesMu.Unlock()
	l.modules[modPath] = result
	return result
}
