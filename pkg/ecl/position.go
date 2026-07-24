package ecl

import (
	"fmt"
	"strings"
)

// Position is a location in an ECL source file.
// Lines and columns are 1-based; columns count runes, not bytes.
type Position struct {
	File   string
	Offset int // byte offset within the file
	Line   int
	Column int
}

// IsValid reports whether the position refers to an actual source location.
func (p Position) IsValid() bool { return p.Line > 0 }

func (p Position) String() string {
	switch {
	case p.File == "" && !p.IsValid():
		return "<unknown position>"
	case !p.IsValid():
		return p.File
	case p.File == "":
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	default:
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
	}
}

// Span is a contiguous range of source text.
type Span struct {
	Start, End Position
}

// sourceFile holds the contents of a parsed file for snippet rendering.
type sourceFile struct {
	name      string
	src       string
	lineStart []int // byte offset of the start of each line, 0-based index
}

func newSourceFile(name, src string) *sourceFile {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &sourceFile{name: name, src: src, lineStart: starts}
}

// line returns the text of the given 1-based line, without the trailing newline.
func (f *sourceFile) line(n int) (string, bool) {
	if f == nil || n < 1 || n > len(f.lineStart) {
		return "", false
	}
	start := f.lineStart[n-1]
	end := len(f.src)
	if n < len(f.lineStart) {
		end = f.lineStart[n] - 1
	}
	return strings.TrimSuffix(f.src[start:end], "\r"), true
}
