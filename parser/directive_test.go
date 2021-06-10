package parser

import (
	"go/token"
	"testing"

	"encr.dev/parser/est"

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
				Params:   map[string]string{},
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
				Params:   map[string]string{},
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api raw endpoint",
			line:        "api public raw",
			expectedErr: "",
			expected: &rpcDirective{
				Access:   est.Public,
				Params:   map[string]string{},
				Raw:      true,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api with params",
			line:        "api public foo=bar",
			expectedErr: "",
			expected: &rpcDirective{
				Access: est.Public,
				Params: map[string]string{
					"foo": "bar",
				},
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api with multiple params",
			line:        "api public foo=bar baz=qux",
			expectedErr: "",
			expected: &rpcDirective{
				Access: est.Public,
				Params: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
				Raw:      false,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api with params, trailing =",
			line:        "api public raw foo=bar==",
			expectedErr: "",
			expected: &rpcDirective{
				Access: est.Public,
				Params: map[string]string{
					"foo": "bar==",
				},
				Raw:      true,
				TokenPos: staticPos,
			},
		},
		{
			desc:        "api with params (duplicate)",
			line:        "api public foo=bar foo=baz",
			expectedErr: "cannot declare duplicate parameter fields",
			expected:    nil,
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

func TestValidateRPCPath(t *testing.T) {
	testcases := []struct {
		desc        string
		path        string
		expectedErr string
	}{
		{
			desc:        "",
			path:        "",
			expectedErr: "path must be non-empty if specified",
		},
		{
			desc:        "simple",
			path:        "/test",
			expectedErr: "",
		},
		{
			desc:        "multi-segment",
			path:        "/test/foo",
			expectedErr: "",
		},
		{
			desc:        "multi-segment with param",
			path:        "/test/:foo",
			expectedErr: "",
		},
		{
			desc:        "multi-segment with multi params",
			path:        "/test/:foo/:bar",
			expectedErr: "",
		},
		{
			desc:        "invalid param placement",
			path:        "/test/fo:oo/:bar",
			expectedErr: "identifiers ':' must be at the start of a path segment",
		},
		{
			desc:        "invalid param placement",
			path:        "/test/:fooo:",
			expectedErr: "path segments can only contain a single ':' identifier",
		},
		{
			desc:        "single wildcard",
			path:        "/test/:foo/*name",
			expectedErr: "",
		},
		{
			desc:        "multi wildcard",
			path:        "/test/*foo/*name",
			expectedErr: "path must only contain a single wildcard operator",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			c := qt.New(t)
			err := validateRPCPath(tc.path)
			if tc.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, tc.expectedErr)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}
