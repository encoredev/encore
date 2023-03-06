package codegentest

import (
	"go/ast"
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
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

func Run(t *testing.T, tests []Case, cmpOpts ...cmp.Option) {
	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test Case) *txtar.Archive {
		return testutil.ParseTxtar(`
-- go.mod --
module example.com
require encore.dev v1.13.4
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

			if len(test.WantErrs) == 0 {
				c.Assert(got, qt.HasLen, 1)

				// Check for equality, ignoring all the AST nodes and pkginfo types.
				opts := append([]cmp.Option{
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&schema.FuncDecl{}, &schema.TypeDecl{}, &pkginfo.File{}, &pkginfo.Package{}, token.Pos(0)),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(schema.StructField{}, schema.NamedType{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
				}, cmpOpts...)
				c.Assert(got[0], qt.CmpEquals(opts...), test.Want)
			}
		})
	}
}
