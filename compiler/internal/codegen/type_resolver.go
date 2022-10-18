package codegen

import (
	"fmt"
	"reflect"

	. "github.com/dave/jennifer/jen"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

// schemaTypeToGoType will convert a `schema.Type` into a `Statement` representing how you would write the
// type for a parameter, return type or variable declaration.
func (b *Builder) schemaTypeToGoType(typ *schema.Type) *Statement {
	switch v := typ.Typ.(type) {
	case *schema.Type_Named:
		return b.schemaNamedToGoType(v.Named)

	case *schema.Type_Struct:
		return b.schemaStructToGoType(v.Struct)

	case *schema.Type_Map:
		return Map(b.schemaTypeToGoType(v.Map.Key)).
			Add(b.schemaTypeToGoType(v.Map.Value))

	case *schema.Type_Builtin:
		return b.schemaBuiltInToGoType(v.Builtin)

	case *schema.Type_Pointer:
		return Op("*").Add(b.schemaTypeToGoType(v.Pointer.Base))

	case *schema.Type_List:
		return Index().
			Add(b.schemaTypeToGoType(v.List.Elem))

	case *schema.Type_TypeParameter:
		panic(bailout{fmt.Errorf("did not expect the type parameter on a initialized value")})

	case *schema.Type_Config:
		if v.Config.IsValuesList {
			return Qual("encore.dev/config", "Values").Types(b.schemaTypeToGoType(v.Config.Elem))
		} else {
			return Qual("encore.dev/config", "Value").Types(b.schemaTypeToGoType(v.Config.Elem))
		}

	default:
		panic(bailout{fmt.Errorf("unknown type: %+v", reflect.TypeOf(typ.Typ))})
	}
}

func (b *Builder) schemaNamedToGoType(named *schema.Named) *Statement {
	decl := b.res.App.Decls[named.Id]
	if decl == nil {
		panic(bailout{fmt.Errorf("unable to fetch decleration id %d", named.Id)})
	}

	statement := Qual(decl.Loc.PkgPath, decl.Name)

	if len(named.TypeArguments) > 0 {
		types := make([]Code, len(named.TypeArguments))

		for i, typeArgs := range named.TypeArguments {
			types[i] = b.schemaTypeToGoType(typeArgs)
		}

		statement = statement.Types(types...)
	}

	return statement
}

// schemaStructToGoType converts a parsed struct back into a Jennifer statement maintaining comments and JSON tags
func (b *Builder) schemaStructToGoType(structure *schema.Struct) *Statement {
	fields := make([]Code, len(structure.Fields))

	for i, field := range structure.Fields {
		statement := Id(field.Name).Add(b.schemaTypeToGoType(field.Typ))

		// Add the field tags
		tags := make(map[string]string)
		if field.JsonName != "" {
			tags["json"] = field.JsonName
			if field.Optional {
				tags["json"] += ",omitEmpty"
			}
		}

		if field.Optional {
			tags["encore"] = "optional"
		}

		if field.QueryStringName != "" {
			tags["qs"] = field.QueryStringName
		}

		if len(tags) > 0 {
			statement = statement.Tag(tags)
		}

		// Add the comment
		if field.Doc != "" {
			statement = statement.Comment(field.Doc)
		}

		fields[i] = statement
	}

	return Struct(fields...)
}

// schemaBuiltInToGoType returns the statement representing the BuiltIn type
func (b *Builder) schemaBuiltInToGoType(builtin schema.Builtin) *Statement {
	switch builtin {
	case schema.Builtin_ANY:
		return Any()

	case schema.Builtin_BOOL:
		return Bool()

	case schema.Builtin_INT8:
		return Int8()

	case schema.Builtin_INT16:
		return Int16()

	case schema.Builtin_INT32:
		return Int32()

	case schema.Builtin_INT64:
		return Int64()

	case schema.Builtin_UINT8:
		return Uint8()

	case schema.Builtin_UINT16:
		return Uint16()

	case schema.Builtin_UINT32:
		return Uint32()

	case schema.Builtin_UINT64:
		return Uint64()

	case schema.Builtin_FLOAT32:
		return Float32()

	case schema.Builtin_FLOAT64:
		return Float64()

	case schema.Builtin_STRING:
		return String()

	case schema.Builtin_BYTES:
		return Index().Byte()

	case schema.Builtin_TIME:
		return Qual("time", "Time")

	case schema.Builtin_UUID:
		return Qual("encore.dev/types/uuid", "UUID")

	case schema.Builtin_JSON:
		return Qual("encoding/json", "RawMessage")

	case schema.Builtin_USER_ID:
		return Qual("encore.dev/beta/auth", "UID")

	case schema.Builtin_INT:
		return Int()

	case schema.Builtin_UINT:
		return Uint()

	default:
		panic(bailout{fmt.Errorf("unknown builtin type: %d", builtin)})
	}
}
