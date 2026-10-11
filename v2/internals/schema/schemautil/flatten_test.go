package schemautil_test

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/internals/testutil"
)

func TestFlattenStruct(t *testing.T) {
	tests := []struct {
		name string
		code string // declares type T
		want []string
	}{
		{
			name: "no_embedding_is_unchanged",
			code: "type T struct { A string; b int; C string `json:\"c\"` }",
			want: []string{"A", "b", "C"},
		},
		{
			name: "embedded_struct_is_flattened",
			code: "type T struct { Inner; Outer string }\ntype Inner struct { X string }",
			want: []string{"Inner.X", "Outer"},
		},
		{
			name: "nested_embedding",
			code: "type T struct { A }\ntype A struct { B; X string }\ntype B struct { Y string }",
			want: []string{"A.B.Y", "A.X"},
		},
		{
			name: "embedded_pointer_is_flattened_and_marked",
			code: "type T struct { *Inner }\ntype Inner struct { X string }",
			want: []string{"*Inner.X"},
		},
		{
			name: "json_named_embed_is_a_field",
			code: "type T struct { Inner `json:\"inner\"` }\ntype Inner struct { X string }",
			want: []string{"Inner"},
		},
		{
			name: "json_dash_embed_is_omitted",
			code: "type T struct { Inner `json:\"-\"`; A string }\ntype Inner struct { X string }",
			want: []string{"A"},
		},
		{
			name: "named_json_dash_field_is_kept",
			code: "type T struct { Inner }\ntype Inner struct { X string `json:\"-\" query:\"x\"` }",
			want: []string{"Inner.X"},
		},
		{
			name: "embedded_non_struct_is_a_field",
			code: "type T struct { ID }\ntype ID string",
			want: []string{"ID"},
		},
		{
			name: "unexported_embedded_struct_promotes_exported_fields",
			code: "type T struct { inner }\ntype inner struct { X string; y string }",
			want: []string{"inner.X"},
		},
		{
			name: "unexported_embedded_pointer_is_flattened",
			code: "type T struct { *inner; A string }\ntype inner struct { X string }",
			want: []string{"*inner.X", "A"},
		},
		{
			name: "shallower_field_wins",
			code: "type T struct { Inner; X string }\ntype Inner struct { X string; Y string }",
			want: []string{"Inner.Y", "X"},
		},
		{
			name: "same_depth_conflict_drops_both",
			code: "type T struct { A; B }\ntype A struct { X string }\ntype B struct { X string }",
			want: nil,
		},
		{
			name: "same_depth_conflict_tagged_wins",
			code: "type T struct { A; B }\ntype A struct { X string }\ntype B struct { Y string `json:\"X\"` }",
			want: []string{"B.Y"},
		},
		{
			name: "generic_embed_is_concretized",
			code: "type T struct { Box[int] }\ntype Box[V any] struct { Value V }",
			want: []string{"Box.Value:int"},
		},
		{
			name: "embedding_cycle_terminates",
			code: "type T struct { *T; A string }",
			want: []string{"A"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			got := flattenForTest(c, test.code)
			c.Assert(got, qt.DeepEquals, test.want)
		})
	}
}

// flattenForTest returns the flattened fields of type T in code as "Path.Name",
// marking embedded pointers with "*".
func flattenForTest(c *qt.C, code string) []string {
	c.Helper()
	ar := testutil.ParseTxtar(`
-- go.mod --
module example.com
require encore.dev v1.52.0
-- code.go --
package foo

` + code + `
`)
	tc := testutil.NewContext(c, false, ar)
	tc.FailTestOnErrors()
	defer tc.FailTestOnBailout()
	tc.GoModDownload()

	l := pkginfo.New(tc.Context)
	p := schema.NewParser(tc.Context, l)
	pkg := l.MustLoadPkg(token.NoPos, "example.com")
	decl := p.ParseTypeDecl(pkg.Names().PkgDecls["T"])
	st := decl.Type.(schema.StructType)

	var out []string
	for _, f := range schemautil.FlattenStruct(tc.Errs, st) {
		var parts []string
		for _, p := range f.Path {
			name, _ := schemautil.EmbeddedFieldName(p)
			if _, ok := p.Type.(schema.PointerType); ok {
				name = "*" + name
			}
			parts = append(parts, name)
		}
		parts = append(parts, f.Field.Name.MustGet())
		s := strings.Join(parts, ".")
		if b, ok := f.Field.Type.(schema.BuiltinType); ok && strings.Contains(code, "[V any]") {
			s += ":" + b.String()
		}
		out = append(out, s)
	}
	return out
}

func TestFlatField_GoPath(t *testing.T) {
	c := qt.New(t)
	exported := schema.StructField{Type: schema.NamedType{DeclInfo: &pkginfo.PkgDeclInfo{Name: "Inner"}}}
	unexported := schema.StructField{Type: schema.NamedType{DeclInfo: &pkginfo.PkgDeclInfo{Name: "inner"}}}
	f := schemautil.FlatField{
		Field: schema.StructField{Type: schema.BuiltinType{Kind: schema.String, AST: ast.NewIdent("string")}},
		Path:  []schema.StructField{exported, unexported},
	}
	// Unexported embedded fields are not named; Go promotes through them.
	c.Assert(f.GoPath(), qt.DeepEquals, []string{"Inner"})
	c.Assert(f.IsPromoted(), qt.IsTrue)
	c.Assert(f.ThroughPointer(), qt.IsFalse)
}
