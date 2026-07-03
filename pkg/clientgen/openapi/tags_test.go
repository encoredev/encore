package openapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/getkin/kin-openapi/openapi3"

	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func TestOpenAPITagAppliesSchemaMetadata(t *testing.T) {
	c := qt.New(t)
	g := New(LatestVersion)

	ref := g.schemaType(&schema.Type{Typ: &schema.Type_Struct{Struct: &schema.Struct{Fields: []*schema.Field{{
		Name:     "Email",
		JsonName: "email",
		Typ:      &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING}},
		Tags: []*schema.Tag{{
			Key:     "openapi",
			Name:    "format=email",
			Options: []string{"example=user@example.com", "default=unknown@example.com", "minLength=3", "maxLength=255", "pattern=^.+@.+$"},
		}},
	}}}}})

	prop := ref.Value.Properties["email"].Value
	c.Assert(prop.Format, qt.Equals, "email")
	c.Assert(prop.Example, qt.Equals, "user@example.com")
	c.Assert(prop.Default, qt.Equals, "unknown@example.com")
	c.Assert(prop.MinLength, qt.Equals, uint64(3))
	c.Assert(*prop.MaxLength, qt.Equals, uint64(255))
	c.Assert(prop.Pattern, qt.Equals, "^.+@.+$")
}

func TestOpenAPIRawTagParsesEnumValues(t *testing.T) {
	c := qt.New(t)
	ref := applyOpenAPIRawTag(gStringSchema(), `openapi:"enum=pending|paid,example=paid"`)

	c.Assert(ref.Value.Enum, qt.DeepEquals, []any{"pending", "paid"})
	c.Assert(ref.Value.Example, qt.Equals, "paid")
}

func gStringSchema() *openapi3.SchemaRef {
	return openapi3.NewStringSchema().NewRef()
}

func TestOpenAPIEnumDeclValues(t *testing.T) {
	c := qt.New(t)
	g := New(LatestVersion)
	g.md = &meta.Data{}
	g.spec = newSpec("test")
	g.md.Decls = []*schema.Decl{{
		Id:   0,
		Name: "Currency",
		Loc:  &schema.Loc{PkgName: "shop"},
		Type: &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING}},
		EnumValues: []*schema.Literal{
			{Value: &schema.Literal_Str{Str: "EUR"}},
			{Value: &schema.Literal_Str{Str: "USD"}},
		},
	}}

	ref := g.schemaType(&schema.Type{Typ: &schema.Type_Named{Named: &schema.Named{Id: 0}}})
	c.Assert(ref.Ref, qt.Equals, "#/components/schemas/shop.Currency")
	c.Assert(g.spec.Components.Schemas["shop.Currency"].Value.Enum, qt.DeepEquals, []any{"EUR", "USD"})
}

func TestOpenAPITagParsesEnumValues(t *testing.T) {
	c := qt.New(t)
	g := New(LatestVersion)

	ref := g.schemaType(&schema.Type{Typ: &schema.Type_Struct{Struct: &schema.Struct{Fields: []*schema.Field{{
		Name:     "Status",
		JsonName: "status",
		Typ:      &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING}},
		Tags: []*schema.Tag{{
			Key:     "openapi",
			Name:    `enum=["pending","paid"]`,
			Options: []string{"example=paid"},
		}},
	}}}}})

	prop := ref.Value.Properties["status"].Value
	c.Assert(prop.Enum, qt.DeepEquals, []any{"pending", "paid"})
	c.Assert(prop.Example, qt.Equals, "paid")
}

func TestValidationExprAppliesOpenAPIConstraints(t *testing.T) {
	c := qt.New(t)
	g := New(LatestVersion)

	ref := g.schemaType(&schema.Type{
		Typ: &schema.Type_Builtin{Builtin: schema.Builtin_STRING},
		Validation: &schema.ValidationExpr{Expr: &schema.ValidationExpr_And_{And: &schema.ValidationExpr_And{Exprs: []*schema.ValidationExpr{
			{Expr: &schema.ValidationExpr_Rule{Rule: &schema.ValidationRule{Rule: &schema.ValidationRule_MinLen{MinLen: 2}}}},
			{Expr: &schema.ValidationExpr_Rule{Rule: &schema.ValidationRule{Rule: &schema.ValidationRule_MaxLen{MaxLen: 8}}}},
			{Expr: &schema.ValidationExpr_Rule{Rule: &schema.ValidationRule{Rule: &schema.ValidationRule_Is_{Is: schema.ValidationRule_EMAIL}}}},
		}}}},
	})

	c.Assert(ref.Value.MinLength, qt.Equals, uint64(2))
	c.Assert(*ref.Value.MaxLength, qt.Equals, uint64(8))
	c.Assert(ref.Value.Format, qt.Equals, "email")
}
