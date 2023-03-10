package selector

import (
	"go/ast"
	"go/token"
	"regexp"
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
	Type  Type
	Value string
}

func (s Selector) String() string {
	if s.Type == All {
		return "all"
	}
	return string(s.Type) + ":" + s.Value
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

func Parse(errs *perr.List, node ast.Node, s string) (Selector, bool) {
	if s == "all" {
		return Selector{Type: All, Value: ""}, true
	}

	typ, val, ok := strings.Cut(s, ":")
	if !ok {
		errs.Add(errMissingSelectorType.AtGoNode(node))
		return Selector{}, false
	}

	sel := Selector{Type: Type(typ), Value: val}

	var re *regexp.Regexp
	switch sel.Type {
	case Tag:
		re = tagRegexp
	default:
		errs.Add(errUnknownSelectorType(typ).AtGoPos(node.Pos(), node.Pos()+token.Pos(len(typ))))
		return Selector{}, false
	}

	if !re.MatchString(val) {
		errs.Add(errInvalidSelectorValue(val).AtGoPos(node.Pos()+token.Pos(len(typ)+1), node.End()))
		return Selector{}, false
	}

	return sel, true
}

var (
	tagRegexp = regexp.MustCompile(`^[a-z]([-_a-z0-9]*[a-z0-9])?$`)
)

type Set []Selector

// Add adds a selector to the set. It reports whether the selector was added,
// meaning it reports false iff the set already contained that selector.
func (s *Set) Add(sel Selector) (added bool) {
	for _, sel2 := range *s {
		if sel2 == sel {
			return false
		}
	}
	*s = append(*s, sel)
	return true
}

func (s *Set) ToProto() []*meta.Selector {
	pbs := make([]*meta.Selector, len(*s))
	for i, sel := range *s {
		pbs[i] = sel.ToProto()
	}
	return pbs
}
