package selector

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Type string

const (
	Tag Type = "tag"
)

type Selector struct {
	Type  Type
	Value string
}

func (s Selector) String() string {
	return string(s.Type) + ":" + s.Value
}

func (s Selector) ToProto() *meta.Selector {
	pb := &meta.Selector{Type: meta.Selector_UNKNOWN, Value: s.Value}
	switch s.Type {
	case Tag:
		pb.Type = meta.Selector_TAG
	}
	return pb
}

func Parse(s string) (Selector, error) {
	typ, val, ok := strings.Cut(s, ":")
	if !ok {
		return Selector{}, errors.New("missing selector type")
	}

	sel := Selector{Type: Type(typ), Value: val}
	switch sel.Type {
	case Tag:
	default:
		return Selector{}, fmt.Errorf("unknown selector type %q", typ)
	}

	if !valueRegexp.MatchString(val) {
		return Selector{}, errors.New("invalid value")
	}

	return sel, nil
}

var valueRegexp = regexp.MustCompile(`^[a-z]([-_a-z0-9]*[a-z0-9])?$`)

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
