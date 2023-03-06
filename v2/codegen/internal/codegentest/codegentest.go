package codegentest

import (
	"path/filepath"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/testutil"
	"encr.dev/v2/parser"
)

type Case struct {
	Name     string
	Imports  []string
	Code     string
	Want     map[string]string // file name -> contents
	WantErrs []string
}

func Run(t *testing.T, tests []Case, fn func(*codegen.Generator, *app.Desc)) {
	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test Case) *txtar.Archive {
		imports := ""
		if len(test.Imports) > 0 {
			imports = "import (\n"
			for _, imp := range test.Imports {
				imports += "\t" + strconv.Quote(imp) + "\n"
			}
			imports += ")\n"
		}

		return testutil.ParseTxtar(`
-- go.mod --
module example.com
require encore.dev v1.13.4
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

			p := parser.NewParser(tc.Context)
			parserResult := p.Parse()
			gen := codegen.New(tc.Context)
			appDesc := app.ValidateAndDescribe(tc.Context, parserResult)

			// Run the codegen
			fn(gen, appDesc)

			if len(test.WantErrs) == 0 {
				// Construct the map of generated code.
				overlays := gen.Overlays()
				got := make(map[string]string, len(overlays))
				for _, o := range overlays {
					key, err := filepath.Rel(tc.MainModuleDir.ToIO(), o.Source.ToIO())
					c.Assert(err, qt.IsNil)
					got[key] = string(o.Contents)
				}
				c.Assert(got, qt.DeepEquals, test.Want)
			}
		})
	}
}
