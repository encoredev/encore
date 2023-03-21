// Package paths parses API paths.
package paths

import (
	"errors"
	"fmt"
	"go/token"
	"net/url"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// Path represents a parsed path.
type Path struct {
	Pos      token.Pos
	Segments []Segment
	Type     Type
}

// String returns the path's string representation.
func (p *Path) String() string {
	var b strings.Builder
	for i, s := range p.Segments {
		if i != 0 || p.Type.LeadingSlash() {
			b.WriteByte('/')
		}

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

// NumParams reports the number of parameterized (non-literal) segments in the path.
func (p *Path) NumParams() int {
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
	Type      SegmentType
	Value     string // literal if Type == Literal; name of parameter otherwise
	ValueType schema.Builtin
}

func (s *Segment) String() string {
	switch s.Type {
	case Param:
		return ":" + s.Value
	case Wildcard:
		return "*" + s.Value
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
	// Wildcard reprsents zero or more path segments of any value.
	Wildcard
)

type Type int

const (
	URL           Type = 0
	CacheKeyspace Type = 1
)

func (t Type) LeadingSlash() bool {
	switch t {
	case URL:
		return true
	default:
		return false
	}
}

func (t Type) SupportsWildcards() bool {
	switch t {
	case URL:
		return true
	default:
		return false
	}
}

// Parse parses a slash-separated path into path segments.
func Parse(pos token.Pos, initialPath string, typ Type) (*Path, error) {
	path := initialPath
	if path == "" {
		return nil, errors.New("empty path")
	}

	switch typ {
	case URL:
		if path[0] != '/' {
			return nil, errors.New("path must begin with '/'")
		}
		if _, err := url.ParseRequestURI(path); err != nil {
			return nil, fmt.Errorf("invalid path: %v", errors.Unwrap(err))
		} else if idx := strings.IndexByte(path, '?'); idx != -1 {
			return nil, fmt.Errorf("path cannot contain '?'")
		}
	case CacheKeyspace:
		if path[0] == '/' {
			return nil, errors.New("path must not begin with '/'")
		}
	}

	var segs []Segment
	leadingSlash := typ.LeadingSlash()
	for path != "" {
		if leadingSlash || len(segs) > 0 {
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
		} else if val != "" && val[0] == '*' && typ.SupportsWildcards() {
			segType = Wildcard
			val = val[1:]
		}
		segs = append(segs, Segment{Type: segType, Value: val, ValueType: schema.Builtin_STRING})
	}

	// Validate the segments
	for i, s := range segs {
		switch s.Type {
		case Literal:
			if s.Value == "" && initialPath != "/" {
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

	return &Path{Pos: pos, Segments: segs, Type: typ}, nil
}

func (p *Path) ToProto() *meta.Path {
	mp := &meta.Path{}
	switch p.Type {
	case URL:
		mp.Type = meta.Path_URL
	case CacheKeyspace:
		mp.Type = meta.Path_CACHE_KEYSPACE
	default:
		panic(fmt.Sprintf("path: unknown type %+v", p.Type))
	}

	for _, seg := range p.Segments {
		s := &meta.PathSegment{Value: seg.Value}
		switch seg.Type {
		case Literal:
			s.Type = meta.PathSegment_LITERAL
		case Param:
			s.Type = meta.PathSegment_PARAM
		case Wildcard:
			s.Type = meta.PathSegment_WILDCARD
		default:
			panic(fmt.Sprintf("unhandled path segment type %v", seg.Type))
		}

		if s.Type != meta.PathSegment_LITERAL {
			switch seg.ValueType {
			case schema.Builtin_STRING:
				s.ValueType = meta.PathSegment_STRING
			case schema.Builtin_BOOL:
				s.ValueType = meta.PathSegment_BOOL
			case schema.Builtin_INT8:
				s.ValueType = meta.PathSegment_INT8
			case schema.Builtin_INT16:
				s.ValueType = meta.PathSegment_INT16
			case schema.Builtin_INT32:
				s.ValueType = meta.PathSegment_INT32
			case schema.Builtin_INT64:
				s.ValueType = meta.PathSegment_INT64
			case schema.Builtin_INT:
				s.ValueType = meta.PathSegment_INT
			case schema.Builtin_UINT8:
				s.ValueType = meta.PathSegment_UINT8
			case schema.Builtin_UINT16:
				s.ValueType = meta.PathSegment_UINT16
			case schema.Builtin_UINT32:
				s.ValueType = meta.PathSegment_UINT32
			case schema.Builtin_UINT64:
				s.ValueType = meta.PathSegment_UINT64
			case schema.Builtin_UINT:
				s.ValueType = meta.PathSegment_UINT
			case schema.Builtin_UUID:
				s.ValueType = meta.PathSegment_UUID
			default:
				panic(fmt.Sprintf("unhandled path segment value type %v", seg.ValueType))
			}
		}

		mp.Segments = append(mp.Segments, s)
	}
	return mp
}

// Set tracks a set of paths, ensuring they are compatible with each other.
// The zero value is ready to use.
type Set struct {
	methods map[string]*node
}

// Add adds a path to the set of paths.
// Errors are always of type *ConflictError.
func (s *Set) Add(method string, path *Path) error {
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

func (s *Set) match(path *Path, seg Segment, curr *node) (next *node, err error) {
	for _, ch := range curr.children {
		switch ch.s.Type {
		case Wildcard:
			switch seg.Type {
			case Param:
				return nil, s.conflictErr(path, ch, "cannot combine parameter ':%s' with path '%s'", seg.Value, ch.findPath())
			case Wildcard:
				return nil, s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
			case Literal:
				return nil, s.conflictErr(path, ch, "cannot combine segment '%s' with path '%s'", seg.Value, ch.findPath())
			}
		case Param:
			switch seg.Type {
			case Param:
				return ch, nil
			case Wildcard:
				return nil, s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
			case Literal:
				return nil, s.conflictErr(path, ch, "cannot combine path segment '%s' with path '%s'", seg.Value, ch.findPath())
			}
		case Literal:
			switch seg.Type {
			case Wildcard:
				return nil, s.conflictErr(path, ch, "cannot combine wildcard '*%s' with path '%s'", seg.Value, ch.findPath())
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
