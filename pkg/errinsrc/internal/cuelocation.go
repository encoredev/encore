package internal

import (
	"os"
	"sort"
	"strings"

	cueerrors "cuelang.org/go/cue/errors"
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

	return &SrcLocation{
		File: &File{
			RelPath:  strings.TrimPrefix(cueLoc.Filename(), pathPrefix),
			FullPath: cueLoc.Filename(),
			Contents: bytes,
		},
		Start: Pos{Line: cueLoc.Line(), Col: cueLoc.Column()},
		End:   Pos{Line: cueLoc.Line(), Col: cueLoc.Column()},
		Type:  LocError,
	}
}
