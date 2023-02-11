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
	"golang.org/x/sync/singleflight"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/perr"
)

// New creates a new Loader.
func New(c *parsectx.Context) *Loader {
	return &Loader{
		c:      c,
		parsed: make(map[string]parseResult),
	}
}

// A Loader provides lazy loading of package information.
type Loader struct {
	c *parsectx.Context

	// parsing is the group of directories currently being parsed.
	parsing singleflight.Group

	// parsed is a cache of parse results, guarded by parsedMu.
	parsedMu sync.Mutex
	parsed   map[string]parseResult
}

// parseResult is the result from attempting to parse a package.
type parseResult struct {
	pkg     *Package
	ok      bool
	bailout bool
}

// parseDir parses a single directory relative to m.
// It returns (nil, false) if the directory contains no Go files.
func (l *Loader) parseDir(m *Module, relPath string) (pkg *Package, ok bool) {
	defer l.c.Trace("pkginfo.parseDir", "module", m, "path", relPath)()
	key := m.key() + ":" + relPath

	// Do we have the result cached already?
	l.parsedMu.Lock()
	cached, ok := l.parsed[key]
	l.parsedMu.Unlock()
	if ok {
		if cached.bailout {
			// re-bailout from the original goroutine
			l.c.Errs.Bailout()
		}
		return cached.pkg, cached.ok
	}

	// Not cached. Use a singleflight group to deduplicate multiple
	// concurrent requests for the same package.
	ch := m.l.parsing.DoChan(key, func() (res any, err error) {
		// Catch any bailout since this runs in a separate goroutine.
		defer func() {
			if _, caught := perr.CatchBailout(recover()); caught {
				res = parseResult{nil, false, true}
			}
			l.parsedMu.Lock()
			l.parsed[key] = res.(parseResult)
			l.parsedMu.Unlock()
		}()

		pkg, ok := m.l.parsePkg(m, relPath)
		return parseResult{pkg, ok, false}, nil
	})

	// Wait for the result, or bail out if the context is cancelled.
	select {
	case res := <-ch:
		r := res.Val.(parseResult)
		if r.bailout {
			l.c.Errs.Bailout()
		}
		return r.pkg, r.ok
	case <-m.l.c.Ctx.Done():
		l.c.Errs.Bailout()
		return nil, false // unreachable
	}
}

// parseLocalPkg parses a single package in the given directory.
// It returns (nil, false) if the directory contains no Go files.
func (l *Loader) parsePkg(m *Module, relPath string) (pkg *Package, ok bool) {
	entries, err := fs.ReadDir(m.fsys, relPath)
	l.c.Errs.AssertFile(err, relPath)

	l.c.Errs.BailoutOnErrors(func() {
		astPkgs, files := l.parseAST(m, relPath, entries)
		pkg = l.processPkg(m, relPath, astPkgs, files)
	})
	return pkg, pkg != nil
}

// processPkg combines the results of parsing a package into a single *Package.
func (l *Loader) processPkg(m *Module, relPath string, pkgs []*ast.Package, files []*File) *Package {
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
			l.c.Errs.AddPosition(token.Position{Filename: relPath}, fmt.Sprintf("found multiple packages: %s", names))
		}
	} else if n == 0 {
		// No Go files; ignore directory
		return nil
	}

	p := pkgs[0]
	pkg := &Package{
		Module:     m,
		AST:        p,
		Name:       p.Name,
		RelPath:    relPath,
		ImportPath: path.Join(m.Path, relPath),
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
func (l *Loader) parseAST(m *Module, dir string, list []fs.DirEntry) ([]*ast.Package, []*File) {
	// Ensure deterministic parsing order.
	slices.SortFunc(list, func(a, b fs.DirEntry) bool { return a.Name() < b.Name() })

	shouldParseFile := func(f fs.DirEntry) bool {
		name := f.Name()
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

	for _, d := range list {
		if !shouldParseFile(d) {
			continue
		}
		filePath := path.Join(dir, d.Name())
		contents, err := fs.ReadFile(m.fsys, filePath)
		if err != nil {
			l.c.Errs.AddForFile(err, filePath)
			continue
		}

		// Check if this file should be part of the build
		matched, err := m.buildCtx().MatchFile(dir, d.Name())
		if err != nil {
			l.c.Errs.AddForFile(err, filePath)
			continue
		} else if !matched {
			continue
		}

		// Parse the package and imports only so code can consult that.
		// We parse the full AST on-demand later.
		mode := goparser.ParseComments | goparser.ImportsOnly
		astFile, err := goparser.ParseFile(l.c.FS, filePath, contents, mode)
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

		pkg.Files[filePath] = astFile

		fileName := d.Name()
		isTestFile := strings.HasSuffix(fileName, "_test.go") || strings.HasSuffix(pkgName, "_test")
		files = append(files, &File{
			Name:     fileName,
			Path:     filePath,
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
		if path, err := strconv.Unquote(s.Path.Value); err == nil {
			imports[path] = true
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
