package selector

import (
	"go/ast"
	"go/token"
	"regexp"
	"sort"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internal/perr"
)

type Type string

// NOTE: New types added should be also added to meta.proto and est/selectors.go.

const (
	All Type = "all"
	Tag Type = "tag"
)

type Selector struct {
	Type     Type
	Value    string
	startPos token.Pos
	endPos   token.Pos
}

var _ ast.Node = Selector{}

func (s Selector) Pos() token.Pos {
	return s.startPos
}

func (s Selector) End() token.Pos {
	return s.endPos
}

func (s Selector) String() string {
	if s.Type == All {
		return "all"
	}
	return string(s.Type) + ":" + s.Value
}

// Equals reports whether s and o are equal on type and value.
//
// It does not compare the start and end positions.
func (s Selector) Equals(o Selector) bool {
	return s.Type == o.Type && s.Value == o.Value
}

func (s Selector) ToProto() *meta.Selector {
	pb := &meta.Selector{Type: meta.Selector_UNKNOWN, Value: s.Value}
	switch s.Type {
	case All:
		pb.Type = meta.Selector_ALL
	case Tag:
		pb.Type = meta.Selector_TAG
	}
	return pb
}

func Parse(errs *perr.List, startPos token.Pos, s string) (Selector, bool) {
	if s == "all" {
		return Selector{Type: All, Value: ""}, true
	}

	typ, val, ok := strings.Cut(s, ":")
	if !ok {
		errs.Add(errMissingSelectorType.AtGoPos(startPos, startPos+token.Pos(len([]byte(s)))))
		return Selector{}, false
	}

	sel := Selector{
		Type:     Type(typ),
		Value:    val,
		startPos: startPos,
		endPos:   startPos + token.Pos(len([]byte(s))),
	}

	var re *regexp.Regexp
	switch sel.Type {
	case Tag:
		re = tagRegexp
	default:
		errs.Add(errUnknownSelectorType(typ).AtGoNode(sel))
		return Selector{}, false
	}

	if !re.MatchString(val) {
		errs.Add(errInvalidSelectorValue(val).AtGoNode(sel))
		return Selector{}, false
	}

	return sel, true
}

var (
	tagRegexp = regexp.MustCompile(`^[a-z]([-_a-z0-9]*[a-z0-9])?$`)
)

type Set struct{ vals []Selector }

// Add adds a selector to the set. It reports whether the selector was added,
// meaning it reports false iff the set already contained that selector.
//
// Add ensures that the set is sorted.
func (s *Set) Add(sel Selector) (added bool) {
	idx := sort.Search(len(s.vals), func(i int) bool { return (s.vals)[i].Value >= sel.Value })
	if idx < len(s.vals) && (s.vals)[idx].Equals(sel) {
		return false
	}

	// Insert at the end
	if idx == len(s.vals) {
		s.vals = append(s.vals, sel)
		return true
	}

	// Make space for the new selector by shifting the rest of the slice
	s.vals = append(s.vals[:idx+1], s.vals[idx:]...)

	// Insert the new selector
	s.vals[idx] = sel

	return true
}

// Merge adds all the selectors from other to s.
//
// It is equivalent to calling Add for each selector in other.
func (s *Set) Merge(other Set) {
	for _, sel := range other.vals {
		s.Add(sel)
	}
}

// Contains reports whether the set contains the given selector.
func (s *Set) Contains(sel Selector) bool {
	idx := sort.Search(len(s.vals), func(i int) bool { return (s.vals)[i].Value >= sel.Value })
	return idx < len(s.vals) && (s.vals)[idx].Equals(sel)
}

// ContainsAny reports whether the set contains any of the selectors in other.
//
// It compares in linear time O(N) where N is the number of selectors in the
// larger set. It is faster than calling Contains for each selector in other.
func (s *Set) ContainsAny(other Set) bool {
	i, j := 0, 0
	for i < len(s.vals) && j < len(other.vals) {
		if s.vals[i].Equals(other.vals[j]) {
			return true
		} else if s.vals[i].Value < other.vals[j].Value {
			i++
		} else {
			j++
		}
	}
	return false
}

// ForEach calls fn for each selector in the set.
func (s *Set) ForEach(fn func(Selector)) {
	for _, sel := range s.vals {
		fn(sel)
	}
}

// Len returns the number of selectors in the set.
func (s *Set) Len() int {
	return len(s.vals)
}

// ToProto returns the set as a slice of proto selectors.
func (s *Set) ToProto() []*meta.Selector {
	pbs := make([]*meta.Selector, len(s.vals))
	for i, sel := range s.vals {
		pbs[i] = sel.ToProto()
	}
	return pbs
}
