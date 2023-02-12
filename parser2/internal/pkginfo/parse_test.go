package pkginfo

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/perr"
)

func TestLoader(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
// Package doc
package pkgname // main file
`)
		dir := writeTxtar(c, a)

		ctx := parsectx.NewForTest(c, false)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, rootDir: dir}
			pkg, ok := m.ParseRelPath("foo")
			c.Assert(ok, qt.Equals, true)
			c.Check(pkg.Name, qt.Equals, "pkgname")
			c.Check(pkg.Doc, qt.Equals, "Package doc\n")

			c.Assert(pkg.Files, qt.HasLen, 1)
			f := pkg.Files[0]
			c.Check(f.Name, qt.Equals, "foo.go")
			c.Check(f.Pkg, qt.Equals, pkg)
			c.Check(f.TestFile, qt.IsFalse)
			c.Check(string(f.Contents), qt.Equals, string(a.Files[0].Data))
		})
	})

	t.Run("with_tests", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo // test file
`)
		dir := writeTxtar(c, a)

		ctx := parsectx.NewForTest(c, true)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, rootDir: dir}
			pkg, ok := m.ParseRelPath("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 2)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)
			c.Check(string(pkg.Files[0].Contents), qt.Equals, string(a.Files[0].Data))

			c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
			c.Check(pkg.Files[1].TestFile, qt.IsTrue)
			c.Check(string(pkg.Files[1].Contents), qt.Equals, string(a.Files[1].Data))
		})
	})

	t.Run("with_external_tests", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo_test // external test file
`)
		dir := writeTxtar(c, a)

		ctx := parsectx.NewForTest(c, true)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, rootDir: dir}
			pkg, ok := m.ParseRelPath("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 2)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)
			c.Check(string(pkg.Files[0].Contents), qt.Equals, string(a.Files[0].Data))

			c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
			c.Check(pkg.Files[1].TestFile, qt.IsTrue)
			c.Check(string(pkg.Files[1].Contents), qt.Equals, string(a.Files[1].Data))
		})
	})

	t.Run("with_tests_ignored", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo // test file
`)
		dir := writeTxtar(c, a)

		ctx := parsectx.NewForTest(c, false)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, rootDir: dir}
			pkg, ok := m.ParseRelPath("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 1)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)
			c.Check(string(pkg.Files[0].Contents), qt.Equals, string(a.Files[0].Data))
		})
	})

	t.Run("with_parse_failure", func(t *testing.T) {
		c := qt.New(t)

		dir := writeTxtar(c, parse(`
-- foo/foo.go --
asdf
`))

		ctx := parsectx.NewForTest(c, false)
		l := New(ctx)
		m := &Module{l: l, rootDir: dir}

		defer func() {
			l, caught := perr.CatchBailout(recover())
			if !caught {
				c.Fatal("expected bailout")
			}
			out := l.FormatErrors()
			c.Assert(out, qt.Matches, `.*/foo/foo\.go:1:1: expected 'package', found asdf\n`)
		}()
		m.ParseRelPath("foo")
	})
}

func writeTxtar(c *qt.C, a *txtar.Archive) (dir string) {
	c.Helper()
	dir = c.TempDir()
	err := txtar.Write(a, dir)
	c.Assert(err, qt.IsNil)
	return dir
}

func parse(in string) *txtar.Archive {
	return txtar.Parse([]byte(in))
}
