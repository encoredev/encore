package internal

import (
	"go/ast"
	"go/parser"
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

	if !start.IsValid() || !end.IsValid() {
		return nil
	}

	return FromGoTokenPositions(start, end)
}

func FromGoTokenPos(fileset *token.FileSet, start, end token.Pos) *SrcLocation {
	startPos := fileset.Position(start)
	endPos := fileset.Position(end)

	if !startPos.IsValid() || !endPos.IsValid() {
		return nil
	}

	return FromGoTokenPositions(startPos, endPos)
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

	// Attempt to convert a single start/end position into a range
	if start == end {
		end = convertSingleGoPositionToRange(start.Filename, bytes, start)
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

// convertSingleGoPositionToRange attempts to convert a single Go token position to a range with a start and end
// position.
//
// This is done by attempting to parse the file, and then if we are able to parse it successfully, we look for an AST
// node which starts at the exact line and column of the position. If we find one, we use the end position of that
// node as the end position of the range.
//
// We use the first found node at that position, as we assume the largest node at that position is the most relevant
func convertSingleGoPositionToRange(filename string, fileBody []byte, start token.Position) (end token.Position) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, filename, fileBody, parser.ParseComments)

	if err != nil || file == nil {
		return start
	}

	var match ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		nodePos := fs.Position(n.Pos())
		if nodePos.Line == start.Line && nodePos.Column == start.Column {
			match = n
			return false
		}

		return true
	})

	if match != nil {
		return fs.Position(match.End())
	}
	return start
}
