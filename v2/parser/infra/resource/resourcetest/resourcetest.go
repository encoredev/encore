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

	"encr.dev/v2/parser/infra/resource"
	pkginfo2 "encr.dev/v2/parser/internal/pkginfo"
	schema2 "encr.dev/v2/parser/internal/schema"
	"encr.dev/v2/parser/internal/testutil"
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
		importList = append(importList, parser.RequiredImports...)
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

			l := pkginfo2.New(tc.Context)
			schemaParser := schema2.NewParser(tc.Context, l)

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
			got := parser.Run(pass)

			if len(test.WantErrs) == 0 {
				c.Assert(got, qt.HasLen, 1)

				// Check for equality, ignoring all the AST nodes and pkginfo types.
				opts := append([]cmp.Option{
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&schema2.FuncDecl{}, &schema2.TypeDecl{}, &pkginfo2.File{}, &pkginfo2.Package{}, token.Pos(0)),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(schema2.StructField{}, schema2.NamedType{}),
					cmp.Comparer(func(a, b *pkginfo2.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
				}, cmpOpts...)
				c.Assert(got[0], qt.CmpEquals(opts...), test.Want)
			}
		})
	}
}
