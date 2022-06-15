package walker

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/parser/internal/locations"
)

type Cursor struct {
	*astutil.Cursor
	parents []ast.Node
}

// Parents returns the parent nodes of the current node, with the first element being the immediate parent and the
// the last element being the root node given to the Walk function.
func (c *Cursor) Parents() []ast.Node {
	return c.parents
}

// Location returns the location of the current node.
func (c *Cursor) Location() (loc locations.Location) {
	for idx, parent := range c.parents {
		switch parent := parent.(type) {
		case *ast.File:
			loc |= locations.File
		case *ast.GenDecl:
			if parent.Tok == token.VAR {
				loc |= locations.Variable
			}
		case *ast.FuncDecl:
			if parent.Name.Name == "init" && !hasAncestor[*ast.FuncDecl](c, idx+1) {
				loc |= locations.InitFunction
			}

			loc |= locations.Function
		}
	}

	return
}

// HasAncestor returns true if the current node has an ancestor of the given type.
func hasAncestor[T ast.Node](cursor *Cursor, startIdx int) bool {
	for i := startIdx; i < len(cursor.parents); i++ {
		if _, ok := cursor.parents[i].(T); ok {
			return true
		}
	}

	return false
}
