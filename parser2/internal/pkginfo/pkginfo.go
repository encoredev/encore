package pkginfo

import (
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"io/fs"
	"sync"
)

// Module holds information for a module.
type Module struct {
	l    *Loader // the loader that created it.
	fsys fs.FS   // file system containing the files for this module

	Path    string // module path
	Version string // module version
	Main    bool   // is this the main module?

	buildCtxOnce   sync.Once
	cachedBuildCtx *build.Context
}

// ParseDir parses the package at relPath, relative to the module root.
// It returns (nil, false) if the directory contains no Go files.
func (m *Module) ParseDir(relPath string) (pkg *Package, exists bool) {
	return m.l.parseDir(m, relPath)
}

func (m *Module) String() string {
	return fmt.Sprintf("module %s@%s", m.Path, m.Version)
}

type Package struct {
	l          *Loader // the loader that created it.
	Module     *Module
	AST        *ast.Package
	Name       string
	Doc        string
	ImportPath string // import path
	RelPath    string // relative path to the module root
	Files      []*File
	Imports    map[string]bool // union of all imports from files
}

type File struct {
	l        *Loader  // the loader that created it.
	Name     string   // file name ("foo.go")
	Pkg      *Package // package it belongs to
	Path     string   // filesystem path
	Contents []byte
	Imports  map[string]bool // imports in the file, keyed by import path
	TestFile bool            // whether the file is a test file

	// initialAST is the AST for the initial parse that only includes
	// package docs and imports.
	initialAST *ast.File

	// Filled in lazily
	astCacheOnce sync.Once
	cachedAST    *ast.File
}

// AST returns the parsed AST for this file.
func (f *File) AST() *ast.File {
	f.astCacheOnce.Do(func() {
		astFile, err := goparser.ParseFile(f.l.c.FS, f.Path, f.Contents, goparser.ParseComments)
		f.l.c.Errs.AssertStd(err)
		f.cachedAST = astFile
	})
	return f.cachedAST
}

// key reports a unique key for this module.
func (m *Module) key() string {
	version := m.Version
	if m.Main && version == "" {
		version = "__main__"
	}
	return m.Path + "@" + version
}
