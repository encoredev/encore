package selector

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
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

func Parse(s string) (Selector, error) {
	if s == "all" {
		return Selector{Type: All, Value: ""}, nil
	}

	typ, val, ok := strings.Cut(s, ":")
	if !ok {
		return Selector{}, errors.New("missing selector type")
	}

	sel := Selector{Type: Type(typ), Value: val}

	var re *regexp.Regexp
	switch sel.Type {
	case Tag:
		re = tagRegexp
	default:
		return Selector{}, fmt.Errorf("unknown selector type %q", typ)
	}

	if !re.MatchString(val) {
		return Selector{}, errors.New("invalid value")
	}

	return sel, nil
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
