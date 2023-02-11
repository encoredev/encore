package pkginfo

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/perr"
)

func TestLoader(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		c := qt.New(t)
		fooData := "// Package doc\npackage pkgname"
		fs := fstest.MapFS{"foo/foo.go": {
			Data: []byte(fooData),
		}}

		ctx := parsectx.NewForTest(c, false)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, fsys: fs}
			pkg, ok := m.ParseDir("foo")
			c.Assert(ok, qt.Equals, true)
			c.Check(pkg.Name, qt.Equals, "pkgname")
			c.Check(pkg.Doc, qt.Equals, "Package doc\n")

			c.Assert(pkg.Files, qt.HasLen, 1)
			f := pkg.Files[0]
			c.Check(f.Name, qt.Equals, "foo.go")
			c.Check(string(f.Contents), qt.Equals, fooData)
			c.Check(f.Pkg, qt.Equals, pkg)
			c.Check(f.TestFile, qt.IsFalse)
		})
	})

	t.Run("with_tests", func(t *testing.T) {
		c := qt.New(t)
		fooData := "package foo\n\n// main file"
		testData := "package foo\n\n// test file"
		fs := fstest.MapFS{
			"foo/foo.go":      {Data: []byte(fooData)},
			"foo/foo_test.go": {Data: []byte(testData)},
		}

		ctx := parsectx.NewForTest(c, true)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, fsys: fs}
			pkg, ok := m.ParseDir("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 2)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(string(pkg.Files[0].Contents), qt.Equals, fooData)
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)

			c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
			c.Check(string(pkg.Files[1].Contents), qt.Equals, testData)
			c.Check(pkg.Files[1].TestFile, qt.IsTrue)
		})
	})

	t.Run("with_external_tests", func(t *testing.T) {
		c := qt.New(t)
		fooData := "package foo\n\n// main file"
		testData := "package foo_test\n\n// external test file"
		fs := fstest.MapFS{
			"foo/foo.go":      {Data: []byte(fooData)},
			"foo/foo_test.go": {Data: []byte(testData)},
		}

		ctx := parsectx.NewForTest(c, true)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, fsys: fs}
			pkg, ok := m.ParseDir("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 2)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(string(pkg.Files[0].Contents), qt.Equals, fooData)
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)

			c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
			c.Check(string(pkg.Files[1].Contents), qt.Equals, testData)
			c.Check(pkg.Files[1].TestFile, qt.IsTrue)
		})
	})

	t.Run("with_tests_ignored", func(t *testing.T) {
		c := qt.New(t)
		fooData := "package foo\n\n// main file"
		testData := "package foo\n\n// test file"
		fs := fstest.MapFS{
			"foo/foo.go":      {Data: []byte(fooData)},
			"foo/foo_test.go": {Data: []byte(testData)},
		}

		ctx := parsectx.NewForTest(c, false)
		parsectx.FailTestOnErrors(ctx, func() {
			l := New(ctx)

			m := &Module{l: l, fsys: fs}
			pkg, ok := m.ParseDir("foo")
			c.Assert(ok, qt.Equals, true)
			c.Assert(pkg.Files, qt.HasLen, 1)

			c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
			c.Check(string(pkg.Files[0].Contents), qt.Equals, fooData)
			c.Check(pkg.Files[0].TestFile, qt.IsFalse)
		})
	})

	t.Run("with_parse_failure", func(t *testing.T) {
		c := qt.New(t)
		fooData := "asdf"
		fs := fstest.MapFS{
			"foo/foo.go": {Data: []byte(fooData)},
		}

		ctx := parsectx.NewForTest(c, false)
		l := New(ctx)
		m := &Module{l: l, fsys: fs}

		defer func() {
			l, caught := perr.CatchBailout(recover())
			if !caught {
				c.Fatal("expected bailout")
			}
			out := l.FormatErrors()
			c.Assert(out, qt.Equals, "foo/foo.go:1:1: expected 'package', found asdf\n")
		}()
		m.ParseDir("foo")
	})
}
