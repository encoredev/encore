package scrub

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

var tests = []scannerTest{
	{
		name: "simple",
		src:  `{"foo": "bar"}`,
		want: []tokDesc{
			objStart, k(str("foo")), str("bar"), objEnd,
		},
	},
	{
		name: "missing_quotes",
		src:  `{"foo": "bar}`,
		want: []tokDesc{
			objStart, k(str("foo")), raw(`"bar}`),
		},
	},
	{
		name: "escaped_string",
		src:  `"foo\"bar"`,
		want: []tokDesc{
			str("foo\"bar"),
		},
	},
	{
		name: "escaped_string_at_end",
		src:  `"foo\""`,
		want: []tokDesc{
			str("foo\""),
		},
	},
	{
		name: "escaped_quoted_string",
		src:  `["\"one\""]`,
		want: []tokDesc{
			arrStart, str("\"one\""), arrEnd,
		},
	},
	{
		name: "newline_reset_key",
		src:  "{0: true, 1:\n2: true}",
		want: []tokDesc{
			objStart,
			k(num(0)), raw("true"),
			k(num(1)),
			k(num(2)), raw("true"),
			objEnd,
		},
	},
	{
		name: "newline_missing_quotes_reset_key",
		src: `{"a
"b": "c"
}`,
		want: []tokDesc{
			objStart,
			k(raw(`"a`)),
			k(str("b")), str("c"),
			objEnd,
		},
	},
	{
		name: "invalid_array",
		src:  `[{"foo", ["test"]]`,
		want: []tokDesc{
			arrStart, objStart, k(str("foo")),
			arrStart, str("test"), arrEnd,
			arrEnd,
		},
	},
	{
		name: "array_as_obj_key",
		src:  `[{["test"]]`,
		want: []tokDesc{
			arrStart, objStart,
			arrStart, str("test"), arrEnd,
			arrEnd,
		},
	},
	{
		name: "multiple_top_level_objs",
		src:  `{"foo", "bar"}["test"]null`,
		want: []tokDesc{
			objStart, k(str("foo")), str("bar"), objEnd,
			arrStart, str("test"), arrEnd,
			raw("null"),
		},
	},
}

func FuzzScanner(f *testing.F) {
	for _, test := range tests {
		f.Add(test.src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		s := newScanner(strings.NewReader(src))
		for s.Next() {
			_ = s.Item()
		}
		if err := s.Err(); err != nil {
			t.Errorf("%v", err)
		}
	})
}

func TestScanner(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lit := func(it scanItem) string {
				if it.from < 0 || it.from > int64(len(test.src)) {
					return fmt.Sprintf("<INVALID FROM: %d>", it.from)
				} else if it.to < 0 || it.to > int64(len(test.src)) {
					return fmt.Sprintf("<INVALID TO: %d>", it.to)
				}
				return test.src[it.from:it.to]
			}

			s := newScanner(strings.NewReader(test.src))
			for i, want := range test.want {
				if !s.Next() {
					t.Fatalf("tok[%d]: got Next() = false, want true", i)
				}
				it := s.Item()
				if it.tok != want.tok {
					t.Fatalf("tok[%d]: got tok kind %s, want %s", i, it.tok, want.tok)
				} else if got := lit(it); got != want.val {
					t.Fatalf("tok[%d]: got lit %s, want %s", i, got, want.val)
				}
			}
			if s.Next() {
				it := s.Item()
				t.Fatalf("got extra token beyond expected tokens: %s (val: %s)", it.tok, lit(it))
			}
			if err := s.Err(); err != nil {
				t.Fatalf("got err %v, want nil", err)
			}
		})
	}
}

type scannerTest struct {
	name string
	src  string
	want []tokDesc
}

type tokDesc struct {
	tok      token
	val      string
	isMapKey bool
}

func str(s string) tokDesc {
	return raw(strconv.Quote(s))
}

func num(n int) tokDesc {
	return tokDesc{tok: literal, val: strconv.Itoa(n)}
}

func raw(s string) tokDesc {
	return tokDesc{tok: literal, val: s}
}

func k(t tokDesc) tokDesc {
	t.isMapKey = true
	return t
}

var (
	objStart = tokDesc{tok: objectBegin, val: "{"}
	objEnd   = tokDesc{tok: objectEnd, val: "}"}
	arrStart = tokDesc{tok: arrayBegin, val: "["}
	arrEnd   = tokDesc{tok: arrayEnd, val: "]"}
)
