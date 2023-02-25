package rpc

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser2/apis/directive"
	"encr.dev/parser2/apis/rpc/apipaths"
	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/testutil"
)

func TestParseRPC(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		def      string
		want     *RPC
		wantErrs []string
	}
	tests := []testCase{
		{
			name: "basic",
			def: `
// Foo does things.
//encore:api public
func Foo(ctx context.Context) error {}
`,
			want: &RPC{
				Name:   "Foo",
				Doc:    "Foo does things.\n",
				Access: Public,
				Path: apipaths.Path{Segments: []apipaths.Segment{
					{Type: apipaths.Literal, Value: "foo.Foo", ValueType: schema.String},
				}},
				HTTPMethods: []string{"GET", "POST"},
			},
		},
		{
			name: "with_fields",
			def: `
//encore:api private path=/foo method=PUT tag:some-tag
func Foo(ctx context.Context) error {}
`,
			want: &RPC{
				Name:   "Foo",
				Doc:    "",
				Access: Private,
				Path: apipaths.Path{Segments: []apipaths.Segment{
					{Type: apipaths.Literal, Value: "foo", ValueType: schema.String},
				}},
				HTTPMethods: []string{"PUT"},
				Tags:        selector.Set{{Type: selector.Tag, Value: "some-tag"}},
			},
		},
		{
			name: "with_string_param",
			def: `
//encore:api auth path=/:key
func Foo(ctx context.Context, key string) error {}
`,
			want: &RPC{
				Name:   "Foo",
				Doc:    "",
				Access: Auth,
				Path: apipaths.Path{Segments: []apipaths.Segment{
					{Type: apipaths.Param, Value: "key", ValueType: schema.String},
				}},
				HTTPMethods: []string{"GET", "POST"},
			},
		},
		{
			name: "with_int_param",
			def: `
//encore:api auth path=/:key
func Foo(ctx context.Context, key int) error {}
`,
			want: &RPC{
				Name:   "Foo",
				Doc:    "",
				Access: Auth,
				Path: apipaths.Path{Segments: []apipaths.Segment{
					{Type: apipaths.Param, Value: "key", ValueType: schema.Int},
				}},
				HTTPMethods: []string{"GET", "POST"},
			},
		},
		{
			name:    "raw",
			imports: []string{"net/http"},
			def: `
//encore:api public raw path=/raw
func Raw(w http.ResponseWriter, req *http.Request) {}
`,
			want: &RPC{
				Name:   "Raw",
				Doc:    "",
				Access: Public,
				Raw:    true,
				Path: apipaths.Path{Segments: []apipaths.Segment{
					{Type: apipaths.Literal, Value: "raw", ValueType: schema.String},
				}},
				HTTPMethods: []string{"*"},
			},
		},
	}

	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test testCase) *txtar.Archive {
		importList := append([]string{"context"}, test.imports...)
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
			dirs, doc, err := directive.Parse(fd.Doc)
			c.Assert(err, qt.IsNil)
			dir, ok := dirs.Get("api")
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
