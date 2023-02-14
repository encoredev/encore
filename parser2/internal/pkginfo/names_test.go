package pkginfo

import (
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/exp/maps"

	"encr.dev/parser2/internal/testutil"
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
		l := New(tc.Context)

		pkg := l.MustLoadPkg(token.NoPos, "example.com/foo")
		c.Assert(pkg.Names().PkgDecls, qt.HasLen, 0)
		c.Assert(pkg.Files[0].Names().nameToPath, qt.HasLen, 0)
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

		l := New(tc.Context)
		pkg := l.MustLoadPkg(token.NoPos, "rsc.io/quote")

		c.Assert(pkg.Names().PkgDecls, qt.CmpEquals(
			cmpopts.IgnoreFields(PkgDeclInfo{}, "File", "Pos", "Func", "Spec", "Doc"),
		), map[string]*PkgDeclInfo{
			"Glass": {Name: "Glass", Type: token.FUNC},
			"Go":    {Name: "Go", Type: token.FUNC},
			"Hello": {Name: "Hello", Type: token.FUNC},
			"Opt":   {Name: "Opt", Type: token.FUNC},
		})

		c.Assert(maps.Keys(pkg.Files[0].Names().nameToPath), qt.CmpEquals(), []string{"sampler"})
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

		l := New(tc.Context)
		pkg := l.MustLoadPkg(token.NoPos, "rsc.io/quote/v3")

		c.Assert(pkg.Names().PkgDecls, qt.CmpEquals(
			cmpopts.IgnoreFields(PkgDeclInfo{}, "File", "Pos", "Func", "Spec", "Doc"),
		), map[string]*PkgDeclInfo{
			"HelloV3": {Name: "HelloV3", Type: token.FUNC},
			"GlassV3": {Name: "GlassV3", Type: token.FUNC},
			"GoV3":    {Name: "GoV3", Type: token.FUNC},
			"OptV3":   {Name: "OptV3", Type: token.FUNC},
		})

		c.Assert(maps.Keys(pkg.Files[0].Names().nameToPath), qt.CmpEquals(), []string{"sampler"})
	})
}
