package resourcetest

import (
	"go/ast"
	"go/token"
	"os"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Case[R resource.Resource] struct {
	Name     string
	Imports  []string
	Code     string
	Want     R
	WantErrs []string
}

func Run[R resource.Resource](t *testing.T, parser *resourceparser.Parser, tests []Case[R], cmpOpts ...cmp.Option) {
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

			l := pkginfo.New(tc.Context)
			schemaParser := schema.NewParser(tc.Context, l)

			if len(test.WantErrs) > 0 {
				defer tc.DeferExpectError(test.WantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			pass := &resourceparser.Pass{
				Context:      tc.Context,
				SchemaParser: schemaParser,
				Pkg:          pkg,
			}
			parser.Run(pass)
			got := pass.Resources()

			if len(test.WantErrs) == 0 {
				c.Assert(got, qt.HasLen, 1)

				lookupMap := map[string]string{
					"APPROOT": tc.MainModuleDir.ToIO(),
				}

				// Check for equality, ignoring all the AST nodes and pkginfo types.
				opts := append([]cmp.Option{
					cmpopts.IgnoreInterfaces(struct{ ast.Expr }{}),
					cmpopts.IgnoreTypes(&schema.FuncDecl{}, &schema.TypeDecl{}, &pkginfo.File{}, &pkginfo.Package{}, token.Pos(0)),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(schema.StructField{}, schema.NamedType{}),
					cmp.AllowUnexported(option.Option[*pkginfo.File]{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),

					cmp.Comparer(func(a, b paths.FS) bool {
						return rewrite(a, lookupMap) == rewrite(b, lookupMap)
					}),
					cmp.Comparer(func(a, b *pkginfo.PkgDeclInfo) bool {
						// HACK(andre) We only check the subset of information that
						// the test helpers actually set. We should be more careful.
						return a.Name == b.Name
					}),
				}, cmpOpts...)
				c.Assert(got[0], qt.CmpEquals(opts...), test.Want)
			}
		})
	}
}

func rewrite[T ~string](val T, lookup map[string]string) T {
	return T(os.Expand(string(val), func(key string) string {
		return lookup[key]
	}))
}
