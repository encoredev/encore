package pkgload

import (
	"go/build"
	"go/token"
	"sync"

	"golang.org/x/exp/slices"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/perr"
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
	mainModule *Module
	buildCtx   *build.Context

	// modules contains loaded module information.
	modulesMu sync.Mutex
	modules   map[paths.Mod]*Module

	// parsed is a cache of parse results, guarded by parsedMu.
	parsedMu sync.Mutex
	parsed   map[paths.Pkg]*parseResult // importPath -> result
}

func (l *Loader) init() {
	// Resolve the main module.
	m := l.loadModuleFromDisk(l.c.MainModuleDir)

	i := l.c.Build
	d := &build.Default
	l.buildCtx = &build.Context{
		GOARCH: i.GOARCH,
		GOOS:   i.GOOS,
		GOROOT: i.GOROOT,

		Dir:         l.c.MainModuleDir.ToIO(),
		CgoEnabled:  i.CgoEnabled,
		UseAllFiles: false,
		Compiler:    d.Compiler,
		BuildTags:   append(slices.Clone(d.BuildTags), i.BuildTags...),
		ToolTags:    slices.Clone(d.ToolTags),
		ReleaseTags: slices.Clone(d.ReleaseTags),
	}
}

// MustLoadPkg loads a package.
// If the package contains no Go files, it bails out.
func (l *Loader) MustLoadPkg(cause token.Pos, pkgPath paths.Pkg) (pkg *Package) {
	pkg, ok := l.LoadPkg(cause, pkgPath)
	if !ok {
		l.c.Errs.Addf(cause, "no buildable Go files in package %q", pkgPath)
		l.c.Errs.Bailout()
	}
	return pkg
}

// LoadPkg loads a package.
// If the package contains no Go files to build, it returns (nil, false).
func (l *Loader) LoadPkg(cause token.Pos, pkgPath paths.Pkg) (pkg *Package, ok bool) {
	tr := l.c.Trace("pkgload.LoadPkg", "pkgPath", pkgPath)
	defer tr.Done("result", pkg, "ok", ok)

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
				// re-bailout
				l.c.Errs.Bailout()
			}
			return result.pkg, result.ok

		case <-l.c.Ctx.Done():
			// The context was cancelled first. Bail out.
			l.c.Errs.Bailout()
			return nil, false
		}
	}

	// Not cached. Do the parsing.
	// Catch any bailout since this runs in a separate goroutine.
	defer func() {
		if _, caught := perr.CatchBailout(recover()); caught {
			result.bailout = true
			// re-bailout
			l.c.Errs.Bailout()
		}
	}()

	targetModule, found := l.mainModule.moduleForPkgPath(pkgPath)
	if !found {

	}

	result.pkg, result.ok = l.doParsePkg(s)
	return result.pkg, result.ok
}
