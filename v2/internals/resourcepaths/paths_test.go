package resourcepaths

import (
	"context"
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/errinsrc"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/schema"
)

func init() {
	errinsrc.ColoursInErrors(false)
}

func TestParseURL(t *testing.T) {
	c := qt.New(t)

	str := schema.String
	tests := []struct {
		Path string
		Want []Segment
		Err  string
	}{
		{"foo", nil, "Paths must always start with a '/'"},
		{"/foo", []Segment{{Literal, "foo", str, 0, 3}}, ""},
		{"/foo/", nil, "Paths cannot end with a trailing slash ('/')"},
		{"/foo/bar", []Segment{{Literal, "foo", str, 0, 3}, {Literal, "bar", str, 4, 7}}, ""},
		{"/foo//bar", nil, "Paths cannot contain an empty segment, i.e. a double slash ('//')."},
		{"/:foo/*bar", []Segment{{Param, "foo", str, 0, 4}, {Wildcard, "bar", str, 5, 9}}, ""},
		{"/:foo/*", nil, "Path parameters must have a name."},
		{"/:foo/*/bar", nil, "Path parameters must have a name."},
		{"/:foo/*bar/baz", nil, "Path wildcards must be the last segment in the path."},
		{"/:foo/*;", nil, "Path parameters must be valid Go identifiers."},
		{"/:;", nil, "Path parameters must be valid Go identifiers."},
		{"/\u0000", nil, "invalid control character in URL"},
		{"/foo?bar=baz", nil, `Paths must not contain the '?' character.`},
	}

	for _, test := range tests {
		c.Run(test.Path, func(c *qt.C) {
			errs := perr.NewList(context.Background(), token.NewFileSet())
			p, ok := Parse(errs, 0, test.Path, Options{
				AllowWildcard: true,
				PrefixSlash:   true,
			})
			if !ok {
				c.Assert(errs.FormatErrors(), qt.Contains, test.Err)
			} else if test.Err != "" {
				c.Fatalf("expected err %q, got nil", test.Err)
			} else {
				c.Assert(p.Segments, qt.DeepEquals, test.Want)
				c.Assert(errs.Len(), qt.Equals, 0)
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
		{"POST", "/foo", `Duplicate Paths found.`},
		{"GET", "/foo", ``},
		{"*", "/foo", `Duplicate Paths found.`},
		{"*", "/bar", ``},
		{"PATCH", "/bar", `Duplicate Paths found.`},
		{"POST", "/foo/bar", ``},
		{"POST", "/foo/:bar", "The path segment `bar` conflicts with the path `/foo/bar`"},
		{"POST", "/moo/:bar", ``},
		{"POST", "/moo/:baz", `Duplicate Paths found.`},
		{"POST", "/moo/:baz/test", ``},
		{"POST", "/moo/:baa/*wild", "The wildcard `*wild` conflicts with the path `/moo/:baz/test`."},
		{"GET", "/moo/:baa/*wild", ``},
		{"POST", "/test/*wild", ``},
		{"POST", "/test/*card", "The wildcard `*card` conflicts with the path `/test/*wild`."},
	}

	set := &Set{}

	for _, test := range paths {
		errs := perr.NewList(context.Background(), token.NewFileSet())
		p, ok := Parse(errs, 0, test.Path, Options{
			AllowWildcard: true,
			PrefixSlash:   true,
		})
		c.Assert(ok, qt.IsTrue)
		ok = set.Add(errs, test.Method, p)
		if test.Err != "" {
			c.Assert(errs.FormatErrors(), qt.Contains, test.Err, qt.Commentf("%s %s", test.Method, test.Path))
			c.Assert(ok, qt.IsFalse)
		} else {
			c.Assert(errs.Len(), qt.Equals, 0, qt.Commentf("%s %s", test.Method, test.Path))
			c.Assert(ok, qt.IsTrue)
		}
	}
}
