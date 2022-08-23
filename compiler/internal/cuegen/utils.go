package cuegen

import (
	"strings"

	"cuelang.org/go/cue/ast"
)

// addCommentToField adds the given string to a Field
//
// If the str is a single line, then the comment group will be positioned at
// the end of the node it is attached to.
//
// If the str is a multi-line string, then the comment group will be positioned
// at the before the node it is attached to.
func addCommentToField(field *ast.Field, str string) {
	lines := strings.Split(strings.TrimSpace(str), "\n")

	// Position 4 = after the attached node, position 0 = before the attached node
	commentPosition := int8(4)
	_, isStruct := field.Value.(*ast.StructLit)
	if len(lines) > 1 || isStruct {
		commentPosition = 0
	}
	grp := &ast.CommentGroup{
		Position: commentPosition,
	}

	// If this is multiline, then the first line should be a blank line with
	// no comment, to give it spacing from the field above it
	// if len(lines) > 1 {
	// 	grp.List = append(grp.List, &ast.Comment{Text: "\n"})
	// }

	for _, line := range lines {
		grp.List = append(grp.List, &ast.Comment{Text: "// " + line})
	}

	field.AddComment(grp)
}
