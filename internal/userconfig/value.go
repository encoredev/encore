package userconfig

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

type Kind int

const (
	String Kind = iota + 1
	Bool
	Int
	Uint
)

func (k Kind) String() string {
	switch k {
	case String:
		return "string"
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Uint:
		return "uint"
	default:
		return "unknown kind"
	}
}

func (k Kind) HumanString() string {
	switch k {
	case String:
		return "a string"
	case Bool:
		return "a boolean (true/false)"
	case Int:
		return "an integer"
	case Uint:
		return "an unsigned integer (>=0)"
	default:
		return "an unknown kind"
	}
}

type Type struct {
	Kind    Kind
	Default *any  // nil means no default
	Oneof   []any // nil means no restrictions
}

type Value struct {
	Val  any
	Type Type
}

func (v Value) String() string {
	return RenderValue(v.Val)
}

func (t Type) ParseAndValidate(val string) (any, error) {
	parsed, err := t.Kind.parseValue(val)
	if err != nil {
		return nil, err
	} else if err := t.validate(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (t Type) validate(val any) error {
	if val == nil {
		return errors.New("value cannot be nil")
	}
	if len(t.Oneof) > 0 {
		for _, v := range t.Oneof {
			if val == v {
				return nil
			}
		}

		strVal := fmt.Sprintf("%v", val)
		return errors.Errorf("value %q is not one of: %s", strVal, RenderOneof(t.Oneof))
	}

	if k, ok := kindOf(val); ok {
		if k != t.Kind {
			return errors.Errorf("value v is not %s", t.Kind.HumanString())
		}
	}

	return nil
}

func RenderValue(v any) string {
	return fmt.Sprintf("%v", v)
}

func RenderOneof(oneof []any) string {
	if len(oneof) == 0 {
		return ""
	}

	// Render as "a, b, or c"
	var s strings.Builder
	for i, v := range oneof {
		if i > 0 {
			if i == len(oneof)-1 {
				if len(oneof) > 2 {
					s.WriteString(", or ")
				} else {
					s.WriteString(" or ")
				}
			} else {
				s.WriteString(", ")
			}
		}

		s.WriteString(RenderValue(v))
	}
	return s.String()
}

func (k Kind) parseValue(value string) (any, error) {
	switch k {
	case String:
		return value, nil
	case Bool:
		return strconv.ParseBool(value)
	case Int:
		return strconv.ParseInt(value, 10, 64)
	case Uint:
		return strconv.ParseUint(value, 10, 64)
	default:
		return nil, fmt.Errorf("unknown kind %v", k)
	}
}

func KindOf[T interface{ string | bool | int | uint }](val T) (k Kind, ok bool) {
	return kindOf(val)
}

func kindOf(val any) (k Kind, ok bool) {
	switch val.(type) {
	case string:
		return String, true
	case bool:
		return Bool, true
	case int:
		return Int, true
	case uint:
		return Uint, true
	default:
		return 0, false
	}
}
