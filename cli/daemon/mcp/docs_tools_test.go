package mcp

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBuildFacetFilterGroups(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name string
		in   []string
		want [][]string
	}{
		{
			name: "no filters",
			in:   nil,
			want: nil,
		},
		{
			name: "single lang includes lang:all",
			in:   []string{"lang:go"},
			want: [][]string{{"lang:go", "lang:all"}},
		},
		{
			name: "multiple langs are ORed together",
			in:   []string{"lang:go", "lang:ts"},
			want: [][]string{{"lang:go", "lang:ts", "lang:all"}},
		},
		{
			name: "explicit lang:all is not duplicated",
			in:   []string{"lang:all", "lang:go"},
			want: [][]string{{"lang:all", "lang:go"}},
		},
		{
			name: "duplicate langs are deduped",
			in:   []string{"lang:go", "lang:go"},
			want: [][]string{{"lang:go", "lang:all"}},
		},
		{
			name: "non-lang filters are ANDed as-is",
			in:   []string{"section:docs", "type:page"},
			want: [][]string{{"section:docs"}, {"type:page"}},
		},
		{
			name: "mixed filters AND the non-lang ones with the lang group",
			in:   []string{"section:docs", "lang:ts"},
			want: [][]string{{"section:docs"}, {"lang:ts", "lang:all"}},
		},
	}

	for _, test := range tests {
		c.Run(test.name, func(c *qt.C) {
			c.Assert(buildFacetFilterGroups(test.in), qt.DeepEquals, test.want)
		})
	}
}
