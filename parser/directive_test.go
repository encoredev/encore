package parser

import (
	"go/token"
	"testing"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"

	qt "github.com/frankban/quicktest"
)

func TestParseDirectiveRPC(t *testing.T) {
	const staticPos = token.Pos(0)

	testcases := []struct {
		desc        string
		line        string
		expected    *rpcDirective
		expectedErr string
	}{
		{
			desc:        "api public endpoint",
			line:        "api public",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   est.Public,
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api private endpoint",
			line:        "api private",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   est.Private,
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "custom method",
			line:        "api public method=FOO",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   est.Public,
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
				Access:   est.Public,
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
				Access:   est.Public,
				Raw:      true,
				TokenPos: staticPos,
				Path: &paths.Path{Pos: staticPos, Segments: []paths.Segment{
					{Type: paths.Literal, Value: "bar", ValueType: schema.Builtin_STRING},
				}},
			},
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
			rpcDir, ok := dir.(*rpcDirective)
			c.Assert(ok, qt.IsTrue)
			c.Assert(rpcDir, qt.DeepEquals, tc.expected)
		})
	}
}
