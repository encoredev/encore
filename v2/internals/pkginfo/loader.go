package pkginfo

import (
	"go/build"
	"go/token"
	"os"
	"sync"

	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
)

// New creates a new Loader.
func New(c *parsectx.Context) *Loader {
	l := &Loader{
		c:       c,
		modules: make(map[paths.Mod]*Module),
		parsed:  make(map[paths.Pkg]*parseResult),
	}
	l.init()
	return l
}

// A Loader provides lazy loading of package information.
type Loader struct {
	c *parsectx.Context

	// initialized by init.
	mainModule     *Module
	buildCtx       *build.Context
	packagesConfig *packages.Config

	// modules contains loaded module information.
	modulesMu sync.Mutex
	modules   map[paths.Mod]*Module

	// parsed is a cache of parse results, guarded by parsedMu.
	parsedMu sync.Mutex
	parsed   map[paths.Pkg]*parseResult // importPath -> result
}

func (l *Loader) init() {
	// Resolve the main module.
	l.mainModule = l.loadModuleFromDisk(l.c.MainModuleDir, "")
	// Manually cache the main module.
	l.modules[l.mainModule.Path] = l.mainModule
	l.modules["encore.dev"] = &Module{
		l:       l,
		RootDir: l.c.Build.EncoreRuntime,
		Path:    "encore.dev",
		Version: "v1.0.0",
	}

	b := l.c.Build
	d := &build.Default
	l.buildCtx = &build.Context{
		GOARCH: b.GOARCH,
		GOOS:   b.GOOS,
		GOROOT: b.GOROOT.ToIO(),

		Dir:         l.c.MainModuleDir.ToIO(),
		CgoEnabled:  b.CgoEnabled,
		UseAllFiles: false,
		Compiler:    d.Compiler,
		BuildTags:   append(slices.Clone(d.BuildTags), b.BuildTags...),
		ToolTags:    slices.Clone(d.ToolTags),
		ReleaseTags: slices.Clone(d.ReleaseTags),
	}

	// Set up the go/packages configuration for resolving modules.
	cgoEnabled := "0"
	if b.CgoEnabled {
		cgoEnabled = "1"
	}
	l.packagesConfig = &packages.Config{
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedModule,
		Context: l.c.Ctx,
		Dir:     l.c.MainModuleDir.ToIO(),
		Env: append(os.Environ(),
			"GOOS="+b.GOOS,
			"GOARCH="+b.GOARCH,
			"GOROOT="+b.GOROOT.ToIO(),
			"CGO_ENABLED="+cgoEnabled,
		),
		Fset:    l.c.FS,
		Tests:   l.c.ParseTests,
		Overlay: nil,
		Logf: func(format string, args ...any) {
			l.c.Log.Debug().Str("component", "pkgload").Msgf("go/packages: "+format, args...)
		},
	}
}

// MainModule returns the parsed main module.
func (l *Loader) MainModule() *Module {
	return l.mainModule
}

// MustLoadPkg loads a package.
// If the package contains no Go files, it bails out.
func (l *Loader) MustLoadPkg(cause token.Pos, pkgPath paths.Pkg) (pkg *Package) {
	pkg, ok := l.LoadPkg(cause, pkgPath)
	if !ok {
		l.c.Errs.Addf(cause, "could not find package %q", pkgPath)
		l.c.Errs.Bailout()
	}
	return pkg
}

// LoadPkg loads a package.
// If the package contains no Go files to build, it returns (nil, false).
func (l *Loader) LoadPkg(cause token.Pos, pkgPath paths.Pkg) (pkg *Package, ok bool) {
	// Do we have the result cached already?
	l.parsedMu.Lock()
	result, wasCached := l.parsed[pkgPath]
	if !wasCached {
		// Not cached; store a new entry so other goroutines will wait for us.
		result = &parseResult{done: make(chan struct{})}
		l.parsed[pkgPath] = result
		defer close(result.done)
	}
	l.parsedMu.Unlock()

	if wasCached {
		// We have a cached package. Wait for parsing to complete.
		select {
		case <-result.done:
			if result.bailout {
				return nil, false
			}
			return result.pkg, result.ok

		case <-l.c.Ctx.Done():
			// The context was cancelled first.
			return nil, false
		}
	}

	// Not cached. Do the parsing.
	// Catch any err since this runs in a separate goroutine.
	defer func() {
		if _, caught := perr.CatchBailout(recover()); caught {
			result.bailout = true
			pkg = nil
			ok = false
		}
	}()

	module := l.resolveModuleForPkg(cause, pkgPath)
	relPath, ok := module.Path.RelativePathToPkg(pkgPath)
	if !ok {
		return nil, false
	}

	result.pkg, result.ok = l.doParsePkg(loadPkgSpec{
		cause: cause,
		path:  pkgPath,
		dir:   module.RootDir.Join(relPath.ToIO()),
	})
	return result.pkg, result.ok
}
