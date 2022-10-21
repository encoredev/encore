package internal

import (
	"go/ast"
	"go/token"
	"os"

	"github.com/rs/zerolog/log"
)

func FromGoASTNodeWithTypeAndText(fileset *token.FileSet, node ast.Node, typ LocationType, text string) *SrcLocation {
	loc := FromGoASTNode(fileset, node)
	loc.Type = typ
	loc.Text = text
	return loc
}

// FromGoASTNode returns a SrcLocation from a Go AST node storing the start and end
// locations of that node.
func FromGoASTNode(fileset *token.FileSet, node ast.Node) *SrcLocation {
	start := fileset.Position(node.Pos())
	end := fileset.Position(node.End())

	// Custom end locations for some node types
	switch node := node.(type) {
	case *ast.CallExpr:
		end = fileset.Position(node.Rparen + 1)
	}

	return FromGoTokenPositions(start, end)
}

// FromGoTokenPositions returns a SrcLocation from two Go token positions.
// They can be the same position or different positions. However, they must
// be locations within the same file.
//
// This function will panic if the locations are in different files.
func FromGoTokenPositions(start token.Position, end token.Position) *SrcLocation {
	if start.Filename != end.Filename {
		panic("FromGoASTNode: start and end files must be the same")
	}

	bytes, err := os.ReadFile(start.Filename)
	if err != nil {
		log.Err(err).Str("filename", start.Filename).Msg("Failed to read Go file")
		// Don't return, `bytes == nil` is fine here
	}

	return &SrcLocation{
		File: &File{
			RelPath:  start.Filename,
			FullPath: start.Filename,
			Contents: bytes,
		},
		Start: Pos{Line: start.Line, Col: start.Column},
		End:   Pos{Line: end.Line, Col: end.Column},
		Type:  LocError,
	}
}
