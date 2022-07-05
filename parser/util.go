package parser

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"encr.dev/parser/est"
)

// findFuncFor will either return a *ast.FuncDecl, *ast.FuncLit or nil in case of an error.
// The second return parameter is the File in which that function is declared.
//
// If the node is:
// - a *ast.FuncLit, it will simply return the FuncLit
// - a *ast.Ident, it will look inside the same package for the FuncDecl that matches the ident
// - a *ast.SelectorExpr, it will look inside the specified package for the FuncDecl
//
// If an error occurs, the error will be reported and nil returned.
func (p *parser) findFuncFor(node ast.Node, from *est.File, errPrefix string) (ast.Node, *est.File) {
	pkg := from.Pkg
	name := ""

	// Check the type of node passed in
	switch node := node.(type) {
	case *ast.FuncLit:
		return node, from
	case *ast.Ident:
		name = node.Name

		if name == "nil" {
			p.errf(node.Pos(), "%s is required, was given as nil.", errPrefix)
			return nil, nil
		}
	case *ast.SelectorExpr:
		pkgPath, objName := pkgObj(p.names[pkg].Files[from], node)
		if pkgPath == "" {
			p.errf(node.Pos(), "%s referenced an unknown package `%s`.", errPrefix, node.X)
			return nil, nil
		}

		pkg = p.pkgMap[pkgPath]
		if pkg == nil {
			p.errf(node.Pos(), "%s references a package outside the Encore application: `%s`.", errPrefix, pkgPath)
			return nil, nil
		}
		name = objName
	default:
		p.errf(node.Pos(), "%s is required to a function reference or a function literal, got a %v.", errPrefix, reflect.TypeOf(node))
		return nil, nil
	}

	// Now we know the package and the name of the function, let's look it up
	decl, found := p.names[pkg].Decls[name]
	if !found {
		p.errf(node.Pos(), "%s with the identifier `%s`, however it was not declared in %s", errPrefix, name, pkg.Name)
		return nil, nil
	}

	if decl.Type != token.FUNC {
		p.errf(node.Pos(), "%s with the identifier `%s` was not declared as a function but as a %s", errPrefix, name, decl.Type)
		return nil, nil
	}

	return decl.Func, decl.File
}

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
