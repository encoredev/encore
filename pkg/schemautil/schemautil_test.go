package schemautil

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

func builtin(b schema.Builtin) *schema.Type {
	return &schema.Type{Typ: &schema.Type_Builtin{Builtin: b}}
}

func field(name, jsonName string, typ *schema.Type) *schema.Field {
	return &schema.Field{Name: name, JsonName: jsonName, Typ: typ}
}

func TestStructBitsWithDeclsRendersOptionValue(t *testing.T) {
	c := qt.New(t)
	optUUID := &schema.Type{Typ: &schema.Type_Option{Option: &schema.Option{Value: builtin(schema.Builtin_UUID)}}}
	s := &schema.Struct{Fields: []*schema.Field{field("ID", "id", optUUID)}}

	_, _, _, body := StructBitsWithDecls(nil, s, nil, "POST", false, false, true)
	c.Assert(body, qt.Contains, `"id": "7d42f515-3517-4e76-be13-30880443546f"`)
}

func TestStructBitsUsesOpenAPIExample(t *testing.T) {
	c := qt.New(t)
	f := field("ID", "id", &schema.Type{Typ: &schema.Type_Option{Option: &schema.Option{Value: builtin(schema.Builtin_UUID)}}})
	f.Tags = []*schema.Tag{{Key: "openapi", Name: "example=00000000-0000-0000-0000-000000000000"}}
	s := &schema.Struct{Fields: []*schema.Field{f}}

	_, _, _, body := StructBitsWithDecls(nil, s, nil, "POST", false, false, true)
	c.Assert(body, qt.Contains, `"id": "00000000-0000-0000-0000-000000000000"`)
}

func TestStructBitsUsesOpenAPIExampleForQueryString(t *testing.T) {
	c := qt.New(t)
	f := field("ID", "id", builtin(schema.Builtin_STRING))
	f.QueryStringName = "id"
	f.Tags = []*schema.Tag{{Key: "openapi", Name: "example=user 123"}}
	s := &schema.Struct{Fields: []*schema.Field{f}}

	query, _, _, _ := StructBitsWithDecls(nil, s, nil, "GET", false, false, false)

	c.Assert(query, qt.Equals, "?id=user+123")
}

func TestStructBitsWithDeclsResolvesNamedGenericType(t *testing.T) {
	c := qt.New(t)
	typeParam := &schema.Type{Typ: &schema.Type_TypeParameter{TypeParameter: &schema.TypeParameterRef{DeclId: 1, ParamIdx: 0}}}
	wrapperStruct := &schema.Struct{Fields: []*schema.Field{field("Value", "value", typeParam)}}
	decls := map[uint32]*schema.Decl{
		1: {Id: 1, Name: "Wrapper", Type: &schema.Type{Typ: &schema.Type_Struct{Struct: wrapperStruct}}},
	}
	namedWrapperUUID := &schema.Type{Typ: &schema.Type_Named{Named: &schema.Named{
		Id:            1,
		TypeArguments: []*schema.Type{builtin(schema.Builtin_UUID)},
	}}}
	s := &schema.Struct{Fields: []*schema.Field{field("Wrapped", "wrapped", namedWrapperUUID)}}

	_, _, _, body := StructBitsWithDecls(decls, s, nil, "POST", false, false, true)
	c.Assert(body, qt.Contains, `"wrapped": {"value": "7d42f515-3517-4e76-be13-30880443546f"}`)
}

func TestStructBitsWithDeclsStopsRecursiveNamedTypes(t *testing.T) {
	c := qt.New(t)
	recursive := &schema.Type{Typ: &schema.Type_Named{Named: &schema.Named{Id: 1}}}
	decls := map[uint32]*schema.Decl{
		1: {Id: 1, Name: "Node", Type: &schema.Type{Typ: &schema.Type_Struct{Struct: &schema.Struct{Fields: []*schema.Field{
			field("Next", "next", recursive),
		}}}}},
	}
	s := &schema.Struct{Fields: []*schema.Field{field("Root", "root", recursive)}}

	_, _, _, body := StructBitsWithDecls(decls, s, nil, "POST", false, false, true)
	c.Assert(body, qt.Contains, `"root": {"next": null}`)
}

func TestStructBitsSkipsJSONIgnoredFieldsWithoutDanglingComma(t *testing.T) {
	c := qt.New(t)
	s := &schema.Struct{Fields: []*schema.Field{
		field("Ignored", "-", builtin(schema.Builtin_STRING)),
		field("Name", "name", builtin(schema.Builtin_STRING)),
	}}

	_, _, _, body := StructBitsWithDecls(nil, s, nil, "POST", false, false, true)
	c.Assert(body, qt.Contains, `"name": ""`)
	c.Assert(strings.Contains(body, `{,`), qt.IsFalse)
}
