package schema

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	"github.com/fatih/structtag"
	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestParser_ParseType(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		typ      string
		want     Type
		wantErrs []string
	}

	file := fileForPkg("foo", "example.com")
	tests := []testCase{
		{
			name: "builtin_int",
			typ:  "int",
			want: BuiltinType{Kind: Int, AST: ast.NewIdent("int")},
		},
		{
			name:     "unsupported_builtin",
			typ:      "uintptr",
			wantErrs: []string{".*unsupported type: uintptr"},
		},
		{
			name: "pointer",
			typ:  "*string",
			want: PointerType{Elem: BuiltinType{Kind: String, AST: ast.NewIdent("string")}},
		},
		{
			name: "decl",
			typ:  "foo\n\ntype foo int",
			want: namedTypeWithDecl(&TypeDecl{
				Name: "foo",
				Type: BuiltinType{Kind: Int, AST: ast.NewIdent("int")},
				File: file,
			}),
		},
		{
			name: "decl_with_type_params",
			typ:  "foo[int]\n\ntype foo[T any] T",
			want: namedTypeWithDecl(
				(func() *TypeDecl {
					d := new(TypeDecl)
					*d = TypeDecl{
						Name: "foo",
						File: file,
						Type: TypeParamRefType{
							AST:   ast.NewIdent("T"),
							Index: 0,
							Decl:  d,
						},
						TypeParams: []DeclTypeParam{
							{Name: "T"},
						},
					}
					return d
				})(),
				BuiltinType{Kind: Int, AST: ast.NewIdent("int")},
			),
		},
		{
			name:    "builtin_encore_uuid",
			imports: []string{"encore.dev/types/uuid"},
			typ:     "uuid.UUID",
			want:    BuiltinType{Kind: UUID, AST: ast.NewIdent("uuid.UUID")},
		},
		{
			name:    "encore_option",
			imports: []string{"encore.dev/types/option", "encore.dev/types/uuid"},
			typ:     "option.Option[uuid.UUID]",
			want:    OptionType{AST: ast.NewIdent("option.Option"), Value: BuiltinType{Kind: UUID, AST: ast.NewIdent("uuid.UUID")}},
		},
		{
			name:    "builtin_encore_userid",
			imports: []string{"encore.dev/beta/auth"},
			typ:     "auth.UID",
			want:    BuiltinType{Kind: UserID, AST: ast.NewIdent("auth.UID")},
		},
		{
			name:    "builtin_time",
			imports: []string{"time"},
			typ:     "time.Time",
			want:    BuiltinType{Kind: Time, AST: ast.NewIdent("time.Time")},
		},
		{
			name:    "builtin_json",
			imports: []string{"encoding/json"},
			typ:     "json.RawMessage",
			want:    BuiltinType{Kind: JSON, AST: ast.NewIdent("json.RawMessage")},
		},
		{
			name: "builtin_error",
			typ:  "error",
			want: BuiltinType{Kind: Error, AST: ast.NewIdent("error")},
		},
		{
			name:    "external_stdlib_type",
			imports: []string{"database/sql"},
			typ:     "sql.NullString",
			want: namedTypeWithDecl(&TypeDecl{
				Name: "NullString",
				File: fileForPkg("sql", "database/sql"),
				Type: StructType{
					Fields: []StructField{
						{
							Name: option.Some("String"),
							Type: BuiltinType{Kind: String},
						},
						{
							Name: option.Some("Valid"),
							Type: BuiltinType{Kind: Bool},
						},
					},
				},
			}),
		},
		{
			name: "map",
			typ:  "map[struct{A int}]struct{}",
			want: MapType{
				Key: StructType{Fields: []StructField{
					{Name: option.Some("A"), Type: BuiltinType{Kind: Int}},
				}},
				Value: StructType{},
			},
		},
		{
			name: "slice",
			typ:  "[]bool",
			want: ListType{
				Elem: BuiltinType{Kind: Bool, AST: ast.NewIdent("bool")},
				Len:  -1,
			},
		},
		{
			name: "array",
			typ:  "[3]bool",
			want: ListType{
				Elem: BuiltinType{Kind: Bool, AST: ast.NewIdent("bool")},
				Len:  3,
			},
		},
		{
			name: "array_unknown_const",
			typ:  "[someConst]bool\nconst someConst = 3",
			want: ListType{
				Elem: BuiltinType{Kind: Bool, AST: ast.NewIdent("bool")},
				Len:  -1, // unknown
			},
		},
		{
			name: "multi_generic",
			typ:  "foo[int, string]\n\ntype foo[T any, U any] struct{A T; B U}",
			want: namedTypeWithDecl(
				(func() *TypeDecl {
					d := new(TypeDecl)
					*d = TypeDecl{
						Name: "foo",
						File: file,
						Type: StructType{
							Fields: []StructField{
								{
									Name: option.Some("A"),
									Type: TypeParamRefType{
										AST:   ast.NewIdent("A"),
										Index: 0,
										Decl:  d,
									},
								},
								{
									Name: option.Some("B"),
									Type: TypeParamRefType{
										AST:   ast.NewIdent("B"),
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
				BuiltinType{Kind: Int, AST: ast.NewIdent("int")},
				BuiltinType{Kind: String, AST: ast.NewIdent("string")},
			),
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
require encore.dev v1.52.0
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
				var options []cmp.Option
				options = []cmp.Option{
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&pkginfo.File{}),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(StructField{}, structtag.Tags{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
					cmp.Comparer(func(a, b NamedType) bool {
						return cmp.Equal(a.Decl(), b.Decl(), options...) && cmp.Equal(a.TypeArgs, b.TypeArgs, options...)
					}),
				}

				c.Assert(got.String(), qt.CmpEquals(options...), test.want.String())
			}
		})
	}
}

func TestParser_ParseFuncDecl(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		decl     string
		want     *FuncDecl
		wantErrs []string
	}
	file := fileForPkg("foo", "example.com")
	tests := []testCase{
		{
			name: "simple",
			decl: "func x() {}",
			want: &FuncDecl{
				Name: "x",
				File: fileForPkg("foo", "example.com"),
				Recv: option.None[*Receiver](),
				Type: FuncType{
					Params:  nil,
					Results: nil,
				},
			},
		},
		{
			name: "recv",
			decl: "type Foo[A, B any] struct{}\nfunc (f *Foo[A, B]) x() {}",
			want: &FuncDecl{
				Name: "x",
				File: file,
				Recv: option.Some((func() *Receiver {
					FooDecl := &TypeDecl{
						File:       file,
						Name:       "Foo",
						Type:       StructType{},
						TypeParams: []DeclTypeParam{{Name: "A"}, {Name: "B"}},
					}

					return &Receiver{
						Name: option.Some("f"),
						Decl: FooDecl,
						Type: PointerType{
							Elem: namedTypeWithDecl(FooDecl,
								TypeParamRefType{
									Decl:  FooDecl,
									Index: 0,
								},
								TypeParamRefType{
									Decl:  FooDecl,
									Index: 1,
								},
							),
						},
					}
				})()),
				Type: FuncType{
					Params:  nil,
					Results: nil,
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
require encore.dev v1.52.0
-- code.go --
package foo
` + imports + `

` + test.decl + `
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

			// Find the first func decl.
			var fd *ast.FuncDecl
			f := pkg.Files[0]
			for _, decl := range f.AST().Decls {
				if f, ok := decl.(*ast.FuncDecl); ok {
					fd = f
					break
				}
			}
			c.Assert(fd, qt.IsNotNil)

			got, ok := p.ParseFuncDecl(f, fd)
			c.Assert(ok, qt.IsTrue)

			if len(test.wantErrs) == 0 {
				// Check for equality, ignoring all the AST nodes and pkginfo types.
				var options []cmp.Option
				options = []cmp.Option{
					cmpopts.IgnoreInterfaces(struct{ ast.Node }{}),
					cmpopts.IgnoreTypes(&pkginfo.File{}),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(StructField{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
					cmp.Comparer(func(a, b NamedType) bool {
						return cmp.Equal(a.Decl(), b.Decl(), options...) && cmp.Equal(a.TypeArgs, b.TypeArgs, options...)
					}),
				}

				c.Assert(got.String(), qt.CmpEquals(options...), test.want.String())
			}
		})
	}
}

func namedTypeWithDecl(decl *TypeDecl, typeArgs ...Type) NamedType {
	lazy := &lazyDecl{}
	lazy.once.Do(func() {}) // mark the lazy.once as used
	lazy.decl = decl

	return NamedType{
		AST:      nil,
		TypeArgs: typeArgs,
		// Approximate the PkgDeclInfo
		DeclInfo: &pkginfo.PkgDeclInfo{
			Name: decl.Name,
			File: decl.File,
			Pos:  token.NoPos,
			Type: token.TYPE,
			Spec: decl.AST,
		},
		decl: lazy,
	}
}

func fileForPkg(pkgName string, pkgPath paths.Pkg) *pkginfo.File {
	return &pkginfo.File{Pkg: &pkginfo.Package{
		Name:       pkgName,
		ImportPath: pkgPath,
	}}
}
