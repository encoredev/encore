package cache

import (
	"errors"
	"fmt"
	"go/token"
	"strings"
)

// KeyspacePath represents a parsed keyspace path.
type KeyspacePath struct {
	Pos      token.Pos
	Segments []Segment
}

// String returns the path's string representation.
func (p *KeyspacePath) String() string {
	var b strings.Builder
	for i, s := range p.Segments {
		if i != 0 {
			b.WriteByte('/')
		}

		switch s.Type {
		case Param:
			b.WriteByte(':')
		}
		b.WriteString(s.Value)
	}
	return b.String()
}

// NumParams reports the number of parameterized (non-literal) segments in the path.
func (p *KeyspacePath) NumParams() int {
	n := 0
	for _, s := range p.Segments {
		if s.Type != Literal {
			n++
		}
	}
	return n
}

// Segment represents a parsed path segment.
type Segment struct {
	Type  SegmentType
	Value string // literal if Type == Literal; name of parameter otherwise
}

func (s *Segment) String() string {
	switch s.Type {
	case Param:
		return ":" + s.Value
	default:
		return s.Value
	}
}

// SegmentType represents the different types of path segments recognized by the parser.
type SegmentType int

const (
	// Literal is a literal string path segment.
	Literal SegmentType = iota
	// Param represents a single path segment of any (non-empty) value.
	Param
)

// ParseKeyspacePath parses a slash-separated path into path segments.
func ParseKeyspacePath(pos token.Pos, path string) (*KeyspacePath, error) {
	if path == "" {
		return nil, errors.New("empty path")
	} else if path[0] == '/' {
		return nil, errors.New("path must not begin with '/'")
	}

	var segs []Segment
	for path != "" {
		if len(segs) > 0 {
			path = path[1:] // drop leading '/'
		}

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

		segType := Literal
		if val != "" && val[0] == ':' {
			segType = Param
			val = val[1:]
		}
		segs = append(segs, Segment{Type: segType, Value: val})
	}

	// Validate the segments
	for _, s := range segs {
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
		}
	}

	return &KeyspacePath{Pos: pos, Segments: segs}, nil
}

// KeyspacePathSet tracks a set of paths, ensuring they are compatible with each other.
// The zero value is ready to use.
type KeyspacePathSet struct {
	methods map[string]*node
}

// Add adds a path to the set of paths.
// Errors are always of type *ConflictError.
func (s *KeyspacePathSet) Add(method string, path *KeyspacePath) error {
	if s.methods == nil {
		s.methods = make(map[string]*node)
	}

	var candidates []string
	if method == "*" {
		// Check all defined methods
		for m := range s.methods {
			if m != method {
				candidates = append(candidates, m)
			}
		}
	} else {
		candidates = []string{"*"}
	}

	// Always check the target method last, so we don't add to the set if there's an error
	// for another method.
	candidates = append(candidates, method)

CandidateLoop:
	for _, m := range candidates {
		curr := s.methods[m]
		if curr == nil {
			curr = &node{}
			s.methods[m] = curr
		}

		for _, seg := range path.Segments {
			next, err := s.match(path, seg, curr)
			if err != nil {
				return err
			} else if next != nil {
				curr = next
			} else {
				// Could not find a match; add it to the tree (if this is the target method)
				if m != method {
					continue CandidateLoop
				}
				curr.children = append(curr.children, &node{s: seg})
				curr = curr.children[len(curr.children)-1]
			}
		}

		if curr.p != nil {
			return s.conflictErr(path, curr, "duplicate path")
		} else if m == method {
			curr.p = path
		}
	}

	return nil
}

func (s *KeyspacePathSet) match(path *KeyspacePath, seg Segment, curr *node) (next *node, err error) {
	for _, ch := range curr.children {
		switch ch.s.Type {
		case Param:
			switch seg.Type {
			case Param:
				return ch, nil
			case Literal:
				return nil, s.conflictErr(path, ch, "cannot combine path segment '%s' with path '%s'", seg.Value, ch.findPath())
			}
		case Literal:
			switch seg.Type {
			case Param:
				return nil, s.conflictErr(path, ch, "cannot combine parameter ':%s' with path '%s'", seg.Value, ch.findPath())
			case Literal:
				if seg.Value == ch.s.Value {
					return ch, nil
				}
			}
		}
	}
	return nil, nil
}

// ConflictError represents a conflict between two paths.
type ConflictError struct {
	Path    *KeyspacePath
	Other   *KeyspacePath
	Context string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("path conflict: %s and %s: %s", e.Path, e.Other, e.Context)
}

type node struct {
	s        Segment
	children []*node
	p        *KeyspacePath // leaf path, if any
}

func (n *node) findPath() *KeyspacePath {
	for n.p == nil {
		n = n.children[0]
	}
	return n.p
}

func (s *KeyspacePathSet) conflictErr(path *KeyspacePath, node *node, format string, args ...interface{}) error {
	other := node.findPath()
	return &ConflictError{Path: path, Other: other, Context: fmt.Sprintf(format, args...)}
}
