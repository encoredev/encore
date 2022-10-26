package internal

import (
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"github.com/rs/zerolog/log"
)

// LocationsFromCueError returns a list of SrcLocations based on what was given in the
// cueerror.Error.
func LocationsFromCueError(err cueerrors.Error, pathPrefix string) SrcLocations {
	// Convert cueerror.Pos to a *CueLocation
	rtn := make(SrcLocations, 0, len(err.InputPositions()))

	if pos := err.Position(); pos.IsValid() {
		rtn = append(rtn, FromCueTokenPos(pos, pathPrefix))
	}

	for _, pos := range err.InputPositions() {
		if pos.IsValid() {
			rtn = append(rtn, FromCueTokenPos(pos, pathPrefix))
		}
	}
	sort.Sort(rtn)

	return rtn
}

// FromCueTokenPos converts a cueerror.Pos to a SrcLocation
//
// We use an interface for `cue/token.Pos` so we can test it
func FromCueTokenPos(cueLoc interface {
	Filename() string
	Line() int
	Column() int
}, pathPrefix string) *SrcLocation {
	// Note: for CUE files we must read the bytes now, as the defer on the CUE load code will delete
	// the source file form the disk before this location is rendered
	bytes, err := os.ReadFile(cueLoc.Filename())
	if err != nil {
		log.Err(err).Str("filename", cueLoc.Filename()).Msg("Failed to read CUE file")
		// Don't return, `bytes == nil` is fine here
	}

	start := Pos{Line: cueLoc.Line(), Col: cueLoc.Column()}
	end := convertSingleCUEPositionToRange(cueLoc.Filename(), bytes, start)

	return &SrcLocation{
		File: &File{
			RelPath:  strings.TrimPrefix(cueLoc.Filename(), pathPrefix),
			FullPath: cueLoc.Filename(),
			Contents: bytes,
		},
		Start: start,
		End:   end,
		Type:  LocError,
	}
}

// convertSingleCUEPositionToRange attempts to convert a CUE error from a single position to a range
//
// It does this by running the CUE parser over the file and looking for the AST node that starts
// at the same line and column. Once found, we use the end position of that node as the end position
// of the error.
func convertSingleCUEPositionToRange(filename string, bytes []byte, start Pos) Pos {
	file, err := parser.ParseFile(filename, bytes, parser.ParseComments)
	if err == nil {
		var matching ast.Node

		ast.Walk(file, func(node ast.Node) bool {
			if node.Pos().Line() == start.Line && node.Pos().Column() == start.Col {
				matching = node
				return false
			}

			return true
		}, nil)

		if matching != nil {
			return Pos{Line: matching.End().Line(), Col: matching.End().Column()}
		}
	}

	return start
}
