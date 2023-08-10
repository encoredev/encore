package pkginfo_test

import (
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestLoader(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
// Package doc
package pkgname // main file
-- go.mod --
module example.com
`)

		tc := testutil.NewContext(c, false, a)
		tc.FailTestOnErrors()
		l := pkginfo.New(tc.Context)

		pkg, ok := l.LoadPkg(token.NoPos, "example.com/foo")
		c.Assert(ok, qt.Equals, true)
		c.Check(pkg.Name, qt.Equals, "pkgname")
		c.Check(pkg.ImportPath, qt.Equals, paths.MustPkgPath("example.com/foo"))
		c.Check(pkg.Doc, qt.Equals, "Package doc\n")

		c.Assert(pkg.Files, qt.HasLen, 1)
		f := pkg.Files[0]
		c.Check(f.Name, qt.Equals, "foo.go")
		c.Check(f.Pkg, qt.Equals, pkg)
		c.Check(f.TestFile, qt.IsFalse)
		c.Check(string(f.Contents()), qt.Equals, string(a.Files[0].Data))
	})

	t.Run("with_tests", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo // test file
-- go.mod --
module example.com
	`)

		tc := testutil.NewContext(c, true, a)
		tc.FailTestOnErrors()
		l := pkginfo.New(tc.Context)

		pkg, ok := l.LoadPkg(token.NoPos, "example.com/foo")
		c.Assert(ok, qt.Equals, true)
		c.Assert(pkg.Files, qt.HasLen, 2)

		c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
		c.Check(pkg.Files[0].TestFile, qt.IsFalse)
		c.Check(string(pkg.Files[0].Contents()), qt.Equals, string(a.Files[0].Data))

		c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
		c.Check(pkg.Files[1].TestFile, qt.IsTrue)
		c.Check(string(pkg.Files[1].Contents()), qt.Equals, string(a.Files[1].Data))
	})

	t.Run("with_external_tests", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo_test // external test file
-- go.mod --
module example.com
	`)

		tc := testutil.NewContext(c, true, a)
		tc.FailTestOnErrors()
		l := pkginfo.New(tc.Context)

		pkg, ok := l.LoadPkg(token.NoPos, "example.com/foo")
		c.Assert(ok, qt.Equals, true)
		c.Assert(pkg.Files, qt.HasLen, 2)

		c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
		c.Check(pkg.Files[0].TestFile, qt.IsFalse)
		c.Check(string(pkg.Files[0].Contents()), qt.Equals, string(a.Files[0].Data))

		c.Check(pkg.Files[1].Name, qt.Equals, "foo_test.go")
		c.Check(pkg.Files[1].TestFile, qt.IsTrue)
		c.Check(string(pkg.Files[1].Contents()), qt.Equals, string(a.Files[1].Data))
	})

	t.Run("with_tests_ignored", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- foo/foo.go --
package foo // main file
-- foo/foo_test.go --
package foo // test file
-- go.mod --
module example.com
	`)

		tc := testutil.NewContext(c, false, a)
		tc.FailTestOnErrors()

		l := pkginfo.New(tc.Context)

		pkg, ok := l.LoadPkg(token.NoPos, "example.com/foo")
		c.Assert(ok, qt.Equals, true)
		c.Assert(pkg.Files, qt.HasLen, 1)

		c.Check(pkg.Files[0].Name, qt.Equals, "foo.go")
		c.Check(pkg.Files[0].TestFile, qt.IsFalse)
		c.Check(string(pkg.Files[0].Contents()), qt.Equals, string(a.Files[0].Data))
	})

	t.Run("with_parse_failure", func(t *testing.T) {
		c := qt.New(t)

		a := parse(`
-- foo/foo.go --
asdf
-- go.mod --
module example.com
	`)

		tc := testutil.NewContext(c, false, a)
		l := pkginfo.New(tc.Context)

		defer tc.DeferExpectError(`expected 'package', found asdf`)

		l.LoadPkg(token.NoPos, "example.com/foo")
	})

	t.Run("external_module", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- go.mod --
module example.com
require rsc.io/hello v1.0.0
`)
		tc := testutil.NewContext(c, false, a)
		l := pkginfo.New(tc.Context)
		defer tc.FailTestOnBailout()
		tc.GoModDownload()
		pkg := l.MustLoadPkg(token.NoPos, "rsc.io/hello")
		c.Assert(pkg.Files, qt.HasLen, 1)
		c.Assert(pkg.Name, qt.Equals, "main")
		c.Assert(pkg.Doc, qt.Equals, "Hello greets the world.\n")

		f := pkg.Files[0]
		c.Check(f.Name, qt.Equals, "hello.go")
		c.Check(f.TestFile, qt.IsFalse)
		c.Check(f.FSPath.ToIO(), qt.Matches, `.*/mod/rsc\.io/hello@v1\.0\.0/hello.go`)
		c.Check(fns.MapKeys(f.Imports), qt.ContentEquals, []paths.Pkg{"fmt", "rsc.io/quote"})
		c.Check(string(pkg.Files[0].Contents()), qt.Contains, "fmt.Println(quote.Hello())")
	})
}

func parse(in string) *txtar.Archive {
	return txtar.Parse([]byte(in))
}
