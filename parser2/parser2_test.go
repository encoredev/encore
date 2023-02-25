package parser2

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/exp/slices"

	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/testutil"
)

func TestParsePackages(t *testing.T) {
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
	parser := NewParserFromCtx(tc.Context)

	var got []*pkginfo.Package
	parser.collectPackages(func(pkg *pkginfo.Package) {
		got = append(got, pkg)
	})
	c.Assert(got, qt.HasLen, 2)

	// Sort the packages by import path since collectPackages processes
	// packages concurrently.
	slices.SortFunc(got, func(a, b *pkginfo.Package) bool {
		return a.ImportPath < b.ImportPath
	})
	c.Assert(got[0].ImportPath, qt.Equals, paths.Pkg("example.com/foo"))
	c.Assert(got[1].ImportPath, qt.Equals, paths.Pkg("example.com/foo/bar"))
}
