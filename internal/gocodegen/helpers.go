package gocodegen

import (
	"fmt"
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"

	. "github.com/dave/jennifer/jen"
)

// ConvertSchemaTypeToString converts a schema.Type to a string that can be used in a log output
// which can be increbily useful for debugging if the parser as generated the expected Schema protobuf
// from the original Go code
func ConvertSchemaTypeToString(typ *schema.Type, md *meta.Data) string {
	// We wrap the type before rendering in "var _ {type}" so Jen correctly formats, then we strip the "var _" part.
	return fmt.Sprintf("%#v", Var().Id("_").Add(ConvertSchemaToJenType(typ, md)))[6:]
}

// ConvertSchemaToJenType converts a schema.Type to a Jen statement which represents the type
func ConvertSchemaToJenType(typ *schema.Type, md *meta.Data) *Statement {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Named:
		id := Id(md.Decls[typ.Named.Id].Name)

		if len(typ.Named.TypeArguments) > 0 {
			typeParams := make([]Code, len(typ.Named.TypeArguments))
			for i, arg := range typ.Named.TypeArguments {
				typeParams[i] = ConvertSchemaToJenType(arg, md)
			}
			id.Types(typeParams...)
		} else if len(md.Decls[typ.Named.Id].TypeParams) > 0 {
			typeParams := make([]Code, len(md.Decls[typ.Named.Id].TypeParams))
			for i, params := range md.Decls[typ.Named.Id].TypeParams {
				typeParams[i] = Id(fmt.Sprintf("_Unknown_Type_%s_", params.Name))
			}
			id.Types(typeParams...)
		}

		return id

	case *schema.Type_Struct:
		fields := make([]Code, len(typ.Struct.Fields))
		for i, field := range typ.Struct.Fields {
			f := Id(field.Name).Add(ConvertSchemaToJenType(field.Typ, md))

			tags := make(map[string]string)
			for _, tag := range field.Tags {
				tags[tag.Key] = tag.Name
				if len(tag.Options) > 0 {
					tags[tag.Key] += fmt.Sprintf(",%s", strings.Join(tag.Options, ","))
				}
			}
			if len(tags) > 0 {
				f = f.Tag(tags)
			}

			if doc := strings.TrimSpace(field.Doc); doc != "" {
				f = f.Comment(doc)
			}

			fields[i] = f
		}
		return Struct(fields...)

	case *schema.Type_Map:
		key := ConvertSchemaToJenType(typ.Map.Key, md)
		value := ConvertSchemaToJenType(typ.Map.Value, md)
		return Map(key).Add(value)

	case *schema.Type_List:
		value := ConvertSchemaToJenType(typ.List.Elem, md)
		return Index().Add(value)

	case *schema.Type_Builtin:
		return ConvertBuiltInSchemaToJenType(typ.Builtin)

	case *schema.Type_TypeParameter:
		return Id(md.Decls[typ.TypeParameter.DeclId].TypeParams[typ.TypeParameter.ParamIdx].Name)

	case *schema.Type_Config:
		if typ.Config.IsValuesList {
			return Qual("encore.dev/config", "Values").Types(ConvertSchemaToJenType(typ.Config.Elem, md))
		} else {

			return Qual("encore.dev/config", "Value").Types(ConvertSchemaToJenType(typ.Config.Elem, md))
		}

	default:
		panic(fmt.Sprintf("ConvertSchemaToJenType doesn't support type: %T", typ))
	}
}

func ConvertBuiltInSchemaToJenType(builtin schema.Builtin) *Statement {
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
		panic(fmt.Sprintf("ConvertBuiltInSchemaToJenType doesn't support builtin: %v", builtin))
	}
}
