package apipaths

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/v2/internal/schema"
)

func TestParseURL(t *testing.T) {
	c := qt.New(t)

	str := schema.String
	tests := []struct {
		Path string
		Want []Segment
		Err  string
	}{
		{"foo", nil, "path must begin with '/'"},
		{"/foo", []Segment{{Literal, "foo", str}}, ""},
		{"/foo/", nil, "path cannot contain trailing slash"},
		{"/foo/bar", []Segment{{Literal, "foo", str}, {Literal, "bar", str}}, ""},
		{"/foo//bar", nil, "path cannot contain double slash"},
		{"/:foo/*bar", []Segment{{Param, "foo", str}, {Wildcard, "bar", str}}, ""},
		{"/:foo/*", nil, "wildcard parameter must have a name"},
		{"/:foo/*/bar", nil, "wildcard parameter must have a name"},
		{"/:foo/*bar/baz", nil, "wildcard parameter must be the last path segment"},
		{"/:foo/*;", nil, "wildcard parameter must be a valid Go identifier name"},
		{"/:;", nil, "path parameter must be a valid Go identifier name"},
		{"/\u0000", nil, "invalid path: .+ invalid control character in URL"},
		{"/foo?bar=baz", nil, `path cannot contain '\?'`},
	}

	for _, test := range tests {
		c.Run(test.Path, func(c *qt.C) {
			p, err := Parse(0, test.Path)
			if err != nil {
				c.Assert(err, qt.ErrorMatches, test.Err)
			} else if test.Err != "" {
				c.Fatalf("expected err %q, got nil", test.Err)
			} else {
				c.Assert(p.Segments, qt.DeepEquals, test.Want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	c := qt.New(t)

	paths := []struct {
		Method string
		Path   string
		Err    string
	}{
		{"POST", "/foo", ``},
		{"POST", "/foo", `.+ /foo and /foo: duplicate path`},
		{"GET", "/foo", ``},
		{"*", "/foo", `.+ /foo and /foo: duplicate path`},
		{"*", "/bar", ``},
		{"PATCH", "/bar", `.+ /bar and /bar: duplicate path`},
		{"POST", "/foo/bar", ``},
		{"POST", "/foo/:bar", `.+ /foo/:bar and /foo/bar: cannot combine parameter ':bar' with path '/foo/bar'`},
		{"POST", "/moo/:bar", ``},
		{"POST", "/moo/:baz", `.+ /moo/:baz and /moo/:bar: duplicate path`},
		{"POST", "/moo/:baz/test", ``},
		{"POST", "/moo/:baa/*wild", `.+ /moo/:baa/\*wild and /moo/:baz/test: cannot combine wildcard '\*wild' with path '/moo/:baz/test'`},
		{"GET", "/moo/:baa/*wild", ``},
		{"POST", "/test/*wild", ``},
		{"POST", "/test/*card", `.+ /test/\*card and /test/\*wild: cannot combine wildcard '\*card' with path '/test/\*wild'`},
	}

	set := &Set{}

	for _, test := range paths {
		p, err := Parse(0, test.Path)
		c.Assert(err, qt.IsNil)
		err = set.Add(test.Method, p)
		if test.Err != "" {
			c.Assert(err, qt.ErrorMatches, test.Err, qt.Commentf("%s %s", test.Method, test.Path))
		} else {
			c.Assert(err, qt.IsNil, qt.Commentf("%s %s", test.Method, test.Path))
		}
	}
}
