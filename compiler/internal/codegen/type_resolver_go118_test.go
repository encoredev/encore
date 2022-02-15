//go:build go1.18
// +build go1.18

package codegen

import . "encr.dev/proto/encore/parser/schema/v1"

func init() {
	schemaTypeToGoTypeTestCases = append(schemaTypeToGoTypeTestCases, schemaTypeToGoTypeTestCase{
		"generic named types", &Type{Typ: &Type_Named{Named: &Named{Id: 0, TypeArguments: []*Type{intType, uuidType}}}}, "codegentest.UserAge[int, uuid.UUID]",
	})
}
