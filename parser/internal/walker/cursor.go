package walker

import (
	"go/ast"
	"go/token"
	"strings"

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
			if parent.Name.Name == "init" {
				if f, _ := getAncestor[*ast.FuncDecl](c, idx+1); f == nil {
					loc |= locations.InitFunction
				}
			}

			loc |= locations.Function
		}
	}

	return
}

// DocComment returns the doc comment associated with the given node.
// It walks through the parent nodes until it finds a node with a Comment field or fields,
// and stops when it comes across a node which represents a block where a previous
// comment would no longer be valid (such as a FuncType, StructType, InterfaceType or BlockStmt).
func (c *Cursor) DocComment() string {
	groupToString := func(node *ast.CommentGroup) string {
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
	}

	for _, node := range c.parents {
		switch node := node.(type) {
		case *ast.Field:
			// Check the Field declaration both it's Doc comment and then it's inline comment
			if cmt := groupToString(node.Doc); cmt != "" {
				return cmt
			}

			if cmt := groupToString(node.Comment); cmt != "" {
				return cmt
			}
		case *ast.ValueSpec:
			// Check the value declaration both it's Doc comment and then it's inline comment
			if cmt := groupToString(node.Doc); cmt != "" {
				return cmt
			}

			if cmt := groupToString(node.Comment); cmt != "" {
				return cmt
			}

		case *ast.GenDecl:
			// Check the declarations comment
			if cmt := groupToString(node.Doc); cmt != "" {
				return cmt
			}

		case *ast.Comment:
			if node == nil {
				return ""
			}

			return node.Text

		case *ast.CommentGroup:
			return groupToString(node)

		case *ast.BlockStmt, *ast.StructType, *ast.InterfaceType, *ast.FuncType:
			return ""
		}
	}

	return ""
}

// GetAncestor returns the closest ancestor of the given type
func GetAncestor[T ast.Node](cursor *Cursor) T {
	rtn, _ := getAncestor[T](cursor, 0)
	return rtn
}

// GetFurthestAncestor returns the furthest ancestor of the given type
func GetFurthestAncestor[T ast.Node](cursor *Cursor) T {
	var rtn T
	idx := -1
	for {
		found, foundIdx := getAncestor[T](cursor, idx+1)
		if foundIdx <= -1 {
			// If not found, return the last found value
			return rtn
		}

		rtn, idx = found, foundIdx
	}
}

// getAncestor checks if the current node has an ancestor of the given type and returns
// that ancestor and the index into the parents slice where it was found.
func getAncestor[T ast.Node](cursor *Cursor, startIdx int) (T, int) {
	for i := startIdx; i < len(cursor.parents); i++ {
		if val, ok := cursor.parents[i].(T); ok {
			return val, i
		}
	}

	var defaultValue T
	return defaultValue, -1
}
