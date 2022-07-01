package walker

import (
	"go/ast"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
)

type Visitor interface {
	Visit(cursor *Cursor) Visitor
}

// Walk walks the AST starting with root.
//
// This function combines the behaviour of ast.Walk, astutil.Inspect, and astutil.Apply.
//
// - From ast.Walk we have the ability to swap out the visitor for each subtree of nodes
// - From astutil.Inspect we track all the parent nodes walked through to get to the current node
// - From astutil.Apply we have the ability to insert, delete or replace nodes via the Cursor
func Walk(root ast.Node, visitor Visitor) {
	visitorStack := []Visitor{visitor}
	parentNodes := []ast.Node{struct{ ast.Node }{root}}

	cursor := &Cursor{}

	popStack := func(node ast.Node) {
		visitorStack = visitorStack[1:]
		parentNodes = parentNodes[1:]
	}

	astutil.Apply(
		root,
		// function called before recursive into a nodes children
		func(utilCursor *astutil.Cursor) bool {
			cursor.Cursor = utilCursor
			cursor.parents = parentNodes

			nextVisitor := visitorStack[0].Visit(cursor)
			if nextVisitor == nil {
				return false
			}

			// Push the next visitor onto the stack
			visitorStack = append([]Visitor{nextVisitor}, visitorStack...)
			parentNodes = append([]ast.Node{utilCursor.Node()}, parentNodes...)

			return true
		},
		// function called after visiting a nodes children
		func(cursor *astutil.Cursor) bool {
			popStack(cursor.Node())
			return true
		},
	)

	if len(visitorStack) != 1 {
		panic("visitor stack should be len 1 after walking the AST, got length: " + strconv.Itoa(len(visitorStack)))
	}
}
