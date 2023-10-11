package pkginfo

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/paths"
)

func Test_findModule(t *testing.T) {
	c := qt.New(t)
	deps := []paths.Mod{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"encore.dev",
	}
	slices.Sort(deps)

	yes := func(pkg paths.Pkg, mod paths.Mod) {
		got, ok := findModule(deps, pkg)
		c.Assert(ok, qt.IsTrue, qt.Commentf("pkg=%q", pkg))
		c.Assert(got, qt.Equals, mod, qt.Commentf("pkg=%q", pkg))
	}
	no := func(pkg paths.Pkg, mod paths.Mod) {
		_, ok := findModule(deps, pkg)
		c.Assert(ok, qt.IsFalse, qt.Commentf("pkg=%q", pkg))
	}

	yes("foo", "foo")
	yes("foo/qux", "foo")
	yes("foo/barbar", "foo")

	yes("foo/bar", "foo/bar")
	yes("foo/bar/boo", "foo/bar")
	yes("foo/bar/baz", "foo/bar/baz")
	yes("foo/bar/baz/boo", "foo/bar/baz")
	yes("foo/bar/baz/boo", "foo/bar/baz")
	yes("encore.dev", "encore.dev")
	yes("encore.dev/foo", "encore.dev")
	yes("encore.dev/foo/bar", "encore.dev")

	no("fo", "")
	no("foono", "")
	no("encore", "")
	no("encore.devno", "")
	no("x", "")
}
