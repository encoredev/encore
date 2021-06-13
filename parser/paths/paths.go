// Package paths parses API paths.
package paths

import (
	"errors"
	"fmt"
	"go/token"
	"net/url"
	"strings"
)

// Path represents a parsed path.
type Path struct {
	Pos      token.Pos
	Segments []Segment
}

// String returns the path's string representation.
func (p *Path) String() string {
	var b strings.Builder
	for _, s := range p.Segments {
		b.WriteByte('/')
		switch s.Type {
		case Param:
			b.WriteByte(':')
		case Wildcard:
			b.WriteByte('*')
		}
		b.WriteString(s.Value)
	}
	return b.String()
}

// Segment represents a parsed path segment.
type Segment struct {
	Type  SegmentType
	Value string // literal if Type == Literal; name of parameter otherwise
}

// SegmentType represents the different types of path segments recognized by the parser.
type SegmentType int

const (
	// Literal is a literal string path segment.
	Literal SegmentType = iota
	// Param represents a single path segment of any (non-empty) value.
	Param
	// Wildcard reprsents zero or more path segments of any value.
	Wildcard
)

// Parse parses a rooted path (starting with '/') into path segments.
func Parse(pos token.Pos, path string) (*Path, error) {
	if path == "" || path[0] != '/' {
		return nil, errors.New("path must begin with '/'")
	} else if _, err := url.ParseRequestURI(path); err != nil {
		return nil, fmt.Errorf("invalid path: %v", errors.Unwrap(err))
	} else if idx := strings.IndexByte(path, '?'); idx != -1 {
		return nil, fmt.Errorf("path cannot contain '?'")
	}

	var segs []Segment
	for path != "" {
		path = path[1:] // drop leading '/'
		// Find the next path segment
		var val string
		switch idx := strings.IndexByte(path, '/'); idx {
		case 0:
			return nil, fmt.Errorf("path cannot contain double slash")
		case -1:
			val = path
			path = ""
		default:
			val = path[:idx]
			path = path[idx:]
		}

		typ := Literal
		if val != "" && val[0] == ':' {
			typ = Param
			val = val[1:]
		} else if val != "" && val[0] == '*' {
			typ = Wildcard
			val = val[1:]
		}
		segs = append(segs, Segment{Type: typ, Value: val})
	}

	// Validate the segments
	for i, s := range segs {
		switch s.Type {
		case Literal:
			if s.Value == "" {
				return nil, fmt.Errorf("path cannot contain trailing slash")
			}
		case Param:
			if s.Value == "" {
				return nil, fmt.Errorf("path parameter must have a name")
			} else if !token.IsIdentifier(s.Value) {
				return nil, fmt.Errorf("path parameter must be a valid Go identifier name")
			}
		case Wildcard:
			if s.Value == "" {
				return nil, fmt.Errorf("wildcard parameter must have a name")
			} else if !token.IsIdentifier(s.Value) {
				return nil, fmt.Errorf("wildcard parameter must be a valid Go identifier name")
			} else if len(segs) > (i + 1) {
				return nil, fmt.Errorf("wildcard parameter must be the last path segment")
			}
		}
	}

	return &Path{Pos: pos, Segments: segs}, nil
}

// Set tracks a set of paths, ensuring they are compatible with each other.
// The zero value is ready to use.
type Set struct {
	root node
}

// Add adds a path to the set of paths.
// Errors are always of type *ConflictError.
func (s *Set) Add(path *Path) error {
	curr := &s.root
SegLoop:
	for _, seg := range path.Segments {
		// Find the child matching seg
		for _, ch := range curr.children {
			switch ch.s.Type {
			case Wildcard:
				switch seg.Type {
				case Param:
					return s.conflictErr(path, ch, "cannot combine parameter ':%s' with path '%s'", seg.Value, ch.findPath())
				case Wildcard:
					return s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
				case Literal:
					return s.conflictErr(path, ch, "cannot combine segment '%s' with path '%s'", seg.Value, ch.findPath())
				}
			case Param:
				switch seg.Type {
				case Param:
					curr = ch
					continue SegLoop
				case Wildcard:
					return s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
				case Literal:
					return s.conflictErr(path, ch, "cannot combine path segment '%s' with path '%s'", seg.Value, ch.findPath())
				}
			case Literal:
				switch seg.Type {
				case Wildcard:
					return s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
				case Param:
					return s.conflictErr(path, ch, "cannot combine parameter ':%s' with path '%s'", seg.Value, ch.findPath())
				case Literal:
					if seg.Value == ch.s.Value {
						curr = ch
						continue SegLoop
					}
				}
			}
		}

		// Could not find a match; add it to the tree
		curr.children = append(curr.children, &node{s: seg})
		curr = curr.children[len(curr.children)-1]
	}

	if curr.p != nil {
		return s.conflictErr(path, curr, "duplicate path")
	}
	curr.p = path
	return nil
}

// ConflictError represents a conflict between two paths.
type ConflictError struct {
	Path    *Path
	Other   *Path
	Context string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("path conflict: %s and %s: %s", e.Path, e.Other, e.Context)
}

type node struct {
	s        Segment
	children []*node
	p        *Path // leaf path, if any
}

func (n *node) findPath() *Path {
	for n.p == nil {
		n = n.children[0]
	}
	return n.p
}

func (s *Set) conflictErr(path *Path, node *node, format string, args ...interface{}) error {
	other := node.findPath()
	return &ConflictError{Path: path, Other: other, Context: fmt.Sprintf(format, args...)}
}
