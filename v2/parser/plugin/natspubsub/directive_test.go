package natspubsub

import (
	"go/ast"
	"testing"

	"encr.dev/v2/parser/apis/directive"
)

func TestParsePubSub_Valid(t *testing.T) {
	d := &directive.Directive{
		Name:    "pubsub",
		Options: []directive.Field{{Value: "orders.created"}},
	}
	decl := handlerDecl(&ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("Context")}, &ast.StarExpr{X: ast.NewIdent("OrderCreated")})

	if err := parsePubSub(d, decl); err != nil {
		t.Fatalf("parsePubSub returned error: %v", err)
	}
}

func TestParsePubSub_InvalidSignature(t *testing.T) {
	d := &directive.Directive{
		Name:    "pubsub",
		Options: []directive.Field{{Value: "orders.created"}},
	}
	decl := handlerDecl(ast.NewIdent("int"), ast.NewIdent("OrderCreated"))

	if err := parsePubSub(d, decl); err == nil {
		t.Fatal("expected parsePubSub to reject invalid signature")
	}
}

func TestValidateNATSSubject(t *testing.T) {
	cases := []struct {
		subject string
		ok      bool
	}{
		{subject: "orders.created", ok: true},
		{subject: "orders.*", ok: true},
		{subject: "orders.>", ok: true},
		{subject: "orders..created", ok: false},
		{subject: "orders.>.created", ok: false},
		{subject: "orders.crea*ted", ok: false},
		{subject: "", ok: false},
	}

	for _, tc := range cases {
		err := validateNATSSubject(tc.subject)
		if tc.ok && err != nil {
			t.Errorf("expected subject %q to be valid, got error: %v", tc.subject, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("expected subject %q to be invalid", tc.subject)
		}
	}
}

func handlerDecl(firstParam, secondParam ast.Expr) *ast.FuncDecl {
	return &ast.FuncDecl{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: []*ast.Field{{Type: firstParam}, {Type: secondParam}}},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent("error")}}},
		},
	}
}
