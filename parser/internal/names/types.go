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

// Application presents all resolved names throughout the Application
type Application map[*est.Package]*Resolution

// PkgObjRef resolves the given ast.Node to a package path, object name and index arguments. In the case it fails to
// resolve, it will return ("", "", nil).
//
// If the given node is a index or index list, then the index arguments will be extracted. These could either be type
// arguments, or map access arguments.
//
// If the given node is:
// - an ident, then it is resolved to a package level deceleration in the same file as the ident.
// - a selector, then it is resolved to an exported ident from the selected package.
// - any other node will result in nothing being returned
//
// Note:
// - This function accounts for scoping and variable shadowing. (i.e. it will not mistake a variable "foo" in a function
//   for the package imported as "foo").
// - This function does not validate the existence of the objName for objects referenced in other packages. As such it
//   can return references for objects in Go Modules.
func (a Application) PkgObjRef(inFile *est.File, node ast.Node) (pkgPath string, objName string, indexArguments []ast.Expr) {
	f, ok := a[inFile.Pkg].Files[inFile]
	if !ok {
		return "", "", nil
	}

	// Resolve the type arguments
	var typArgs []ast.Expr
	switch typNode := node.(type) {
	case *ast.IndexExpr:
		node = typNode.X
		typArgs = []ast.Expr{typNode.Index}
	case *ast.IndexListExpr:
		node = typNode.X
		typArgs = typNode.Indices
	}

	// Resolve the identifier
	switch node := node.(type) {
	case *ast.Ident:
		// If it's an ident, then we're looking for something which resolves to a package-level object defined
		// in the same package as the ident is located in.
		if name := f.Idents[node]; name != nil && name.Package {
			return inFile.Pkg.ImportPath, node.Name, typArgs
		}
	case *ast.SelectorExpr:
		// If it's a selector, then we're looking for something which has been imported from another package
		if pkgName, ok := node.X.(*ast.Ident); ok {
			if resolvedIdent := f.Idents[pkgName]; resolvedIdent != nil && resolvedIdent.ImportPath != "" {
				return resolvedIdent.ImportPath, node.Sel.Name, typArgs
			}
		}
	}

	return "", "", nil
}

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
