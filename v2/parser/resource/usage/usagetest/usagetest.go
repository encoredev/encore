package usagetest

import (
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource/usage"
)

type Case struct {
	Name     string
	Imports  []string
	Code     string
	Want     []usage.Usage
	WantErrs []string
}

func Run(t *testing.T, standardImports []string, tests []Case, cmpOpts ...cmp.Option) {
	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test Case) *txtar.Archive {
		importList := append([]string{"context"}, standardImports...)
		importList = append(importList, test.Imports...)

		imports := ""
		if len(importList) > 0 {
			imports = "import (\n"
			for _, imp := range importList {
				imports += "\t" + strconv.Quote(imp) + "\n"
			}
			imports += ")\n"
		}

		return testutil.ParseTxtar(`
-- go.mod --
module example.com
require encore.dev v1.52.0
-- code.go --
package foo
` + imports + `

` + test.Code + `
`)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			c := qt.New(t)
			a := testArchive(test)
			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()

			if len(test.WantErrs) > 0 {
				defer tc.DeferExpectError(test.WantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pp := parser.NewParser(tc.Context)
			result := pp.Parse()

			if len(test.WantErrs) == 0 {
				// Check for equality, ignoring all the AST nodes and pkginfo types.
				opts := append([]cmp.Option{
					cmpopts.IgnoreTypes(usage.Base{}),
					cmpopts.EquateEmpty(),
				}, cmpOpts...)
				usages := result.AllUsages()
				c.Assert(usages, qt.CmpEquals(opts...), test.Want)
			}
		})
	}
}
