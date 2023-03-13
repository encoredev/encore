package resourcetest

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/testutil"
	"encr.dev/v2/parser/resource"
)

type Case[R resource.Resource] struct {
	Name     string
	Imports  []string
	Code     string
	Want     R
	WantErrs []string
}

func Run[R resource.Resource](t *testing.T, parser *resource.Parser, tests []Case[R], cmpOpts ...cmp.Option) {
	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test Case[R]) *txtar.Archive {
		importList := []string{"context"}
		for _, imp := range parser.InterestingImports {
			importList = append(importList, imp.String())
		}
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

			l := pkginfo.New(tc.Context)
			schemaParser := schema.NewParser(tc.Context, l)

			if len(test.WantErrs) > 0 {
				defer tc.DeferExpectError(test.WantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			pass := &resource.Pass{
				Context:      tc.Context,
				SchemaParser: schemaParser,
				Pkg:          pkg,
			}
			parser.Run(pass)
			got := pass.Resources()

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
