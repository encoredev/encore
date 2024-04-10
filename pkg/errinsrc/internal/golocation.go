package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/rs/zerolog/log"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
)

func FromGoASTNodeWithTypeAndText(fileset *token.FileSet, node ast.Node, typ LocationType, text string, fileReaders ...paths.FileReader) option.Option[*SrcLocation] {
	loc := FromGoASTNode(fileset, node, fileReaders...)
	if l, ok := loc.Get(); ok {
		l.Type = typ
		l.Text = text
	}
	return loc
}

// FromGoASTNode returns a SrcLocation from a Go AST node storing the start and end
// locations of that node.
func FromGoASTNode(fileset *token.FileSet, node ast.Node, fileReaders ...paths.FileReader) option.Option[*SrcLocation] {
	start := fileset.Position(node.Pos())
	end := fileset.Position(node.End())

	// Custom end locations for some node types
	switch node := node.(type) {
	case *ast.CallExpr:
		end = fileset.Position(node.Rparen + 1)
	}

	if !start.IsValid() || !end.IsValid() {
		return option.None[*SrcLocation]()
	}

	return FromGoTokenPositions(start, end, fileReaders...)
}

func FromGoTokenPos(fileset *token.FileSet, start, end token.Pos, fileReaders ...paths.FileReader) option.Option[*SrcLocation] {
	startPos := fileset.Position(start)
	endPos := fileset.Position(end)

	if !startPos.IsValid() || !endPos.IsValid() {
		return option.None[*SrcLocation]()
	}

	return FromGoTokenPositions(startPos, endPos, fileReaders...)
}

// FromGoTokenPositions returns a SrcLocation from two Go token positions.
// They can be the same position or different positions. However, they must
// be locations within the same file.
//
// This function will panic if the locations are in different files.
func FromGoTokenPositions(start token.Position, end token.Position, fileReaders ...paths.FileReader) option.Option[*SrcLocation] {
	if start.Filename != end.Filename {
		panic("FromGoASTNode: start and end files must be the same")
	}
	fileReaders = append(fileReaders, os.ReadFile)
	var bytes []byte
	var err error
	for _, reader := range fileReaders {
		if reader == nil {
			continue
		}
		bytes, err = reader(start.Filename)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Err(err).Str("filename", start.Filename).Msg("Failed to read Go file")
		// Don't return, `bytes == nil` is fine here
	}

	// Attempt to convert a single start/end position into a range
	if start == end {
		end = convertSingleGoPositionToRange(start.Filename, bytes, start)
	}

	// If either position is invalid, return nil
	// as that means we're not dealing with a Go Token Position
	if !start.IsValid() || !end.IsValid() {
		log.Warn().Str("start", start.String()).Str("end", end.String()).Msg("Invalid Go token position")
		return option.None[*SrcLocation]()
	}

	return option.Some(&SrcLocation{
		File: &File{
			RelPath:  start.Filename,
			FullPath: start.Filename,
			Contents: bytes,
		},
		Start: Pos{Line: start.Line, Col: start.Column},
		End:   Pos{Line: end.Line, Col: end.Column},
		Type:  LocError,
	})
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
		end = start
		// If the file is not parsable for some reason (e.g. syntax error), we can't determine the end position
		// based on ast.Nodes. If so, we can fall back on guessing the end position by looking for common delimiters
		offset, ok := findPositionOffset(start, fileBody)
		if !ok {
			return end
		}
		endOffset := GuessEndColumn(fileBody, offset)
		end.Column += endOffset - offset
		return end
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

func findPositionOffset(pos token.Position, data []byte) (int, bool) {
	line, col := 1, 1
	for i, c := range data {
		if line == pos.Line && col == pos.Column {
			return i, true
		} else if line > pos.Line {
			return -1, false
		}
		if c == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return -1, false
}

func GuessEndColumn(data []byte, offset int) int {
	var params, brackets, braces int
	inBackticks := false

	for i := offset; i < len(data); i++ {
		switch data[i] {
		case '(':
			params++
		case '[':
			brackets++
		case '{':
			braces++
		case ')':
			params--
			if params <= 0 {
				return i + 1
			}
		case ']':
			brackets--
			if brackets <= 0 {
				return i + 1
			}
		case '}':
			braces--
			if braces <= 0 {
				return i + 1
			}
		case '`':
			inBackticks = !inBackticks
		case ';', ',', ':', '"', '\'':
			if !inBackticks && params == 0 && brackets == 0 && braces == 0 {
				return i + 1
			}
		case ' ', '\t', '\n', '\r':
			if params == 0 && brackets == 0 && braces == 0 {
				return i + 1
			}
		}
	}

	return len(data) + 1
}
