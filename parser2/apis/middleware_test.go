package apis

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	. "encr.dev/parser2/internal/schema/schematest"
	"encr.dev/parser2/internal/testutil"
)

func TestParseMiddleware(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		def      string
		want     *Middleware
		wantErrs []string
	}

	mwParams := []schema.Param{
		Param(Named(TypeInfo("Request"))),
		Param(Named(TypeInfo("Next"))),
	}
	mwResults := []schema.Param{
		Param(Named(TypeInfo("Response"))),
	}

	tests := []testCase{
		{
			name: "basic",
			def: `
//encore:middleware target=tag:foo
func Foo(req middleware.Request, next middleware.Next) middleware.Response {}
`,
			want: &Middleware{
				Decl: &schema.FuncDecl{
					Name: "Foo",
					Type: schema.FuncType{
						Params:  mwParams,
						Results: mwResults,
					},
				},
				Target: selector.Set{{
					Type:  selector.Tag,
					Value: "foo",
				}},
			},
		},
	}

	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test testCase) *txtar.Archive {
		importList := append([]string{"context", "encore.dev/middleware"}, test.imports...)
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

` + test.def + `
`)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			a := testArchive(test)
			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()

			l := pkginfo.New(tc.Context)
			schemaParser := schema.NewParser(tc.Context, l)
			p := NewParser(tc.Context, schemaParser)

			if len(test.wantErrs) > 0 {
				defer tc.DeferExpectError(test.wantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			f := pkg.Files[0]
			fd := testutil.FindNodes[*ast.FuncDecl](f.AST())[0]

			// Parse the directive from the func declaration.
			dir, doc := p.parseDirectives(fd.Doc)
			mwDir, ok := dir.(*middlewareDirective)
			c.Assert(ok, qt.IsTrue)

			got := p.parseMiddleware(f, fd, mwDir, doc)
			if len(test.wantErrs) == 0 {
				// Check for equality, ignoring all the AST nodes and pkginfo types.
				cmpEqual := qt.CmpEquals(
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&schema.FuncDecl{}, &schema.TypeDecl{}, &pkginfo.File{}, &pkginfo.Package{}, token.Pos(0)),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(schema.StructField{}, schema.NamedType{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
				)
				c.Assert(got, cmpEqual, test.want)
			}
		})
	}
}
