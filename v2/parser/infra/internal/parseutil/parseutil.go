package parseutil

import (
	"go/ast"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
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
func hasRequiredImports(imports map[paths.Pkg]bool, required []paths.Pkg) bool {
	for _, pkg := range required {
		if !imports[pkg] {
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

// resolvedAssignedVar resolves the identifier the resource is being assigned to,
// as well as the doc comment associated with it.
// If no identifier can be found it reports None.
func resolveAssignedVar(stack []ast.Node) option.Option[*ast.Ident] {
	for i := len(stack) - 1; i >= 0; i-- {
		vs, ok := stack[i].(*ast.ValueSpec)
		if !ok {
			continue
		}

		// We have found the value spec. Now determine which of the values
		// that contains the resource we're processing.
		if (i + 1) >= len(stack) {
			// We're at the top of the stack already, so there's no
			// resource here. That should never happen.
			panic("internal error: resource not found in stack")
		}

		for n := 0; n < len(vs.Names); n++ {
			if len(vs.Values) > n && vs.Values[n] == stack[i+1] {
				// We've found the value that contains the resource.
				return option.AsOptional(vs.Names[n])
			}
		}
		break
	}
	return option.None[*ast.Ident]()
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
