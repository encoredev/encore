package internal

import (
	"cmp"
	"slices"
	"sort"
)

type LocationType uint8

const (
	LocError   LocationType = iota // Renders in red
	LocWarning                     // Renders in yellow
	LocHelp                        // Renders in blue
)

type SrcLocation struct {
	Type  LocationType `json:"type"`           // The type of this location
	Text  string       `json:"text,omitempty"` // Optional text to render at this location
	File  *File        `json:"file,omitempty"` // The file containing the error
	Start Pos          `json:"start"`          // The position this location starts at
	End   Pos          `json:"end"`            // The position this location ends at
}

func (s *SrcLocation) Less(other *SrcLocation) bool {
	// Order by type of location first (Err, then Warn, then Help)
	// as we always want errors rendered above warnings and warnings above help
	// if s.Type != other.Type {
	// 	return s.Type < other.Type
	// }

	// Order by file first
	if s.File.FullPath != other.File.FullPath {
		return s.File.FullPath < other.File.FullPath
	}

	// And then by the line number of where they start
	if s.Start.Line != other.Start.Line {
		return s.Start.Line < other.Start.Line
	}

	// And then where they start on that line
	if s.Start.Col != other.Start.Col {
		return s.Start.Col < other.Start.Col
	}

	// And which line they end on
	if s.End.Line != other.End.Line {
		return s.End.Line < other.End.Line
	}

	// Then by ending column
	if s.End.Col != other.End.Col {
		return s.End.Col < other.End.Col
	}

	// Finally, by the text in the location
	if s.Text != other.Text {
		return s.Text < other.Text
	}

	if s.Type != other.Type {
		return s.Type < other.Type
	}

	return false
}

type Pos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type File struct {
	RelPath  string // The relative path within the project
	FullPath string // The full path to the file
	Contents []byte // The contents of the file
}

// SrcLocations represents a list of locations
// within the source code. It can be sorted and split
// up into separate lists using GroupByFile
type SrcLocations []*SrcLocation

var _ sort.Interface = SrcLocations{}

func (s SrcLocations) Len() int {
	return len(s)
}

func (s SrcLocations) Less(i, j int) bool {
	return s[i].Less(s[j])
}

func (s SrcLocations) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// GroupByFile groups all locations by file and returns a new
// SrcLocations for each file.
//
// If a file has overlapping locations or two locations on the same line
// then more than one SrcLocations will be returned for that file. This
// is due to a limitation in the srcrender, and may be relaxed in the future.
func (s SrcLocations) GroupByFile() []SrcLocations {
	type locationGroup struct {
		fileName  string
		locations SrcLocations
	}

	var nonOverlappingLocations []*locationGroup

	inlineOnSameLine := func(a, b *SrcLocation) bool {
		return a.Start.Line == a.End.Line &&
			b.Start.Line == b.End.Line &&
			a.Start.Line == b.Start.Line &&
			a.Text == "" && b.Text == "" && // Don't inline if there is text as we don't support rendering this yet
			((a.Start.Col > b.End.Col) ||
				(a.End.Col < b.Start.Col))

	}

	// Add locations to groups on the same file without overlaps
nextOriginalLoc:
	for _, loc := range s {
		// Attempt to match it into an existing group
		for _, grp := range nonOverlappingLocations {
			if grp.fileName == loc.File.FullPath {
				for _, other := range grp.locations {
					if other.Start.Line > loc.End.Line ||
						other.End.Line < loc.Start.Line ||
						inlineOnSameLine(other, loc) {
						grp.locations = append(grp.locations, loc)
						continue nextOriginalLoc
					}
				}
			}
		}

		// if here we found no matching groups
		nonOverlappingLocations = append(nonOverlappingLocations, &locationGroup{
			fileName:  loc.File.FullPath,
			locations: SrcLocations{loc},
		})
	}

	// Sort the locations in each group
	rtn := make([]SrcLocations, len(nonOverlappingLocations))
	for i, grp := range nonOverlappingLocations {
		sort.Sort(grp.locations)
		rtn[i] = grp.locations
	}

	// Now sort the groups by the lowest location hint
	// this means that errors will be rendered first, then warnings, then help
	// if they are in different files
	slices.SortStableFunc(rtn, func(a, b SrcLocations) int {
		lowestA := LocHelp
		lowestB := LocHelp

		for _, loc := range a {
			if loc.Type < lowestA {
				lowestA = loc.Type
			}
		}

		for _, loc := range b {
			if loc.Type < lowestB {
				lowestB = loc.Type
			}
		}

		return cmp.Compare(lowestA, lowestB)
	})

	return rtn
}
