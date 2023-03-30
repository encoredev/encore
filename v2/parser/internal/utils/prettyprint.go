package utils

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"
)

func PrettyPrint(node ast.Expr) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name

	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", PrettyPrint(node.X), node.Sel.Name)

	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", PrettyPrint(node.X), PrettyPrint(node.Index))

	case *ast.IndexListExpr:
		indices := make([]string, 0, len(node.Indices))
		for _, n := range node.Indices {
			indices = append(indices, PrettyPrint(n))
		}
		return fmt.Sprintf("%s[%s]", PrettyPrint(node.X), strings.Join(indices, ", "))

	case *ast.FuncLit:
		return "a function literal"

	case *ast.StarExpr:
		return fmt.Sprintf("a pointer to %s", PrettyPrint(node.X))

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}
