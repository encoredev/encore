package cuegen

import (
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
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

	for _, line := range lines {
		grp.List = append(grp.List, &ast.Comment{Text: "// " + line})
	}

	// If this comment is multiline, then the first line should be positioned on a NewSection for force an empty line
	// before the comment group
	if commentPosition == 0 {
		grp.List[0].Slash = token.NewSection.Pos()
	}

	field.AddComment(grp)
}

// commentAlreadyPresent returns true if the field already contains all of the comment of `contains`
// within one of it's existing comment groups
func commentAlreadyPresent(field *ast.Field, contains *ast.CommentGroup) bool {
	if len(contains.List) == 0 {
		return true
	}

commentGroupLoop:
	for _, c := range field.Comments() {
		// Range over this group comment
	groupLineLoop:
		for i, line := range c.List {
			if i+len(contains.List) > len(c.List) {
				// If we've not got enough lines to contain the entire comment, then
				// we can try the next comment group
				continue commentGroupLoop
			}

			// If we find the first line then great, let's check it
			if strings.TrimSpace(line.Text) == strings.TrimSpace(contains.List[0].Text) {
				for j, containsLine := range contains.List {
					if strings.TrimSpace(c.List[i+j].Text) != strings.TrimSpace(containsLine.Text) {
						continue groupLineLoop
					}
				}

				// If here the entire comment group is contained within field already
				return true
			}
		}
	}
	return false
}

func hasCommentInPosition(field *ast.Field, pos int8) bool {
	for _, c := range field.Comments() {
		if len(c.List) > 1 {
			return true
		}
		if c.Position == pos {
			return true
		}
	}
	return false
}
