package legacymeta

import (
	"fmt"

	schema "encr.dev/proto/encore/parser/schema/v1"
	schemav2 "encr.dev/v2/internal/schema"
)

func (b *builder) builtinType(typ schemav2.BuiltinType) schema.Builtin {
	switch typ.Kind {
	case schemav2.Bool:
		return schema.Builtin_BOOL
	case schemav2.Int:
		return schema.Builtin_INT
	case schemav2.Int8:
		return schema.Builtin_INT8
	case schemav2.Int16:
		return schema.Builtin_INT16
	case schemav2.Int32:
		return schema.Builtin_INT32
	case schemav2.Int64:
		return schema.Builtin_INT64
	case schemav2.Uint:
		return schema.Builtin_UINT
	case schemav2.Uint8:
		return schema.Builtin_UINT8
	case schemav2.Uint16:
		return schema.Builtin_UINT16
	case schemav2.Uint32:
		return schema.Builtin_UINT32
	case schemav2.Uint64:
		return schema.Builtin_UINT64

	case schemav2.Float32:
		return schema.Builtin_FLOAT32
	case schemav2.Float64:
		return schema.Builtin_FLOAT64
	case schemav2.String:
		return schema.Builtin_STRING
	case schemav2.Bytes:
		return schema.Builtin_BYTES

	case schemav2.Time:
		return schema.Builtin_TIME
	case schemav2.UUID:
		return schema.Builtin_UUID
	case schemav2.JSON:
		return schema.Builtin_JSON
	case schemav2.UserID:
		return schema.Builtin_USER_ID

	default:
		panic(fmt.Sprintf("unknown builtin type %v", typ.Kind))
	}
}

func (b *builder) schemaType(typ schemav2.Type) *schema.Type {
	return nil // TODO
}

func (b *builder) typeDeclRef(typ *schemav2.TypeDeclRef) *schema.Type {
	return nil // TODO
}
