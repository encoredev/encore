package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

// errInSrc reports an error in the source code.
//
// Note: err must be either a errinsrc.ErrorList or an *errinsrc.Error.
func (p *parser) errInSrc(err error) {
	p.errors.Report(err)
}

// Deprecated: use errors.errInSrc instead
func (p *parser) err(pos token.Pos, msg string) {
	p.errors.Add(pos, msg)
}

// Deprecated: use errors.errInSrc instead
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
