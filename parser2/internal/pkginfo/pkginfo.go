package pkginfo

import (
	"go/ast"
	"go/build"
	"sync"

	"golang.org/x/tools/go/packages"

	"encr.dev/parser2/internal/pkginfo/pkgload"
)

// Module holds information for a module.
type legacyModule struct {
	l       *pkgload.Loader // the loader that created it.
	rootDir string          // directory for the module root; absolute or relative

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

type Package struct {
	l          *pkgload.Loader // the loader that created it.
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
	l        *pkgload.Loader // the loader that created it.
	Name     string          // file name ("foo.go")
	Pkg      *Package        // package it belongs to
	Path     string          // filesystem path
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
