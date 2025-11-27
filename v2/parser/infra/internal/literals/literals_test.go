package literals

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestParseConstantValue(t *testing.T) {
	type testCase struct {
		Expr    string
		Imports []string
		Want    any
		Err     string
	}

	tests := []testCase{
		{
			Expr: "true",
			Want: true,
		},
		{
			Expr: "false",
			Want: false,
		},
		{
			Expr: "\"hello world\"",
			Want: "hello world",
		},
		{
			Expr: "\"hello world\" == \"hello world\"",
			Want: true,
		},
		{
			Expr: "\"hello world\" != \"hello world\"",
			Want: false,
		},
		{
			Imports: []string{"encore.dev/cron"},
			Expr:    "1*cron.Minute",
			Want:    1 * 60,
		},
		{
			Imports: []string{"encore.dev/cron"},
			Expr:    "(4/2)*cron.Minute",
			Want:    2 * 60,
		},
		{
			Imports: []string{"encore.dev/cron"},
			Expr:    "(4-2)*cron.Minute + cron.Hour",
			Want:    2*60 + 3600,
		},
		{
			Expr: "5 / 2",
			Want: 2.5,
		},
		{
			Expr: "(5 - 4) - 1",
			Want: int64(0),
		},
		// Note the "(?s)" allows for "." to match newlines
		// This is needed when running tests with the tag `dev_build` which includes
		// stack traces from the parser in the error message.
		{
			Expr: "2.3 / 0",
			Err:  `(?s).+Cannot divide by zero.*`,
		},
	}

	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test testCase) *txtar.Archive {
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
require encore.dev v1.52.0
-- code.go --
package foo
` + imports + `

const x = ` + test.Expr + `
`)
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test[%d]", i), func(t *testing.T) {
			c := qt.New(t)
			a := testArchive(test)
			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()

			l := pkginfo.New(tc.Context)
			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			decl := pkg.Names().PkgDecls["x"]
			x := decl.Spec.(*ast.ValueSpec).Values[0]

			value := ParseConstant(tc.Errs, decl.File, x)

			if test.Err != "" {
				c.Check(value.Kind(), qt.Equals, constant.Unknown, qt.Commentf("Wanted an Unknown value, got: %s", value.Kind().String()))
				c.Check(tc.Errs.Len(), qt.Not(qt.Equals), 0)
				c.Check(tc.Errs.FormatErrors(), qt.Matches, test.Err)
			} else {
				c.Check(tc.Errs.Len(), qt.Equals, 0)
				c.Check(value.Kind(), qt.Not(qt.Equals), constant.Unknown, qt.Commentf("Result was unknown: %s", tc.Errs.FormatErrors()))

				switch w := test.Want.(type) {
				case bool:
					c.Check(constant.BoolVal(value), qt.Equals, test.Want)
				case string:
					c.Check(constant.StringVal(value), qt.Equals, test.Want)
				case float64:
					got, _ := constant.Float64Val(constant.ToFloat(value))
					c.Check(got, qt.Equals, test.Want)
				case int:
					got, _ := constant.Int64Val(constant.ToInt(value))
					c.Check(got, qt.Equals, int64(w))
				case int64:
					got, _ := constant.Int64Val(constant.ToInt(value))
					c.Check(got, qt.Equals, test.Want)
				}
			}
		})
	}
}
