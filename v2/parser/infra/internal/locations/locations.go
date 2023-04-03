package locations

import (
	"go/ast"
)

type Classification interface {
	classification()
}

type PkgVar struct {
	Ident *ast.Ident
	Name  string
}

type InFunc struct {
	Node ast.Node // *ast.FuncDecl or *ast.FuncLit
}

type OtherPkgExpr struct{}

func (PkgVar) classification()       {}
func (InFunc) classification()       {}
func (OtherPkgExpr) classification() {}

// Classify classifies the current location based on the ancestor stack.
// The last node in the stack is the expression to classify.
func Classify(stack []ast.Node) (c Classification) {
	num := len(stack)

	// target is the target expression
	target := stack[num-1]

	// First determine if we're inside any function declaration,
	// as that takes precedence over any other classification.
	for i := num - 2; i >= 0; i-- {
		switch node := stack[i].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return InFunc{Node: node}
		}
	}

	// Iterate over the node stack from inside to out.
	for i := num - 2; i >= 0; i-- {
		switch node := stack[i].(type) {

		case *ast.ValueSpec:
			for n := 0; n < len(node.Names); n++ {
				if len(node.Values) > n && node.Values[n] == target {
					// We've found the value that contains the resource.
					return PkgVar{
						Ident: node.Names[n],
						Name:  node.Names[n].Name,
					}
				}
			}
		}
	}

	return OtherPkgExpr{}
}
