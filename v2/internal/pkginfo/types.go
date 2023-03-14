package pkginfo

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/inspector"

	"encr.dev/v2/internal/paths"
)

// Module describes a Go module.
type Module struct {
	l *Loader // the loader that created it.

	RootDir paths.FS  // the dir containing go.mod
	Path    paths.Mod // module path
	Version string    // module version

	// file is the parsed modfile.
	file *modfile.File

	// sortedNestedDeps and sortedOtherDeps contain lists of
	// this module's dependencies, categorized into nested dependencies
	// (with module paths rooted within this module) and other dependencies (the rest).
	//
	// The lists are sorted to facilitate efficient lookups
	// to determine the right module to query for a given import path.
	sortedNestedDeps []paths.Mod
	sortedOtherDeps  []paths.Mod
}

type Package struct {
	l          *Loader // the loader that created it.
	AST        *ast.Package
	Name       string
	Doc        string
	ImportPath paths.Pkg
	FSPath     paths.FS
	Files      []*File
	Imports    map[paths.Pkg]bool // union of all imports from files

	namesOnce  sync.Once
	namesCache *PkgNames
}

func (p *Package) GoString() string {
	return fmt.Sprintf("&pkginfo.Package{ImportPath: %q, Name: %q}", p.ImportPath, p.Name)
}

// Names returns the computed package-level names.
func (p *Package) Names() *PkgNames {
	p.namesOnce.Do(func() {
		p.namesCache = resolvePkgNames(p)
	})
	return p.namesCache
}

type File struct {
	l        *Loader            // the loader that created it.
	Name     string             // file name ("foo.go")
	Pkg      *Package           // package it belongs to
	FSPath   paths.FS           // where the file lives on disk
	Imports  map[paths.Pkg]bool // imports in the file, keyed by import path
	TestFile bool               // whether the file is a test file

	// initialAST is the AST for the initial parse that only includes
	// package docs and imports.
	initialAST *ast.File

	// Filled in lazily; each one guarded by a sync.Once.
	astCacheOnce sync.Once
	cachedAST    *ast.File
	cachedToken  *token.File

	contentsOnce   sync.Once
	cachedContents []byte

	namesOnce  sync.Once
	namesCache *FileNames

	inspectorOnce  sync.Once
	inspectorCache *inspector.Inspector
}

func (f *File) GoString() string {
	if f == nil {
		return "(*pkginfo.File)(nil)"
	}

	pkgPath := "(UNKNOWN)"
	if f.Pkg != nil {
		pkgPath = f.Pkg.ImportPath.String()
	}
	return fmt.Sprintf("&pkginfo.File{Pkg: %q, Name: %q}", pkgPath, f.Name)
}

// Names returns the computed file-level names.
func (f *File) Names() *FileNames {
	f.namesOnce.Do(func() {
		f.namesCache = resolveFileNames(f)
	})
	return f.namesCache
}

// Contents returns the full file contents.
func (f *File) Contents() []byte {
	f.contentsOnce.Do(func() {
		ioPath := f.FSPath.ToIO()
		data, err := os.ReadFile(ioPath)
		f.l.c.Errs.AssertFile(err, ioPath)
		f.cachedContents = data
	})
	return f.cachedContents
}

// AST returns the parsed AST for this file.
func (f *File) AST() *ast.File {
	f.astCacheOnce.Do(func() {
		astFile, err := goparser.ParseFile(f.l.c.FS, f.FSPath.ToIO(), f.Contents(), goparser.ParseComments)
		f.l.c.Errs.AssertStd(err)
		f.cachedAST = astFile
		f.cachedToken = f.l.c.FS.File(astFile.Pos())
	})
	return f.cachedAST
}

func (f *File) Token() *token.File {
	// Ensure f.cachedToken is set.
	_ = f.AST()

	return f.cachedToken
}

// ASTInspector returns an AST inspector that's optimized for finding
// nodes of particular types. See [inspector.Inspector] for more information.
func (f *File) ASTInspector() *inspector.Inspector {
	f.inspectorOnce.Do(func() {
		tr := f.l.c.Trace("ASTInspector", "pkg", f.Pkg.ImportPath, "file", f.Name)
		defer tr.Done()

		f.inspectorCache = inspector.New([]*ast.File{f.AST()})
	})

	return f.inspectorCache
}

// A QualifiedName is the combination of a package path and a package-level name.
// It can be used to uniquely reference a package-level declaration.
type QualifiedName struct {
	PkgPath paths.Pkg
	Name    string
}

// NaiveDisplayName returns the name as "pkgname.Name" by assuming
// the package name is equal to the last path component.
func (q QualifiedName) NaiveDisplayName() string {
	return path.Base(string(q.PkgPath)) + "." + q.Name
}
