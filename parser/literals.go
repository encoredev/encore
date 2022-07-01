package parser

import (
	"go/ast"
	"go/token"
)

// litString will return the string value of a given node
//
// If the given node isn't a string literal, it will return an empty string and false
func litString(node ast.Node) (string, bool) {
	if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value[1 : len(lit.Value)-1], true
	}
	return "", false
}
