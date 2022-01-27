package names

import (
	"go/ast"
	"go/token"

	"encr.dev/parser/est"
)

// TrackedPackages defines the set of packages to track,
// defined as a map from import path to package name.
//
// It is used to allow Resolve to efficiently determine
// the name of a package given its import path, as one
// otherwise needs to parse the package source to find it.
type TrackedPackages map[string]string

// Resolution represents the name resolution results.
type Resolution struct {
	Decls map[string]*PkgDecl // package-level declarations, keyed by name
	Files map[*est.File]*File
}

// PkgDecl provides metadata for a package-level declaration.
type PkgDecl struct {
	Name string
	File *est.File
	Pos  token.Pos
	Type token.Token   // CONST, TYPE, VAR, FUNC
	Func *ast.FuncDecl // for Type == FUNC
	Spec ast.Spec      // for other types
	Doc  string
}

// File provides file-level name resolution results.
type File struct {
	PathToName map[string]string    // path -> local name
	NameToPath map[string]string    // local name -> path
	Idents     map[*ast.Ident]*Name // ident -> resolved
	Calls      []*ast.CallExpr
}

// Name provides metadata for a single identifier.
type Name struct {
	Package    bool   // package symbol
	Local      bool   // locally defined symbol
	ImportPath string // non-zero indicates it resolves to the package with the given import path
}
