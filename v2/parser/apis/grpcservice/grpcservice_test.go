package grpcservice_test

import (
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/go-internal/txtar"
	"google.golang.org/protobuf/reflect/protoreflect"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/protoparse"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/grpcservice"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/apis/servicestruct"
)

func TestParseEndpoints(t *testing.T) {
	type testCase struct {
		name     string
		imports  []string
		def      string
		want     []*api.GRPCEndpoint
		wantErrs []string
	}
	tests := []testCase{
		{
			name: "with_grpc_pkgpath",
			def: `
//encore:service grpc=path.to.grpc.Service
type Foo struct {}

func (f *Foo) Bar(ctx context.Context) error { return nil }
-- proto/path/to/grpc.proto --
syntax = "proto3";
package path.to.grpc;

service Service {
	rpc Bar (BarRequest) returns (BarResponse);
}
message BarRequest {}
message BarResponse {}
`,
			want: []*api.GRPCEndpoint{
				{
					Name:     "Bar",
					FullName: "path.to.grpc.Service.Bar",
					Path: &resourcepaths.Path{
						Segments: []resourcepaths.Segment{
							{Type: resourcepaths.Literal, Value: "path.to.grpc.Service", ValueType: schema.String},
							{Type: resourcepaths.Literal, Value: "Bar", ValueType: schema.String},
						},
					},
					Decl: &schema.FuncDecl{Name: "Bar"},
				},
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
			protoParser := protoparse.NewParser(tc.Errs, []paths.FS{
				tc.MainModuleDir.Join("proto"),
			})

			if len(test.wantErrs) > 0 {
				defer tc.DeferExpectError(test.wantErrs...)
			} else {
				tc.FailTestOnErrors()
				defer tc.FailTestOnBailout()
			}

			pkg := l.MustLoadPkg(token.NoPos, "example.com")
			f := pkg.Files[0]
			gd := testutil.FindNodes[*ast.GenDecl](f.AST())[1]

			// Parse the directive from the func declaration.
			dir, doc, ok := directive.Parse(tc.Errs, gd.Doc)
			c.Assert(ok, qt.IsTrue)

			pd := servicestruct.ParseData{
				Errs:   tc.Errs,
				Proto:  protoParser,
				Schema: schemaParser,
				File:   f,
				Decl:   gd,
				Dir:    dir,
				Doc:    doc,
			}

			var endpoints []*api.GRPCEndpoint
			if ss := servicestruct.Parse(tc.Ctx, pd); ss != nil {
				if proto, ok := ss.Proto.Get(); ok {
					endpoints = grpcservice.ParseEndpoints(grpcservice.ServiceDesc{
						Errs:   tc.Errs,
						Proto:  proto,
						Schema: schemaParser,
						Pkg:    pd.File.Pkg,
						Decl:   ss.Decl,
					})
				}
			}
			if len(test.wantErrs) == 0 {
				// Check for equality, ignoring all the AST nodes and pkginfo types.
				cmpEqual := qt.CmpEquals(
					cmpopts.IgnoreInterfaces(struct{ protoreflect.MethodDescriptor }{}),
					cmpopts.IgnoreTypes(&schema.FuncDecl{}, &schema.TypeDecl{}, &pkginfo.File{}, &pkginfo.Package{}, token.Pos(0)),
					cmpopts.EquateEmpty(),
					cmpopts.IgnoreUnexported(schema.StructField{}, schema.NamedType{}),
					cmp.Comparer(func(a, b *pkginfo.Package) bool {
						return a.ImportPath == b.ImportPath
					}),
				)
				c.Assert(endpoints, cmpEqual, test.want)
			}
		})
	}
}
