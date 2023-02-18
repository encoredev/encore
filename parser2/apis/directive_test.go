package apis

import (
	"go/token"
	"testing"

	"encr.dev/parser2/apis/apipaths"
	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/schema"

	qt "github.com/frankban/quicktest"
)

func TestParseDirective(t *testing.T) {
	const staticPos = token.Pos(0)

	testcases := []struct {
		desc        string
		line        string
		expected    directive
		expectedErr string
	}{
		{
			desc:        "api public endpoint",
			line:        "api public",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Public,
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api private endpoint",
			line:        "api private",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Private,
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "custom method",
			line:        "api public method=FOO",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Public,
				Raw:      false,
				TokenPos: staticPos,
				Method:   []string{"FOO"},
			},
		},
		{
			desc:        "multiple methods",
			line:        "api public raw method=GET,POST",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Public,
				Raw:      true,
				TokenPos: staticPos,
				Method:   []string{"GET", "POST"},
			},
		},
		{
			desc:        "api with params, trailing =",
			line:        "api public raw path=/bar",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Public,
				Raw:      true,
				TokenPos: staticPos,
				Path: &apipaths.Path{Pos: staticPos, Segments: []apipaths.Segment{
					{Type: apipaths.Literal, Value: "bar", ValueType: schema.String},
				}},
			},
		},
		{
			desc:        "api with tags",
			line:        "api public tag:foo tag:bar",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   Public,
				TokenPos: staticPos,
				Tags: selector.Set{
					{Type: selector.Tag, Value: "foo"},
					{Type: selector.Tag, Value: "bar"},
				},
			},
		},
		{
			desc:        "api with duplicate tag",
			line:        "api public tag:foo tag:foo",
			expectedErr: `duplicate tag "tag:foo"`,
		},
		{
			desc:        "api with invalid selector",
			line:        "api public tag:foo.bar",
			expectedErr: `invalid tag format "tag:foo.bar": invalid value`,
		},
		{
			desc: "middleware",
			line: "middleware target=tag:foo,tag:bar",
			expected: &middlewareDirective{
				Target: selector.Set{
					{Type: selector.Tag, Value: "foo"},
					{Type: selector.Tag, Value: "bar"},
				},
			},
		},
		{
			desc: "global middleware",
			line: "middleware global target=tag:foo",
			expected: &middlewareDirective{
				Global: true,
				Target: selector.Set{
					{Type: selector.Tag, Value: "foo"},
				},
			},
		},
		{
			desc: "middleware target all",
			line: "middleware target=all",
			expected: &middlewareDirective{
				Target: selector.Set{
					{Type: selector.All, Value: ""},
				},
			},
		},
		{
			desc:        "middleware duplicate tag",
			line:        "middleware target=tag:foo,tag:foo",
			expectedErr: `duplicate tag "tag:foo"`,
		},
		{
			desc:        "middleware missing target",
			line:        "middleware",
			expectedErr: `middleware must specify at least one target tag`,
		},
		{
			desc:        "middleware empty target",
			line:        "middleware target=",
			expectedErr: `empty directive field: "target="`,
		},
		{
			desc:        "middleware empty target",
			line:        "middleware target",
			expectedErr: `middleware field "target" must be in the form 'target=value'`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			c := qt.New(t)
			dir, err := parseDirective(staticPos, tc.line)
			if tc.expectedErr != "" || err != nil {
				c.Assert(err, qt.ErrorMatches, tc.expectedErr)
				return
			}
			c.Assert(dir, qt.DeepEquals, tc.expected)
		})
	}
}
