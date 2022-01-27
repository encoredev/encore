//go:build go1.18
// +build go1.18

package names

import (
	"go/ast"
)

type go118Resolver struct{}

func init() {
	registerLanguageLevelResolver(&go118Resolver{})
}

func (g *go118Resolver) LanguageVersion() string {
	return "1.18"
}

func (g *go118Resolver) expr(r *resolver, expr ast.Expr) (ok bool) {
	switch expr := expr.(type) {

	// An IndexListExpr node represents an expression followed by multiple indices.
	// e.g. `X[A, B, C]` or `X[1, 2]`
	case *ast.IndexListExpr:
		r.expr(expr.X)
		for _, index := range expr.Indices {
			r.expr(index)
		}

		return true

	default:
		// default case is we didn't resolve anything
		return false
	}
}
