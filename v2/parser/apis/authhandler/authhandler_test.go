package authhandler

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	. "encr.dev/v2/internals/schema/schematest"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser/apis/directive"
)

func TestParseAuthHandler(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		def      string
		want     *AuthHandler
		wantErrs []string
	}

	ctxParam := Param(Named(TypeInfo("Context")))
	uidResult := Param(Builtin(schema.UserID))

	tests := []testCase{
		{
			name: "basic_legacy",
			def: `
//encore:authhandler
func Foo(ctx context.Context, token string) (auth.UID, error) {}
`,
			want: &AuthHandler{
				Decl: &schema.FuncDecl{
					Name: "Foo",
					Type: schema.FuncType{
						Params: []schema.Param{
							ctxParam,
							Param(String()),
						},
						Results: []schema.Param{
							uidResult,
							Param(Error()),
						},
					},
				},
				Param: String(),
			},
		},
		{
			name: "struct_params",
			def: `
type Params struct{}
//encore:authhandler
func Foo(ctx context.Context, p *Params) (auth.UID, error) {}
`,
			want: &AuthHandler{
				Decl: &schema.FuncDecl{
					Name: "Foo",
					Type: schema.FuncType{
						Params: []schema.Param{
							ctxParam,
							Param(Ptr(Named(TypeInfo("Params")))),
						},
						Results: []schema.Param{
							uidResult,
							Param(Error()),
						},
					},
				},
				Param: Ptr(Named(TypeInfo("Params"))),
			},
		},
	}

	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test testCase) *txtar.Archive {
		importList := append([]string{"context", "encore.dev/beta/auth"}, test.imports...)
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
			dir, doc, ok := directive.Parse(tc.Errs, fd.Doc)
			c.Assert(ok, qt.IsTrue)

			pd := ParseData{
				Errs:   tc.Errs,
				Schema: schemaParser,
				File:   f,
				Func:   fd,
				Dir:    dir,
				Doc:    doc,
			}

			got := Parse(pd)

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
