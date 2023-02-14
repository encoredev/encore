package pkgload

import (
	"go/ast"
	goparser "go/parser"
	"os"
	"sync"

	"golang.org/x/mod/modfile"

	"encr.dev/parser2/internal/paths"
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
	Files      []*File
	Imports    map[string]bool // union of all imports from files

	namesOnce  sync.Once
	namesCache *PkgNames
}

// Names returns the computed package-level names.
func (p *Package) Names() *PkgNames {
	p.namesOnce.Do(func() {
		p.namesCache = resolvePkgNames(p)
	})
	return p.namesCache
}

type File struct {
	l        *Loader         // the loader that created it.
	Name     string          // file name ("foo.go")
	Pkg      *Package        // package it belongs to
	FSPath   paths.FS        // where the file lives on disk
	Imports  map[string]bool // imports in the file, keyed by import path
	TestFile bool            // whether the file is a test file

	// initialAST is the AST for the initial parse that only includes
	// package docs and imports.
	initialAST *ast.File

	// Filled in lazily; each one guarded by a sync.Once.
	astCacheOnce   sync.Once
	cachedAST      *ast.File
	contentsOnce   sync.Once
	cachedContents []byte
	namesOnce      sync.Once
	namesCache     *FileNames
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
	})
	return f.cachedAST
}
