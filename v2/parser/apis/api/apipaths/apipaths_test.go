package apipaths

import (
	"context"
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/errinsrc"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/schema"
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
		{"foo", nil, "API paths must always start with a '/'"},
		{"/foo", []Segment{{Literal, "foo", str, 1, 3}}, ""},
		{"/foo/", nil, "API paths cannot end with a trailing slash ('/')"},
		{"/foo/bar", []Segment{{Literal, "foo", str, 1, 3}, {Literal, "bar", str, 5, 7}}, ""},
		{"/foo//bar", nil, "API paths cannot contain an empty segment, i.e. a double slash ('//')."},
		{"/:foo/*bar", []Segment{{Param, "foo", str, 1, 4}, {Wildcard, "bar", str, 6, 9}}, ""},
		{"/:foo/*", nil, "API path parameters must have a name."},
		{"/:foo/*/bar", nil, "API path parameters must have a name."},
		{"/:foo/*bar/baz", nil, "API path wildcards must be the last segment in the path."},
		{"/:foo/*;", nil, "API path parameters must be valid Go identifiers."},
		{"/:;", nil, "API path parameters must be valid Go identifiers."},
		{"/\u0000", nil, "invalid control character in URL"},
		{"/foo?bar=baz", nil, `API paths must not contain the '?' character.`},
	}

	for _, test := range tests {
		c.Run(test.Path, func(c *qt.C) {
			errs := perr.NewList(context.Background(), token.NewFileSet())
			p, ok := Parse(errs, 0, test.Path)
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
		{"POST", "/foo", `Duplicate API paths found.`},
		{"GET", "/foo", ``},
		{"*", "/foo", `Duplicate API paths found.`},
		{"*", "/bar", ``},
		{"PATCH", "/bar", `Duplicate API paths found.`},
		{"POST", "/foo/bar", ``},
		{"POST", "/foo/:bar", "The path segment `bar` conflicts with the path `/foo/bar`"},
		{"POST", "/moo/:bar", ``},
		{"POST", "/moo/:baz", `Duplicate API paths found.`},
		{"POST", "/moo/:baz/test", ``},
		{"POST", "/moo/:baa/*wild", "The wildcard `*wild` conflicts with the path `/moo/:baz/test`."},
		{"GET", "/moo/:baa/*wild", ``},
		{"POST", "/test/*wild", ``},
		{"POST", "/test/*card", "The wildcard `*card` conflicts with the path `/test/*wild`."},
	}

	set := &Set{}

	for _, test := range paths {
		errs := perr.NewList(context.Background(), token.NewFileSet())
		p, ok := Parse(errs, 0, test.Path)
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
