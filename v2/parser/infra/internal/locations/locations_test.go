package locations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/go/ast/inspector"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want Classification
	}{
		{
			name: "simple_pkg_var",
			expr: "var x = TARGET",
			want: PkgVar{Ident: ast.NewIdent("x"), Name: "x"},
		},
		{
			name: "field_lookup",
			expr: "var x = TARGET.Stdlib",
			want: OtherPkgExpr{},
		},
		{
			name: "method_call",
			expr: "var x = TARGET.Stdlib()",
			want: OtherPkgExpr{},
		},
		{
			name: "method_call_in_func_lit",
			expr: "var x = func() { TARGET.Stdlib() }",
			want: InFunc{Node: &ast.FuncLit{}},
		},
		{
			name: "method_call_in_func_lit_call",
			expr: "var x = func() { TARGET.Stdlib() }()",
			want: InFunc{Node: &ast.FuncLit{}},
		},
		{
			name: "in_func_decl",
			expr: "func foo() { TARGET }",
			want: InFunc{Node: &ast.FuncDecl{}},
		},
		{
			name: "func_arg",
			expr: "var x = foo(blah, TARGET)",
			want: OtherPkgExpr{},
		},
		{
			name: "func_arg_method",
			expr: "var x = foo(TARGET.Stdlib())",
			want: OtherPkgExpr{},
		},
		{
			name: "unary_expr",
			expr: "var x = +TARGET",
			want: OtherPkgExpr{},
		},
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreInterfaces(struct {
			ast.Expr
			ast.Stmt
		}{}),
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			stack := parseStack(c, test.expr)
			got := Classify(stack)
			c.Assert(got, qt.CmpEquals(cmpOpts...), test.want)
		})
	}
}

func parseStack(c *qt.C, expr string) []ast.Node {
	code := "package pkg\n\n" + expr
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, c.Name()+".go", code, parser.ParseComments)
	c.Assert(err, qt.IsNil)

	insp := inspector.New([]*ast.File{file})

	var found []ast.Node
	insp.WithStack([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if id, ok := stack[len(stack)-1].(*ast.Ident); ok && id.Name == "TARGET" {
			found = slices.Clone(stack)
		}
		return true
	})
	c.Assert(found, qt.Not(qt.IsNil))
	return found
}
