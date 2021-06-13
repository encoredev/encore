package paths

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParse(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		Path string
		Want []Segment
		Err  string
	}{
		{"/foo", []Segment{{Literal, "foo"}}, ""},
		{"/foo/", nil, "path cannot contain trailing slash"},
		{"/foo/bar", []Segment{{Literal, "foo"}, {Literal, "bar"}}, ""},
		{"/foo//bar", nil, "path cannot contain double slash"},
		{"/:foo/*bar", []Segment{{Param, "foo"}, {Wildcard, "bar"}}, ""},
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
		Path string
		Err  string
	}{
		{"/foo", ``},
		{"/foo", `.+ /foo and /foo: duplicate path`},
		{"/foo/bar", ``},
		{"/foo/:bar", `.+ /foo/:bar and /foo/bar: cannot combine parameter ':bar' with path '/foo/bar'`},
		{"/moo/:bar", ``},
		{"/moo/:baz", `.+ /moo/:baz and /moo/:bar: duplicate path`},
		{"/moo/:baz/test", ``},
		{"/moo/:baa/*wild", `.+ /moo/:baa/\*wild and /moo/:baz/test: cannot combine wildcard '\*wild' with path '/moo/:baz/test'`},
		{"/test/*wild", ``},
		{"/test/*card", `.+ /test/\*card and /test/\*wild: cannot combine wildcard '\*card' with path '/test/\*wild'`},
	}

	set := &Set{}

	for _, test := range paths {
		p, err := Parse(0, test.Path)
		c.Assert(err, qt.IsNil)
		err = set.Add(p)
		if test.Err != "" {
			c.Assert(err, qt.ErrorMatches, test.Err)
		} else {
			c.Assert(err, qt.IsNil)
		}
	}
}
