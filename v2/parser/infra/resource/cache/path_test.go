package cache

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseKeyspacePath(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		Path string
		Want []Segment
		Err  string
	}{
		{"foo", []Segment{{Literal, "foo"}}, ""},
		{"foo/", nil, "path cannot contain trailing slash"},
		{"/foo", nil, "path must not begin with '/'"},
		{"foo/bar", []Segment{{Literal, "foo"}, {Literal, "bar"}}, ""},
		{"foo//bar", nil, "path cannot contain double slash"},
		{":foo/*bar", []Segment{{Param, "foo"}, {Literal, "*bar"}}, ""},
		{":foo/*", []Segment{{Param, "foo"}, {Literal, "*"}}, ""},
		{":foo/*/bar", []Segment{{Param, "foo"}, {Literal, "*"}, {Literal, "bar"}}, ""},
		{":foo/*;", []Segment{{Param, "foo"}, {Literal, "*;"}}, ""},
		{":;", nil, "path parameter must be a valid Go identifier name"},
		{":foo/*;", []Segment{{Param, "foo"}, {Literal, "*;"}}, ""},
		{"\u0000", []Segment{{Literal, "\u0000"}}, ""},
		{"foo?bar=baz", []Segment{{Literal, "foo?bar=baz"}}, ""},
	}

	for _, test := range tests {
		c.Run(test.Path, func(c *qt.C) {
			p, err := ParseKeyspacePath(0, test.Path)
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
		{"POST", "foo", ``},
		{"POST", "foo", `.+ foo and foo: duplicate path`},
		{"GET", "foo", ``},
		{"*", "foo", `.+ foo and foo: duplicate path`},
		{"*", "bar", ``},
		{"PATCH", "bar", `.+ bar and bar: duplicate path`},
		{"POST", "foo/bar", ``},
		{"POST", "foo/:bar", `.+ foo/:bar and foo/bar: cannot combine parameter ':bar' with path 'foo/bar'`},
		{"POST", "moo/:bar", ``},
		{"POST", "moo/:baz", `.+ moo/:baz and moo/:bar: duplicate path`},
		{"POST", "moo/:baz/test", ``},
	}

	set := &KeyspacePathSet{}

	for _, test := range paths {
		p, err := ParseKeyspacePath(0, test.Path)
		c.Assert(err, qt.IsNil)
		err = set.Add(test.Method, p)
		if test.Err != "" {
			c.Assert(err, qt.ErrorMatches, test.Err, qt.Commentf("%s %s", test.Method, test.Path))
		} else {
			c.Assert(err, qt.IsNil, qt.Commentf("%s %s", test.Method, test.Path))
		}
	}
}
