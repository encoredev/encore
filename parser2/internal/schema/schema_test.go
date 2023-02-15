package schema

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/testutil"
)

func TestParser_ParseType(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		typ      string
		want     Type
		wantErrs []string
	}
	tests := []testCase{
		{
			name: "builtin_int",
			typ:  "int",
			want: BuiltinType{Kind: Int},
		},
		{
			name:     "unsupported_builtin",
			typ:      "uintptr",
			wantErrs: []string{".*unsupported type: uintptr"},
		},
		{
			name: "pointer",
			typ:  "*string",
			want: PointerType{Elem: BuiltinType{Kind: String}},
		},
		{
			name: "decl",
			typ:  "foo\n\ntype foo int",
			want: NamedType{Decl: &Decl{
				Name: "foo",
				Type: BuiltinType{Kind: Int},
				Pkg:  &pkginfo.Package{ImportPath: "example.com"},
			}},
		},
		{
			name: "decl_with_type_params",
			typ:  "foo[int]\n\ntype foo[T any] T",
			want: NamedType{
				Decl: (func() *Decl {
					d := new(Decl)
					*d = Decl{
						Name: "foo",
						Pkg:  &pkginfo.Package{ImportPath: "example.com"},
						Type: TypeParamRefType{
							Index: 0,
							Decl:  d,
						},
						TypeParams: []DeclTypeParam{
							{Name: "T"},
						},
					}
					return d
				})(),
				TypeArgs: []Type{BuiltinType{Kind: Int}},
			},
		},
		{
			name:    "builtin_encore_uuid",
			imports: []string{"encore.dev/types/uuid"},
			typ:     "uuid.UUID",
			want:    BuiltinType{Kind: UUID},
		},
		{
			name:    "builtin_encore_userid",
			imports: []string{"encore.dev/beta/auth"},
			typ:     "auth.UID",
			want:    BuiltinType{Kind: UserID},
		},
		{
			name:    "builtin_time",
			imports: []string{"time"},
			typ:     "time.Time",
			want:    BuiltinType{Kind: Time},
		},
		{
			name:    "builtin_json",
			imports: []string{"encoding/json"},
			typ:     "json.RawMessage",
			want:    BuiltinType{Kind: JSON},
		},
		{
			name:    "external_stdlib_type",
			imports: []string{"database/sql"},
			typ:     "sql.NullString",
			want: NamedType{Decl: &Decl{
				Name: "NullString",
				Pkg:  &pkginfo.Package{ImportPath: "database/sql"},
				Type: StructType{
					Fields: []*StructField{
						{
							Name: "String",
							Type: BuiltinType{Kind: String},
						},
						{
							Name: "Valid",
							Type: BuiltinType{Kind: Bool},
						},
					},
				},
			}},
		},
		{
			name: "map",
			typ:  "map[struct{A int}]struct{}",
			want: MapType{
				Key:   StructType{Fields: []*StructField{{Name: "A", Type: BuiltinType{Kind: Int}}}},
				Value: StructType{},
			},
		},
		{
			name: "slice",
			typ:  "[]bool",
			want: ListType{
				Elem: BuiltinType{Kind: Bool},
				Len:  -1,
			},
		},
		{
			name: "array",
			typ:  "[3]bool",
			want: ListType{
				Elem: BuiltinType{Kind: Bool},
				Len:  3,
			},
		},
		{
			name: "array_unknown_const",
			typ:  "[someConst]bool\nconst someConst = 3",
			want: ListType{
				Elem: BuiltinType{Kind: Bool},
				Len:  -1, // unknown
			},
		},
		{
			name: "multi_generic",
			typ:  "foo[int, string]\n\ntype foo[T any, U any] struct{A T; B U}",
			want: NamedType{
				Decl: (func() *Decl {
					d := new(Decl)
					*d = Decl{
						Name: "foo",
						Pkg:  &pkginfo.Package{ImportPath: "example.com"},
						Type: StructType{
							Fields: []*StructField{
								{
									Name: "A",
									Type: TypeParamRefType{
										Index: 0,
										Decl:  d,
									},
								},
								{
									Name: "B",
									Type: TypeParamRefType{
										Index: 1,
										Decl:  d,
									},
								},
							},
						},
						TypeParams: []DeclTypeParam{
							{Name: "T"},
							{Name: "U"},
						},
					}
					return d
				})(),
				TypeArgs: []Type{
					BuiltinType{Kind: Int},
					BuiltinType{Kind: String},
				},
			},
		},
	}

	// testArchive renders the txtar archive to use for a given test.
	testArchive := func(test testCase) *txtar.Archive {
		imports := ""
		if len(test.imports) > 0 {
			imports = "import (\n"
			for _, imp := range test.imports {
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

var x ` + test.typ + `
`)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			a := testArchive(test)
			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()

			l := pkginfo.New(tc.Context)
			p := NewParser(tc.Context, l)

			if len(test.wantErrs) > 0 {
				defer tc.DeferExpectError(test.wantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			f := pkg.Files[0]
			typeExpr := pkg.Names().PkgDecls["x"].Spec.(*ast.ValueSpec).Type
			got := p.ParseType(f, typeExpr)

			if len(test.wantErrs) == 0 {
				// Check for equality, ignoring all the AST nodes and pkginfo types.
				cmpEqual := qt.CmpEquals(
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&pkginfo.File{}),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(StructField{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
				)
				c.Assert(got, cmpEqual, test.want)
			}
		})
	}
}
