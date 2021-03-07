package compiler

import (
	"go/ast"
	"go/token"
	"strconv"
)

func usesImport(f *ast.File, pkgName, path string, exceptions map[*ast.SelectorExpr]bool) bool {
	// Find if the import has been given a different name
	for _, s := range f.Imports {
		if p, _ := strconv.Unquote(s.Path.Value); p == path {
			if s.Name != nil {
				pkgName = s.Name.Name
			}
			break
		}
	}

	var used bool
	ast.Walk(visitFn(func(n ast.Node) {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && isTopName(sel.X, pkgName) && !exceptions[sel] {
			used = true
		}
	}), f)
	return used
}

func findImport(f *ast.File, path string) (*ast.ImportSpec, *ast.GenDecl, bool) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			impspec := spec.(*ast.ImportSpec)
			if p, _ := strconv.Unquote(impspec.Path.Value); p == path {
				return impspec, gen, true
			}
		}
	}
	return nil, nil, false
}

type visitFn func(node ast.Node)

func (fn visitFn) Visit(node ast.Node) ast.Visitor {
	fn(node)
	return fn
}

// isTopName returns true if n is a top-level unresolved identifier with the given name.
func isTopName(n ast.Expr, name string) bool {
	id, ok := n.(*ast.Ident)
	return ok && id.Name == name && id.Obj == nil
}
