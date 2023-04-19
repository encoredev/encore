package pkginfo_test

import (
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestNames(t *testing.T) {
	t.Run("empty_pkg", func(t *testing.T) {
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

		pkg := l.MustLoadPkg(token.NoPos, "example.com/foo")
		c.Assert(pkg.Names().PkgDecls, qt.HasLen, 0)
	})

	t.Run("external_module", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- go.mod --
module example.com
require rsc.io/quote v1.5.2
-- foo/foo.go --
package foo
import "rsc.io/quote"
var _ = quote.Hello()
`)
		tc := testutil.NewContext(c, false, a)
		defer tc.FailTestOnBailout()
		tc.GoModTidy()

		l := pkginfo.New(tc.Context)
		pkg := l.MustLoadPkg(token.NoPos, "rsc.io/quote")

		c.Assert(pkg.Names().PkgDecls, qt.CmpEquals(
			cmpopts.IgnoreFields(pkginfo.PkgDeclInfo{}, "File", "Func", "Pos", "GenDecl", "Spec", "Doc"),
		), map[string]*pkginfo.PkgDeclInfo{
			"Glass": {Name: "Glass", Type: token.FUNC},
			"Go":    {Name: "Go", Type: token.FUNC},
			"Hello": {Name: "Hello", Type: token.FUNC},
			"Opt":   {Name: "Opt", Type: token.FUNC},
		})

		gotPath, ok := pkg.Files[0].Names().ResolvePkgPath(token.NoPos, "sampler")
		c.Assert(ok, qt.IsTrue)
		c.Assert(gotPath, qt.Equals, paths.Pkg("rsc.io/sampler"))
	})

	t.Run("external_module_major_version", func(t *testing.T) {
		c := qt.New(t)
		a := parse(`
-- go.mod --
module example.com
require rsc.io/quote v1.5.3-0.20180710144737-5d9f230bcfba
-- foo/foo.go --
package foo
import "rsc.io/quote"
var _ = quote.HelloV3()
`)
		tc := testutil.NewContext(c, false, a)
		defer tc.FailTestOnBailout()
		tc.GoModTidy()

		l := pkginfo.New(tc.Context)
		pkg := l.MustLoadPkg(token.NoPos, "rsc.io/quote/v3")

		c.Assert(pkg.Names().PkgDecls, qt.CmpEquals(
			cmpopts.IgnoreFields(pkginfo.PkgDeclInfo{}, "File", "Func", "Pos", "GenDecl", "Spec", "Doc"),
		), map[string]*pkginfo.PkgDeclInfo{
			"HelloV3": {Name: "HelloV3", Type: token.FUNC},
			"GlassV3": {Name: "GlassV3", Type: token.FUNC},
			"GoV3":    {Name: "GoV3", Type: token.FUNC},
			"OptV3":   {Name: "OptV3", Type: token.FUNC},
		})

		gotPath, ok := pkg.Files[0].Names().ResolvePkgPath(token.NoPos, "sampler")
		c.Assert(ok, qt.IsTrue)
		c.Assert(gotPath, qt.Equals, paths.Pkg("rsc.io/sampler"))
	})
}
