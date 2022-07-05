package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

func (p *parser) err(pos token.Pos, msg string) {
	p.errors.Add(pos, msg)
}

func (p *parser) errf(pos token.Pos, format string, args ...interface{}) {
	p.errors.Addf(pos, format, args...)
}

func (p *parser) abort() {
	p.errors.Abort()
}

func prettyPrint(node ast.Expr) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name

	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", prettyPrint(node.X), node.Sel.Name)

	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", prettyPrint(node.X), prettyPrint(node.Index))

	case *ast.IndexListExpr:
		indices := make([]string, 0, len(node.Indices))
		for _, n := range node.Indices {
			indices = append(indices, prettyPrint(n))
		}
		return fmt.Sprintf("%s[%s]", prettyPrint(node.X), strings.Join(indices, ", "))

	case *ast.FuncLit:
		return "a function literal"

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}
