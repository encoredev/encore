package directive

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseDirective(t *testing.T) {
	testcases := []struct {
		desc     string
		line     string
		expected Directive
		wantErr  string
	}{
		{
			desc: "api public endpoint",
			line: "api public",
			expected: Directive{
				Name:    "api",
				Options: []string{"public"},
			},
		},
		{
			desc: "custom method",
			line: "api public method=FOO",
			expected: Directive{
				Name:    "api",
				Options: []string{"public"},
				Fields:  []Field{{Key: "method", Value: "FOO"}},
			},
		},
		{
			desc: "multiple methods",
			line: "api public raw method=GET,POST",
			expected: Directive{
				Name:    "api",
				Options: []string{"public", "raw"},
				Fields:  []Field{{Key: "method", Value: "GET,POST"}},
			},
		},
		{
			desc: "api with tags",
			line: "api public tag:foo method=FOO raw tag:bar",
			expected: Directive{
				Name:    "api",
				Options: []string{"public", "raw"},
				Fields:  []Field{{Key: "method", Value: "FOO"}},
				Tags:    []string{"tag:foo", "tag:bar"},
			},
		},
		{
			desc:    "api with duplicate tag",
			line:    "api public tag:foo tag:foo",
			wantErr: "invalid encore:api directive: duplicate tag \"tag:foo\"",
		},
		{
			desc: "middleware",
			line: "middleware target=tag:foo,tag:bar",
			expected: Directive{
				Name:   "middleware",
				Fields: []Field{{Key: "target", Value: "tag:foo,tag:bar"}},
			},
		},
		{
			desc:    "middleware empty target",
			line:    "middleware target=",
			wantErr: `invalid encore:middleware directive: field "target" has no value`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			c := qt.New(t)
			dir, err := parseOne(tc.line, 0)
			if tc.wantErr != "" {
				c.Assert(err, qt.ErrorMatches, tc.wantErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(dir, qt.CmpEquals(cmpopts.EquateEmpty()), tc.expected)
			}
		})
	}
}
