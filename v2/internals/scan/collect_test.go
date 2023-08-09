package scan

import (
	"cmp"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestProcessModule(t *testing.T) {
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
}
