package pkginfo

import (
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/packages"
)

// RelPath is a slash-separated path relative to the module root.
type RelPath string

func (rp RelPath) toFilePath() string {
	return filepath.FromSlash(string(rp))
}

// Module holds information for a module.
type Module struct {
	l       *Loader // the loader that created it.
	rootDir string  // directory for the module root; absolute or relative

	Path    string // module path
	Version string // module version
	Main    bool   // is this the main module?

	buildCtxOnce   sync.Once
	cachedBuildCtx *build.Context

	// pkgsConfig is the go/packages config to use for
	// resolving packages external to the current module given an import path.
	cachedPkgsConfig *packages.Config
	pkgsConfigOnce   sync.Once
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
	Files      []*File
	Imports    map[string]bool // union of all imports from files

	pkgNamesOnce   sync.Once
	cachedPkgNames *PkgNames
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
