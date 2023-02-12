package pkginfo

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/slices"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/perr"
	"encr.dev/pkg/fns"
)

// New creates a new Loader.
func New(c *parsectx.Context) *Loader {
	return &Loader{
		c:       c,
		parsed:  make(map[string]*parseResult),
		modules: make(map[string]*Module),
	}
}

// A Loader provides lazy loading of package information.
type Loader struct {
	c *parsectx.Context

	// parsed is a cache of parse results, guarded by parsedMu.
	parsedMu sync.Mutex
	parsed   map[string]*parseResult

	// modules contains the modules we've loaded, guarded by modulesMu.
	modulesMu sync.Mutex
	modules   map[string]*Module
}

// parseResult is the result from attempting to parse a package.
type parseResult struct {
	done    chan struct{} // closed when parsing is completed
	pkg     *Package
	ok      bool
	bailout bool
}

// loadPkgSpec is the specification for how to load a package.
type loadPkgSpec struct {
	// m is the module containing the package.
	m *Module

	// dir is the directory containing the package
	dir string

	// importPath is the package's import path.
	importPath string

	// filePaths are the file paths to parse.
	// They may be relative or absolute.
	filePaths []string
}

// parsePkg parses a single package.
// It returns (nil, false) if the directory contains no Go files.
func (l *Loader) parsePkg(s loadPkgSpec) (pkg *Package, ok bool) {
	tr := l.c.Trace("pkginfo.parsePkg", "spec", s)
	defer tr.Done("result", pkg, "ok", ok)

	key := fmt.Sprintf("%s:%s", s.m.key(), s.importPath)

	// Do we have the result cached already?
	l.parsedMu.Lock()
	result, wasCached := l.parsed[key]
	if !wasCached {
		// Not cached; store a new entry so other goroutines will wait for us.
		result = &parseResult{done: make(chan struct{})}
		l.parsed[key] = result
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

	result.pkg, result.ok = l.doParsePkg(s)
	return result.pkg, result.ok
}

// doParsePkg parses a single package in the given directory.
// It returns (nil, false) if the directory contains no Go files.
func (l *Loader) doParsePkg(s loadPkgSpec) (pkg *Package, ok bool) {
	l.c.Errs.BailoutOnErrors(func() {
		astPkgs, files := l.parseAST(s)
		pkg = l.processPkg(s, astPkgs, files)
	})
	return pkg, pkg != nil
}

// processPkg combines the results of parsing a package into a single *Package.
func (l *Loader) processPkg(s loadPkgSpec, pkgs []*ast.Package, files []*File) *Package {
	if n := len(pkgs); n > 1 {
		// Make sure the extra packages are just "_test" packages.
		// Pull out the package names.
		var pkgNames []string
		for _, pkg := range pkgs {
			pkgNames = append(pkgNames, pkg.Name)
		}
		sort.Strings(pkgNames)
		if n == 2 && pkgNames[1] == pkgNames[0]+"_test" {
			// We're good
		} else {
			names := strings.Join(pkgNames[:n-1], ", ") + " and " + pkgNames[n-1]
			l.c.Errs.AddPosition(token.Position{Filename: s.dir}, fmt.Sprintf("found multiple packages: %s", names))
		}
	} else if n == 0 {
		// No Go files; ignore directory
		return nil
	}

	p := pkgs[0]
	pkg := &Package{
		Module:     s.m,
		AST:        p,
		Name:       p.Name,
		ImportPath: s.importPath,
		Files:      files,
		Imports:    make(map[string]bool),
	}

	for _, f := range files {
		f.Pkg = pkg
		// Fill in imports.
		for importPath := range f.Imports {
			pkg.Imports[importPath] = true
		}

		// Fill in package docs.
		if pkg.Doc == "" && !f.TestFile && f.initialAST.Doc != nil {
			pkg.Doc = f.initialAST.Doc.Text()
		}
	}

	return pkg
}

// parseAST is like go/parser.ParseDir but it constructs *File objects instead.
func (l *Loader) parseAST(s loadPkgSpec) ([]*ast.Package, []*File) {
	if len(s.filePaths) == 0 {
		return nil, nil
	}

	type fileInfo struct {
		path     string
		baseName string
	}
	infos := fns.Map(s.filePaths, func(path string) fileInfo {
		return fileInfo{path: path, baseName: filepath.Base(path)}
	})

	// Ensure deterministic parsing order.
	slices.SortFunc(infos, func(a, b fileInfo) bool {
		return a.baseName < b.baseName
	})

	shouldParseFile := func(info fileInfo) bool {
		name := info.baseName
		switch {
		// Don't parse encore.gen.go files, since they're not intended to be checked in.
		// We've had several issues where things work locally but not in CI/CD because
		// the encore.gen.go file was parsed for local development which papered over issues.
		case strings.Contains(name, "encore.gen.go"):
			return false
		case !l.c.ParseTests && strings.HasSuffix(name, "_test.go"):
			return false
		case !strings.HasSuffix(name, ".go"):
			return false
		default:
			return true
		}
	}

	var pkgs []*ast.Package
	var files []*File
	seenPkgs := make(map[string]*ast.Package) // package name -> pkg

	for _, d := range infos {
		if !shouldParseFile(d) {
			continue
		}
		contents, err := os.ReadFile(d.path)
		if err != nil {
			l.c.Errs.AddForFile(err, d.path)
			continue
		}

		// Check if this file should be part of the build
		matched, err := s.m.buildCtx().MatchFile(s.dir, d.baseName)
		if err != nil {
			l.c.Errs.AddForFile(err, d.path)
			continue
		} else if !matched {
			continue
		}

		// Parse the package and imports only so code can consult that.
		// We parse the full AST on-demand later.
		mode := goparser.ParseComments | goparser.ImportsOnly
		astFile, err := goparser.ParseFile(l.c.FS, d.path, contents, mode)
		if err != nil {
			l.c.Errs.AddStd(err)
			continue
		}

		pkgName := astFile.Name.Name
		pkg, found := seenPkgs[pkgName]
		if !found {
			pkg = &ast.Package{
				Name:  pkgName,
				Files: make(map[string]*ast.File),
			}
			seenPkgs[pkgName] = pkg
			pkgs = append(pkgs, pkg)
		}

		pkg.Files[d.path] = astFile

		isTestFile := strings.HasSuffix(d.baseName, "_test.go") || strings.HasSuffix(pkgName, "_test")
		files = append(files, &File{
			Name:     d.baseName,
			Path:     d.path,
			Pkg:      nil, // will be set later
			Imports:  getFileImports(astFile),
			Contents: contents,
			TestFile: isTestFile,

			initialAST: astFile,
		})
	}

	return pkgs, files
}

func getFileImports(f *ast.File) map[string]bool {
	imports := make(map[string]bool)
	for _, s := range f.Imports {
		if importPath, err := strconv.Unquote(s.Path.Value); err == nil {
			imports[importPath] = true
		}
	}
	return imports
}

// walkFunc is the callback called by walkDirs to process a directory.
// dir is the path to the current directory; relPath is the (slash-separated)
// relative path from the original root dir.
type walkFunc func(dir, relPath string, files []fs.DirEntry) error

// walkDirs is like filepath.Walk but it calls walkFn once for each directory and not for individual files.
// It also reports both the full path and the path relative to the given root dir.
// It does not allow skipping directories in any way; any error returned from walkFn aborts the walk.
func walkDirs(root string, walkFn walkFunc) error {
	return walkDir(root, ".", walkFn)
}

// walkDir processes a single directory and recurses.
// dir is the current directory path, and rel is the relative path from the original root.
// rel is always in slash form, while dir uses the OS-native filepath separator.
func walkDir(dir, rel string, walkFn walkFunc) error {
	if ignored(dir) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Split the files and dirs
	files := make([]fs.DirEntry, 0, len(entries))
	var dirs []fs.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	if err := walkFn(dir, rel, files); err != nil {
		return err
	}

	for _, d := range dirs {
		dir2 := filepath.Join(dir, d.Name())
		rel2 := path.Join(rel, d.Name())
		if err := walkDir(dir2, rel2, walkFn); err != nil {
			return err
		}
	}
	return nil
}

// ignored returns true if a given directory should be ignored for parsing.
func ignored(dir string) bool {
	name := filepath.Base(filepath.Clean(dir))
	if strings.EqualFold(name, "node_modules") {
		return true
	}
	// Don't watch hidden folders like `.git` or `.idea`.
	if len(name) > 1 && name[0] == '.' {
		return true
	}
	return false
}
