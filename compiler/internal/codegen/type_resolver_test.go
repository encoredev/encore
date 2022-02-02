package codegen

import (
	"fmt"
	"testing"

	"github.com/dave/jennifer/jen"
	qt "github.com/frankban/quicktest"

	"encr.dev/parser"
	"encr.dev/parser/est"
	. "encr.dev/proto/encore/parser/schema/v1"
)

func Test_schemaTypeToGoType(t *testing.T) {
	t.Parallel()

	intType := &Type{Typ: &Type_Builtin{Builtin: Builtin_INT}}
	uuidType := &Type{Typ: &Type_Builtin{Builtin: Builtin_UUID}}
	structType := &Type{Typ: &Type_Struct{Struct: &Struct{Fields: []*Field{
		{Name: "Age", Typ: intType, Doc: "age at sign up"},
		{Name: "DateOfBirth", Typ: &Type{Typ: &Type_Builtin{Builtin: Builtin_TIME}}, Doc: "date of birth", JsonName: "dob", Optional: true},
		{Name: "Id", Typ: uuidType, JsonName: "-"},
	}}}}

	tests := []struct {
		name string
		typ  *Type
		want string
	}{
		{"any", &Type{Typ: &Type_Builtin{Builtin: Builtin_ANY}}, "any"},
		{"base language type", intType, "int"},
		{"byte slices", &Type{Typ: &Type_Builtin{Builtin: Builtin_BYTES}}, "[]byte"},
		{"standard library type", &Type{Typ: &Type_Builtin{Builtin: Builtin_TIME}}, "time.Time"},
		{"encore types", &Type{Typ: &Type_Builtin{Builtin: Builtin_UUID}}, "uuid.UUID"},
		{"map types", &Type{Typ: &Type_Map{Map: &Map{
			Key:   intType,
			Value: uuidType,
		}}}, "map[int]uuid.UUID"},
		{"struct types", structType,
			`struct {
	Age         int       // age at sign up
	DateOfBirth time.Time ` + "`json:\"dob,omitEmpty\"`" + ` // date of birth
	Id          uuid.UUID ` + "`json:\"-\"`" + `
}`,
		},
		{
			"basic named types", &Type{Typ: &Type_Named{Named: &Named{Id: 0}}}, "codegentest.UserAge",
		},
		{
			"generic named types", &Type{Typ: &Type_Named{Named: &Named{Id: 0, TypeArguments: []*Type{intType, uuidType}}}}, "codegentest.UserAge[int, uuid.UUID]",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := qt.New(t)

			b := &Builder{
				res: &parser.Result{
					App: &est.Application{
						Decls: []*Decl{
							{
								Id:   0,
								Name: "UserAge",
								Type: structType,
								Loc:  &Loc{PkgPath: "github.com/encoredev/compiler/internal/codegentest"},
							},
						},
					},
				},
			}

			statement := b.schemaTypeToGoType(tt.typ)
			source := fmt.Sprintf(
				"%#v",
				jen.Var().Id("a").Add(statement), // note: we wrap in a "var a" to allow index types ([]byte) to render as expected
			)

			c.Assert(source, qt.Equals, "var a "+tt.want)
		})
	}
}
