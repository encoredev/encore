//go:build go1.16
// +build go1.16

package names

import (
	"go/ast"
)

type go116Resolver struct{}

func init() {
	registerLanguageLevelResolver(&go116Resolver{})
}

func (g *go116Resolver) LanguageVersion() string {
	return "1.16"
}

func (g *go116Resolver) expr(r *resolver, expr ast.Expr) (ok bool) {
	switch expr := expr.(type) {
	case *ast.Ident:
		r.ident(expr)

	case *ast.Ellipsis:
		r.expr(expr.Elt)

	case *ast.FuncLit:
		r.openScope()
		defer r.closeScope()

		// First resolve types before introducing names
		for _, param := range expr.Type.Params.List {
			r.expr(param.Type)
		}
		if expr.Type.Results != nil {
			for _, result := range expr.Type.Results.List {
				r.expr(result.Type)
			}
		}

		for _, field := range expr.Type.Params.List {
			for _, name := range field.Names {
				r.define(name, &Name{Local: true})
			}
		}
		if expr.Type.Results != nil {
			for _, field := range expr.Type.Results.List {
				for _, name := range field.Names {
					r.define(name, &Name{Local: true})
				}
			}
		}
		if expr.Body != nil {
			r.stmt(expr.Body)
		}

	case *ast.CompositeLit:
		r.expr(expr.Type)
		r.exprList(expr.Elts)

	case *ast.ParenExpr:
		r.expr(expr.X)

	case *ast.SelectorExpr:
		r.expr(expr.X)
		// Note: we don't treat 'Foo' in 'x.Foo' as an identifier,
		// as it does not introduce a new name to any scope.

	case *ast.IndexExpr:
		r.expr(expr.X)
		r.expr(expr.Index)

	case *ast.SliceExpr:
		r.expr(expr.X)
		r.expr(expr.Low)
		r.expr(expr.High)
		r.expr(expr.Max)

	case *ast.TypeAssertExpr:
		r.expr(expr.X)
		r.expr(expr.Type)

	case *ast.CallExpr:
		r.Calls = append(r.Calls, expr)
		r.expr(expr.Fun)
		r.exprList(expr.Args)

	case *ast.StarExpr:
		r.expr(expr.X)

	case *ast.UnaryExpr:
		r.expr(expr.X)

	case *ast.BinaryExpr:
		r.expr(expr.X)
		r.expr(expr.Y)

	case *ast.KeyValueExpr:
		// HACK: We want to track uses of functions. This is tricky because
		// struct types use keys that are idents that refer to the struct field,
		// while map types can use keys to refer to idents in scope.
		//
		// Unfortunately We cannot easily know the type of the composite literal
		// without typechecking. However, funcs are incomparable and therefore
		// are not valid as map keys. So let's simply avoid tracking idents
		// in the keys, and rely on the compiler to eventually catch this for us.
		if _, ok := expr.Key.(*ast.Ident); !ok {
			r.expr(expr.Key)
		}
		r.expr(expr.Value)

	case *ast.ArrayType:
		r.expr(expr.Len)
		r.expr(expr.Elt)

	case *ast.StructType:
		for _, field := range expr.Fields.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}

	case *ast.FuncType:
		for _, field := range expr.Params.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}
		if expr.Results != nil {
			for _, field := range expr.Results.List {
				r.expr(field.Type)
				// Don't look at names; they don't resolve to outside scope
			}
		}

	case *ast.InterfaceType:
		for _, field := range expr.Methods.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}

	case *ast.MapType:
		r.expr(expr.Key)
		r.expr(expr.Value)

	case *ast.ChanType:
		r.expr(expr.Value)

	case *ast.BadExpr, *ast.BasicLit:
		// do nothing

	default:
		// If we don't process this then return false
		return false
	}

	// Otherwise we processed it, so all ok
	return true
}
