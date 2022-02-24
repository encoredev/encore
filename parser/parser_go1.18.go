//go:build go1.18
// +build go1.18

package parser

import (
	"go/ast"
	"reflect"

	"encr.dev/parser/est"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func init() {
	additionalTypeResolver = go118ResolveType
}

func go118ResolveType(p *parser, pkg *est.Package, file *est.File, expr ast.Expr) *schema.Type {
	switch expr := expr.(type) {
	// Needed for generic types with single generic parameters: `X[Index]` (i.e. `Vector[string]`)
	case *ast.IndexExpr:
		return resolveWithTypeArguments(p, pkg, file, expr.X, expr.Index)

	// Needed for generic types with multiple generic parameters: `X[A, B]` (i.e. `Skiplist[string, string]`)
	case *ast.IndexListExpr:
		return resolveWithTypeArguments(p, pkg, file, expr.X, expr.Indices...)
	}

	return nil
}

// resolveWithTypeArguments first resolves the parameterized declaration of `ident`, before resolving each of
// the `typeArguments` to concrete types. It then returns a `*schema.Name` representing that instantiated type.
func resolveWithTypeArguments(p *parser, pkg *est.Package, file *est.File, ident ast.Expr, typeArguments ...ast.Expr) *schema.Type {
	parameterizedType := p.resolveType(pkg, file, ident, nil)

	named := parameterizedType.GetNamed()
	if named == nil {
		p.errf(ident.Pos(), "expected to get a named type, got %+v", reflect.TypeOf(parameterizedType.Typ))
		return parameterizedType
	}

	decl := p.decls[named.Id]
	if decl == nil {
		p.errf(ident.Pos(), "unable to find decl referenced")
		p.abort()
	}

	if len(decl.TypeParams) != len(typeArguments) {
		p.errf(ident.Pos(), "expected %d type parameters, got %d for reference to %s", len(decl.TypeParams), len(typeArguments), decl.Name)
		p.abort()
	}

	named.TypeArguments = make([]*schema.Type, len(decl.TypeParams))
	for idx, expr := range typeArguments {
		named.TypeArguments[idx] = p.resolveType(pkg, file, expr, nil)
	}

	return parameterizedType
}
