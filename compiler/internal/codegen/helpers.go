package codegen

import (
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func derefPointer(typ *schema.Type) *schema.Type {
	if p, ok := typ.Typ.(*schema.Type_Pointer); ok {
		return derefPointer(p.Pointer.Base)
	}
	return typ
}
