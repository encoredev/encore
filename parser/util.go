package parser

import (
	"go/ast"
	"strings"
)

// walkerFunc allows us to pass a function to ast.Walk which will recursively descend the AST until the function returns false
type walkerFunc func(node ast.Node) bool

func (w walkerFunc) Visit(node ast.Node) ast.Visitor {
	if w(node) {
		return w
	}

	return nil
}

// docNodeToString converts a Comment or CommentGroup into a parsed string
//
// If the given node isn't a comment or comment group, a blank string will be returned
func docNodeToString(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Comment:
		if node == nil {
			return ""
		}

		return node.Text

	case *ast.CommentGroup:
		if node == nil {
			return ""
		}

		var str strings.Builder
		for i, comment := range node.List {
			if i > 0 {
				str.WriteString("\n")
			}

			str.WriteString(comment.Text)
		}
		return str.String()

	default:
		return ""
	}
}

func getTypeArguments(node ast.Node) []ast.Expr {
	switch node := node.(type) {
	case *ast.IndexExpr:
		return []ast.Expr{node.Index}
	case *ast.IndexListExpr:
		return node.Indices
	default:
		return nil
	}
}
