// Package resourcepaths parses API and other resource paths.
package resourcepaths

import (
	"fmt"
	"go/ast"
	"go/token"
	"net/url"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/schema"
)

// Path represents a parsed path.
type Path struct {
	StartPos token.Pos
	Segments []Segment
}

var _ ast.Node = (*Path)(nil)

func (p *Path) Pos() token.Pos {
	return p.StartPos
}

func (p *Path) End() token.Pos {
	if len(p.Segments) > 0 {
		return p.Segments[len(p.Segments)-1].End()
	}
	return p.StartPos
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
		case Fallback:
			b.WriteByte('!')
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

// Params returns the segments that are not literals.
func (p *Path) Params() []Segment {
	var params []Segment
	for _, s := range p.Segments {
		if s.Type != Literal {
			params = append(params, s)
		}
	}
	return params
}

// HasFallback is true if the path contains a fallback segment.
func (p *Path) HasFallback() bool {
	for _, s := range p.Segments {
		if s.Type == Fallback {
			return true
		}
	}
	return false
}

// Segment represents a parsed path segment.
type Segment struct {
	Type      SegmentType
	Value     string // literal if Type == Literal; name of parameter otherwise
	ValueType schema.BuiltinKind
	StartPos  token.Pos
	EndPos    token.Pos
}

var _ ast.Node = Segment{}

func (s *Segment) String() string {
	switch s.Type {
	case Param:
		return ":" + s.Value
	case Wildcard:
		return "*" + s.Value
	case Fallback:
		return "!" + s.Value
	default:
		return s.Value
	}
}

func (s Segment) Pos() token.Pos {
	return s.StartPos
}

func (s Segment) End() token.Pos {
	return s.EndPos
}

// SegmentType represents the different types of path segments recognized by the parser.
type SegmentType int

const (
	// Literal is a literal string path segment.
	Literal SegmentType = iota
	// Param represents a single path segment of any (non-empty) value.
	Param
	// Wildcard represents zero or more path segments of any value.
	Wildcard
	// Fallback represents zero or more path segments of any value
	// that are lower priority than any other path.
	Fallback
)

type Options struct {
	// AllowWildcard indicates whether the parser should allow wildcard segments.
	AllowWildcard bool

	// AllowFallback indicates whether the parser should allow fallback segments.
	AllowFallback bool

	// PrefixSlash indicates whether the parser should require a leading slash
	// or require that it's not present
	PrefixSlash bool
}

// Parse parses a slash-separated path into path segments.
//
// strPos is the position of where the path string was found in the source code.
func Parse(errs *perr.List, startPos token.Pos, path string, options Options) (parsedPath *Path, ok bool) {
	endPos := token.Pos(len([]byte(path))) + startPos

	if path == "" {
		errs.Add(errEmptyPath.AtGoPos(startPos, endPos))
		return nil, false
	} else if path[0] != '/' && options.PrefixSlash {
		errs.Add(errInvalidPathMissingPrefix.AtGoPos(startPos, endPos))
		return nil, false
	} else if path[0] == '/' && !options.PrefixSlash {
		errs.Add(errInvalidPathPrefix.AtGoPos(startPos, endPos))
		return nil, false
	}

	urlPath := path
	if !options.PrefixSlash {
		urlPath = "/" + urlPath
	}
	if _, err := url.ParseRequestURI(urlPath); err != nil {
		errs.Add(errInvalidPathURI.AtGoPos(startPos, endPos).Wrapping(err))
		return nil, false
	} else if idx := strings.IndexByte(path, '?'); idx != -1 {
		errs.Add(errPathContainedQuery.AtGoPos(startPos, endPos))
		return nil, false
	}

	var segs []Segment
	segStart := startPos
	for path != "" {
		if options.PrefixSlash || len(segs) > 0 {
			path = path[1:] // drop leading '/'
			segStart++
		}

		// Find the next path segment
		var val string
		idx := strings.IndexByte(path, '/')
		segEnd := segStart
		switch idx {
		case 0:
			errs.Add(errEmptySegment.AtGoPos(segStart-1, segStart))
			return nil, false
		case -1:
			val = path
			path = ""
		default:
			val = path[:idx]
			path = path[idx:]
		}
		segEnd += token.Pos(len([]byte(val))) - 1

		segType := Literal
		if val != "" && val[0] == ':' {
			segType = Param
			val = val[1:]
		} else if val != "" && val[0] == '*' && options.AllowWildcard {
			segType = Wildcard
			val = val[1:]
		} else if val != "" && val[0] == '!' && options.AllowFallback {
			segType = Fallback
			val = val[1:]
		}
		segs = append(segs, Segment{
			Type: segType, Value: val, ValueType: schema.String,
			StartPos: segStart - 1, EndPos: segEnd,
		})
		segStart = segEnd + 1
	}

	// Validate the segments
	for i, s := range segs {
		switch s.Type {
		case Literal:
			if s.Value == "" {
				errs.Add(errTrailingSlash.AtGoNode(s))
				return nil, false
			}
		case Param:
			switch {
			case s.Value == "":
				errs.Add(errParameterMissingName.AtGoNode(s))
				return nil, false
			case !token.IsIdentifier(s.Value):
				errs.Add(errInvalidParamIdentifier.AtGoNode(s))
				return nil, false
			}
		case Wildcard:
			switch {
			case s.Value == "":
				errs.Add(errParameterMissingName.AtGoNode(s))
				return nil, false
			case !token.IsIdentifier(s.Value):
				errs.Add(errInvalidParamIdentifier.AtGoNode(s))
				return nil, false
			case len(segs) > (i + 1):
				errs.Add(errWildcardNotLastSegment.AtGoNode(s))
				return nil, false
			}
		case Fallback:
			switch {
			case s.Value == "":
				errs.Add(errParameterMissingName.AtGoNode(s))
				return nil, false
			case !token.IsIdentifier(s.Value):
				errs.Add(errInvalidParamIdentifier.AtGoNode(s))
				return nil, false
			case len(segs) > (i + 1):
				errs.Add(errFallbackNotLastSegment.AtGoNode(s))
				return nil, false
			}
		}
	}

	return &Path{StartPos: startPos, Segments: segs}, true
}

func (p *Path) ToProto() *meta.Path {
	mp := &meta.Path{}
	mp.Type = meta.Path_URL

	for _, seg := range p.Segments {
		s := &meta.PathSegment{Value: seg.Value}
		switch seg.Type {
		case Literal:
			s.Type = meta.PathSegment_LITERAL
		case Param:
			s.Type = meta.PathSegment_PARAM
		case Wildcard:
			s.Type = meta.PathSegment_WILDCARD
		case Fallback:
			s.Type = meta.PathSegment_FALLBACK
		default:
			panic(fmt.Sprintf("unhandled path segment type %v", seg.Type))
		}

		if s.Type != meta.PathSegment_LITERAL {
			switch seg.ValueType {
			case schema.String:
				s.ValueType = meta.PathSegment_STRING
			case schema.Bool:
				s.ValueType = meta.PathSegment_BOOL
			case schema.Int:
				s.ValueType = meta.PathSegment_INT
			case schema.Int8:
				s.ValueType = meta.PathSegment_INT8
			case schema.Int16:
				s.ValueType = meta.PathSegment_INT16
			case schema.Int32:
				s.ValueType = meta.PathSegment_INT32
			case schema.Int64:
				s.ValueType = meta.PathSegment_INT64
			case schema.Uint:
				s.ValueType = meta.PathSegment_UINT
			case schema.Uint8:
				s.ValueType = meta.PathSegment_UINT8
			case schema.Uint16:
				s.ValueType = meta.PathSegment_UINT16
			case schema.Uint32:
				s.ValueType = meta.PathSegment_UINT32
			case schema.Uint64:
				s.ValueType = meta.PathSegment_UINT64
			case schema.UUID:
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

func NewSet() *Set {
	return &Set{
		methods: make(map[string]*node),
	}
}

// Add adds a path to the set of paths.
func (s *Set) Add(errs *perr.List, method string, path *Path) (ok bool) {
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
			next, ok := s.match(errs, path, seg, curr)
			if !ok {
				return false
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
			errs.Add(errDuplicatePath.AtGoNode(path).AtGoNode(curr.p))
			return false
		} else if m == method {
			curr.p = path
		}
	}

	return true
}

func (s *Set) match(errs *perr.List, path *Path, seg Segment, curr *node) (next *node, ok bool) {
	for _, ch := range curr.children {
		switch ch.s.Type {
		case Wildcard:
			switch seg.Type {
			case Param:
				errs.Add(errConflictingParameterizedPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			case Wildcard:
				errs.Add(errConflictingWildcardPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			case Literal:
				errs.Add(errConflictingLiteralPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			}
		case Param:
			switch seg.Type {
			case Param:
				return ch, true
			case Wildcard:
				errs.Add(errConflictingWildcardPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			case Literal:
				errs.Add(errConflictingLiteralPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			}
		case Literal:
			switch seg.Type {
			case Wildcard:
				errs.Add(errConflictingWildcardPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			case Param:
				errs.Add(errConflictingLiteralPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			case Literal:
				if seg.Value == ch.s.Value {
					return ch, true
				}
			}
		case Fallback:
			switch seg.Type {
			case Fallback:
				errs.Add(errConflictingFallbackPath(seg.Value, ch.findPath()).
					AtGoNode(path).
					AtGoNode(ch.findPath()))
				return nil, false
			}
		}
	}
	return nil, true
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
