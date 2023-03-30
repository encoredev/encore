package parseutil

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

// Converts a node to a string which looks like the original go code.
// such as a ast.SelectorExpr will become "foo.Blah"
//
// It's not intended to be an exact representation, but rather a helperful
// representation for error messages.
func nodeAsGoSrc(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name

	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", nodeAsGoSrc(node.X), node.Sel.Name)

	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", nodeAsGoSrc(node.X), nodeAsGoSrc(node.Index))

	case *ast.IndexListExpr:
		indices := make([]string, 0, len(node.Indices))
		for _, n := range node.Indices {
			indices = append(indices, nodeAsGoSrc(n))
		}
		return fmt.Sprintf("%s[%s]", nodeAsGoSrc(node.X), strings.Join(indices, ", "))

	case *ast.FuncLit:
		return "a function literal"

	case *ast.BasicLit:
		return node.Value

	case *ast.CallExpr:
		return fmt.Sprintf("%s(...)", nodeAsGoSrc(node.Fun))

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}

// NodeType converts a node to a string that can be used in an error message.
// such as a ast.CallExpr will return "a function call to foo.Blah"
func NodeType(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Ident:
		return "an identifier"

	case *ast.SelectorExpr:
		return "an identifier"

	case *ast.IndexExpr:
		return "a identifier"

	case *ast.IndexListExpr:
		return "a identifier"

	case *ast.FuncLit:
		return "a function literal"

	case *ast.BasicLit:
		switch node.Kind {
		case token.INT:
			return "an integer literal"
		case token.FLOAT:
			return "a float literal"
		case token.IMAG:
			return "an imaginary literal"
		case token.CHAR:
			return "a character literal"
		case token.STRING:
			return "a string literal"
		default:
			return "a literal"
		}

	case *ast.CallExpr:
		return fmt.Sprintf("a function call to %s", nodeAsGoSrc(node.Fun))

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}
