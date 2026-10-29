package scan

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestProcessModule(t *testing.T) {
	t.Run("finds every package", func(t *testing.T) {
		c := qt.New(t)
		a := testutil.ParseTxtar(`
-- go.mod --
module example.com
-- foo/foo.go --
package foo
-- foo/bar/bar.go --
package bar
`)
		tc := testutil.NewContext(c, false, a)
		tc.FailTestOnErrors()
		loader := pkginfo.New(tc.Context)

		var got testutil.PackageList
		ProcessModule(tc.Errs, loader, tc.MainModuleDir, got.Collector())
		c.Assert(got, qt.HasLen, 2)

		// Sort the packages by import path since collectPackages processes
		// packages concurrently.
		slices.SortFunc(got, func(a, b *pkginfo.Package) int {
			return cmp.Compare(a.ImportPath, b.ImportPath)
		})
		c.Assert(got[0].ImportPath, qt.Equals, paths.Pkg("example.com/foo"))
		c.Assert(got[1].ImportPath, qt.Equals, paths.Pkg("example.com/foo/bar"))
	})

	t.Run("a syntax error is reported when the file is used", func(t *testing.T) {
		c := qt.New(t)
		a := testutil.ParseTxtar(`
-- go.mod --
module example.com
-- foo/foo.go --
package foo

var x =
`)
		tc := testutil.NewContext(c, false, a)
		loader := pkginfo.New(tc.Context)

		var got testutil.PackageList
		ProcessModule(tc.Errs, loader, tc.MainModuleDir, got.Collector())
		c.Assert(got, qt.HasLen, 1)
		c.Assert(tc.Errs.Len(), qt.Equals, 0, qt.Commentf("errors before the file was used: %s", tc.Errs.FormatErrors()))

		defer tc.DeferExpectError(`expected`)
		got[0].Files[0].AST()
	})
}

func BenchmarkProcessModule(b *testing.B) {
	var archive strings.Builder
	archive.WriteString("-- go.mod --\nmodule example.com\n")
	for p := range 100 {
		for f := range 10 {
			fmt.Fprintf(&archive, "-- p%d/f%d.go --\npackage p%d\n\n", p, f, p)
			for fn := range 20 {
				fmt.Fprintf(&archive, "func F%d_%d(a, b int) int {\n\tif a > b {\n\t\treturn a - b\n\t}\n\treturn a + b\n}\n\n", f, fn)
			}
		}
	}
	c := qt.New(b)
	a := testutil.ParseTxtar(archive.String())

	for b.Loop() {
		b.StopTimer()
		tc := testutil.NewContext(c, false, a)
		loader := pkginfo.New(tc.Context)
		b.StartTimer()
		// The parser reads every file's full AST as it processes each package.
		ProcessModule(tc.Errs, loader, tc.MainModuleDir, func(pkg *pkginfo.Package) {
			for _, f := range pkg.Files {
				f.AST()
			}
		})
	}
}
