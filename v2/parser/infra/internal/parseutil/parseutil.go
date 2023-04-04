package parseutil

import (
	"go/ast"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
)

// FindPkgNameRefs finds all references in the given package that references
// any of the given pkgNames. For each such reference it calls fn.
func FindPkgNameRefs(pkg *pkginfo.Package, pkgNames []pkginfo.QualifiedName, fn func(f *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node)) {
	// Scan files that contain the required imports.
	requiredImports := computeRequiredImports(pkgNames)
	// If the union of all imports is not a subset of the package's imports,
	// might as well not even look at individual files.
	if !hasRequiredImports(pkg.Imports, requiredImports) {
		return
	}

	// Turn cfg.Funcs into a lookup table.
	wantNames := make(map[pkginfo.QualifiedName]bool, len(pkgNames))
	for _, name := range pkgNames {
		wantNames[name] = true
	}

	for _, file := range pkg.Files {
		if !hasRequiredImports(file.Imports, requiredImports) {
			continue
		}

		// We have a file that contains the required imports.
		// Scan it for resource creation calls.
		inspector := file.ASTInspector()
		fileNames := file.Names()

		// nodeFilter are the AST types we care about inspecting.
		nodeFilter := []ast.Node{(*ast.SelectorExpr)(nil)}

		// Walk the AST to find references. Use stack information to resolve whether a particular
		// reference is in a valid location.
		inspector.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) (proceed bool) {
			// If we're popping the stack, there's nothing to do.
			if !push {
				return true
			}

			// We have a reference to a selector. See if that selector
			// references the constructors we care about.
			sel := n.(*ast.SelectorExpr) // guaranteed based on our node filter.
			name, ok := fileNames.ResolvePkgLevelRef(sel)
			if !ok {
				// Not a package-level reference.
				return true
			} else if !wantNames[name] {
				// Not a reference we care about. Keep recursing.
				return true
			}

			fn(file, name, stack)
			return true
		})
	}
}

// computeRequiredImports computes the required imports based on
// a list of func names to look for.
func computeRequiredImports(funcs []pkginfo.QualifiedName) []paths.Pkg {
	var result []paths.Pkg
	seen := make(map[paths.Pkg]bool)
	for _, f := range funcs {
		if !seen[f.PkgPath] {
			seen[f.PkgPath] = true
			result = append(result, f.PkgPath)
		}
	}
	return result
}

// hasRequiredImports reports whether the given imports set
// contains all the required imports.
func hasRequiredImports(imports map[paths.Pkg]ast.Node, required []paths.Pkg) bool {
	for _, pkg := range required {
		if _, found := imports[pkg]; !found {
			return false
		}
	}
	return true
}

// resolveTypeArgs resolves the type argument expressions for the given node.
func resolveTypeArgs(node ast.Node) []ast.Expr {
	switch n := node.(type) {
	case *ast.IndexExpr:
		return []ast.Expr{n.Index}
	case *ast.IndexListExpr:
		return n.Indices
	}
	return nil
}

func resolveResourceDoc(stack []ast.Node) (doc string) {
	getDoc := func(candidates ...*ast.CommentGroup) string {
		for _, cg := range candidates {
			if t := cg.Text(); t != "" {
				return t
			}
		}
		return ""
	}

	for i := len(stack) - 1; i >= 0; i-- {
		switch node := stack[i].(type) {
		case *ast.Field:
			if cmt := getDoc(node.Doc, node.Comment); cmt != "" {
				return cmt
			}
		case *ast.ValueSpec:
			if cmt := getDoc(node.Doc, node.Comment); cmt != "" {
				return cmt
			}

		case *ast.GenDecl:
			if cmt := getDoc(node.Doc); cmt != "" {
				return cmt
			}

		case *ast.Comment:
			if node == nil {
				return ""
			}
			return node.Text

		case *ast.CommentGroup:
			return getDoc(node)

		case *ast.BlockStmt, *ast.StructType, *ast.InterfaceType, *ast.FuncType:
			return ""
		}
	}

	return ""
}
