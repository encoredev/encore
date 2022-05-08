package serde

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDecoder_Payload(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  RequestDecoder
		err   string // expected error, as a regexp
	}{
		{
			name:  "null_object",
			input: `null`,
			want:  &PayloadTester{},
			err:   "",
		},
		{
			name:  "null_key",
			input: `{null: "blah"}"`,
			want:  &PayloadTester{},
			err:   `.*expect ".+but found n.*`,
		},
		{
			name:  "object_keys_case_match",
			input: `{"Foo": "bar"}`,
			want:  &PayloadTester{Foo: "bar"},
			err:   "",
		},
		{
			name:  "object_keys_case_insensitive",
			input: `{"foo": "bar"}`,
			want:  &PayloadTester{Foo: "bar"},
			err:   "",
		},
		{
			name:  "object_keys_case_insensitive2",
			input: `{"fOo": "bar"}`,
			want:  &PayloadTester{Foo: "bar"},
			err:   "",
		},
		{
			name:  "multiple_keys",
			input: `{"fOo": "bar", "bar": 1}`,
			want:  &PayloadTester{Foo: "bar", Bar: 1},
			err:   "",
		},
		{
			name:  "simple_maps",
			input: `{"Bar": 1, "Map": {"foo": 2, "Foo": 3}}`,
			want:  &PayloadTester{Bar: 1, Map: map[string]int{"foo": 2, "Foo": 3}},
			err:   "",
		},
		{
			name:  "complex_maps",
			input: `{"StructMap": {"foo": {"A": "a", "b": "b"}}}`,
			want: &PayloadTester{StructMap: map[string]struct{ A, B string }{
				"foo": {A: "a", B: "b"},
			}},
			err: "",
		},
		{
			name:  "nested_types",
			input: `{"Foo": "foo", "nested": {"Bar": 2}}`,
			want:  &PayloadTester{Foo: "foo", Nested: &PayloadTester{Bar: 2}},
			err:   "",
		},
		{
			name:  "pointer_nulls",
			input: `{"nested": null}`,
			want:  &PayloadTester{Nested: nil},
			err:   "",
		},
		{
			name:  "wrong_type_1",
			input: `{"foo": 1}`,
			want:  &PayloadTester{},
			err:   `.*expects " or n, but found 1.*`,
		},
		{
			name:  "wrong_type_2",
			input: `{"bar": "string"}`,
			want:  &PayloadTester{},
			err:   `.*unexpected character.*`,
		},
	}

	c := qt.New(t)
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			// Test our decoder.
			{
				dec := NewDecoder(RequestSpec{
					Payload: strings.NewReader(tt.input),
				})
				dst := reflect.New(reflect.ValueOf(tt.want).Elem().Type()).Interface().(RequestDecoder)
				dst.DecodeRequest(dec)
				err := dec.Err()

				if tt.err != "" {
					c.Assert(err, qt.ErrorMatches, tt.err)
				} else {
					c.Assert(err, qt.IsNil)
					c.Assert(dst, qt.DeepEquals, tt.want)
				}
			}

			// Test the standard library implementation to ensure same behavior.
			{
				dst := reflect.New(reflect.ValueOf(tt.want).Elem().Type()).Interface()
				err := json.Unmarshal([]byte(tt.input), dst)
				if tt.err != "" {
					c.Assert(err, qt.IsNotNil, qt.Commentf("expected failure, got success"))
				} else {
					c.Assert(err, qt.IsNil)
					c.Assert(dst, qt.DeepEquals, tt.want)
				}
			}
		})
	}
}

type PayloadTester struct {
	Foo string
	Bar int
	Map map[string]int

	StructMap map[string]struct{ A, B string }
	Nested    *PayloadTester
}

func (t *PayloadTester) DecodeRequest(dec *Decoder) bool {
	decodeMap := func() bool {
		if dec.Null() {
			t.Map = nil
			return true
		}
		MakeMap(&t.Map)
		return dec.MapCB(func(_ *Decoder, key string) bool {
			t.Map[key] = dec.Int()
			return true
		})
	}

	decodeStructMap := func() bool {
		if dec.Null() {
			t.StructMap = nil
			return true
		}
		MakeMap(&t.StructMap)
		return dec.MapCB(func(_ *Decoder, key string) bool {
			val := AllocMapValue(t.StructMap)
			if ok := dec.Any(&val); ok {
				t.StructMap[key] = val
				return true
			}
			return false
		})
	}

	return dec.ObjectCB(func(_ *Decoder, field string) bool {
		switch field {
		case "Foo", "foo":
			t.Foo = dec.String()
		case "Bar", "bar":
			t.Bar = dec.Int()
		case "Map", "map":
			return decodeMap()
		case "StructMap", "structMap":
			return decodeStructMap()
		case "Nested", "nested":
			if dec.Null() {
				t.Nested = nil
			} else {
				NewPtrValue(&t.Nested)
				return t.Nested.DecodeRequest(dec)
			}
		default:
			// Case-insensitive match?
			switch strings.ToLower(field) {
			case "foo":
				t.Foo = dec.String()
			case "bar":
				t.Foo = dec.String()
			case "map":
				return decodeMap()
			case "structmap":
				return decodeStructMap()
			case "nested":
				if dec.Null() {
					t.Nested = nil
				} else {
					NewPtrValue(&t.Nested)
					return t.Nested.DecodeRequest(dec)
				}
			default:
				// Unknown field
				dec.Skip()
			}
		}
		return true
	})
}
