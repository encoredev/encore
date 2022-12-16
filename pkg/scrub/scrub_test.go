package scrub

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_groupNodes(t *testing.T) {
	f := func(name string) PathEntry {
		return PathEntry{Kind: ObjectField, FieldName: name, CaseSensitive: false}
	}
	cf := func(name string) PathEntry {
		return PathEntry{Kind: ObjectField, FieldName: name, CaseSensitive: true}
	}
	nf := func(name string, ch ...node) node {
		return node{Kind: ObjectField, FieldName: name, CaseSensitive: false, Children: ch}
	}
	ncf := func(name string, ch ...node) node {
		return node{Kind: ObjectField, FieldName: name, CaseSensitive: true, Children: ch}
	}
	scr := node{Kind: scrubNode}

	tests := []struct {
		name  string
		paths []Path
		want  []node
	}{
		{
			name: "single",
			paths: []Path{
				{f("a")},
			},
			want: []node{
				nf("a", scr),
			},
		},

		{
			name: "two_separate",
			paths: []Path{
				{f("a")},
				{f("b")},
			},
			want: []node{nf("a", scr), nf("b", scr)},
		},

		{
			name: "case_sensitive_match",
			paths: []Path{
				{cf("a")},
				{cf("a"), f("b")},
			},
			want: []node{ncf("a", scr)},
		},

		{
			name: "case_sensitive_mismatch",
			paths: []Path{
				{cf("a")},
				{f("a"), cf("a")},
			},
			want: []node{
				ncf("a", scr),
				nf("a", ncf("a", scr)),
			},
		},

		{
			name: "complex",
			paths: []Path{
				{f("a"), f("b"), f("c")},
				{f("a"), f("d"), cf("e")},
				{f("a"), cf("b"), cf("e")},
			},
			want: []node{
				nf("a",
					nf("b", nf("c", scr)),
					nf("d", ncf("e", scr)),
					ncf("b", ncf("e", scr)),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupNodes(tt.paths)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func Test_scrub(t *testing.T) {
	tests := []struct {
		input   string
		bounds  []Bounds
		replace string
		want    string
	}{
		{
			input:   "aaa bbb ccc",
			bounds:  []Bounds{{0, 3}},
			replace: "X",
			want:    "X bbb ccc",
		},
		{
			input:   "aaa bbb ccc",
			bounds:  []Bounds{{0, 1}, {1, 2}, {2, 3}},
			replace: "X",
			want:    "XXX bbb ccc",
		},
		{
			input:   "aaa bbb ccc",
			bounds:  []Bounds{{0, 1}, {1, 2}, {2, 3}},
			replace: "XX",
			want:    "XXXXXX bbb ccc",
		},
		{
			input:   "aaa",
			bounds:  []Bounds{{0, 3}},
			replace: "XX",
			want:    "XX",
		},
		{
			input:   "a",
			bounds:  []Bounds{{0, 0}},
			replace: "XXX",
			want:    "XXXa",
		},
		{
			input:   "a",
			bounds:  []Bounds{{1, 1}},
			replace: "XXX",
			want:    "aXXX",
		},
		{
			input:   "",
			bounds:  []Bounds{{0, 0}},
			replace: "XXX",
			want:    "XXX",
		},
		{
			input:   "a",
			bounds:  []Bounds{{0, 1}},
			replace: "XXX",
			want:    "XXX",
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if got := scrub([]byte(tt.input), tt.bounds, []byte(tt.replace)); !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("scrub() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		paths      []Path
		wantOutput string
	}{
		{
			name:  "simple_object",
			input: `{"a": "1234"}`,
			paths: []Path{
				[]PathEntry{{Kind: ObjectField, FieldName: `"a"`, CaseSensitive: true}},
			},
			wantOutput: `{"a": "[sensitive]"}`,
		},
		{
			name:  "array_nesting",
			input: `[[{"a": "1234"}], [{"b": "1234"}], [[[{"a":"1234"}, {"a": "1234"}]]]`,
			paths: []Path{
				[]PathEntry{{Kind: ObjectField, FieldName: `"a"`, CaseSensitive: true}},
			},
			wantOutput: `[[{"a": "[sensitive]"}], [{"b": "1234"}], [[[{"a":"[sensitive]"}, {"a": "[sensitive]"}]]]`,
		},
		{
			name:  "case_sensitivity",
			input: `[[{"a": "1234"}], [{"A": "1234"}], [[[{"aa":"1234"}, ["aA": "1234"]]]]`,
			paths: []Path{
				[]PathEntry{{Kind: ObjectField, FieldName: `"a"`, CaseSensitive: true}},
			},
			wantOutput: `[[{"a": "[sensitive]"}], [{"A": "1234"}], [[[{"aa":"1234"}, ["aA": "1234"]]]]`,
		},
		{
			name:  "object_nesting_1",
			input: `{"a": "1234", "b": {"a": "1234"}`,
			paths: []Path{
				[]PathEntry{{Kind: ObjectField, FieldName: `"a"`, CaseSensitive: true}},
			},
			wantOutput: `{"a": "[sensitive]", "b": {"a": "1234"}`,
		},
		{
			name:  "object_nesting_2",
			input: `{"a": "1234", "b": {"a": "1234"}`,
			paths: []Path{
				[]PathEntry{
					{Kind: ObjectField, FieldName: `"b"`, CaseSensitive: true},
					{Kind: ObjectField, FieldName: `"a"`, CaseSensitive: true},
				},
			},
			wantOutput: `{"a": "1234", "b": {"a": "[sensitive]"}`,
		},
		{
			name:  "missing_map_values",
			input: "{0:\n 1: 123}",
			paths: []Path{
				[]PathEntry{
					{Kind: ObjectField, FieldName: "1", CaseSensitive: true},
				},
			},
			wantOutput: "{0:\n 1: \"[sensitive]\"}",
		},
		{
			name:  "map_values_multiline",
			input: "{0:\n 1}",
			paths: []Path{
				[]PathEntry{
					{Kind: ObjectField, FieldName: "0", CaseSensitive: true},
				},
			},
			wantOutput: "{0:\n \"[sensitive]\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			if out := JSON(input, tt.paths, []byte(`"[sensitive]"`)); !bytes.Equal(out, []byte(tt.wantOutput)) {
				t.Errorf("scrub() = %s, want %s", out, tt.wantOutput)
			}
		})
	}
}
