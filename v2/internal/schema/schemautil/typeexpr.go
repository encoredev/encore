package schemautil

import (
	"fmt"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/internal/schema"
)

// TypeToString converts a schema.Type to a string.
func TypeToString(typ schema.Type) string {
	// We wrap the type before rendering in "var _ {type}" so Jen correctly formats, then we strip the "var _" part.
	return fmt.Sprintf("%#v", Var().Id("_").Add(TypeToJen(typ)))[6:]
}

// TypeToJen converts a schema.Type to a jen statement which represents the type.
func TypeToJen(typ schema.Type) *Statement {
	switch typ := typ.(type) {
	case schema.NamedType:
		id := Qual(typ.DeclInfo.File.Pkg.ImportPath.String(), typ.DeclInfo.Name)

		if num := len(typ.TypeArgs); num > 0 {
			typeParams := make([]Code, num)
			for i, arg := range typ.TypeArgs {
				typeParams[i] = TypeToJen(arg)
			}
			id.Types(typeParams...)
		} else if params := typ.Decl().TypeParams; len(params) > 0 {
			typeParams := make([]Code, len(params))
			for i, p := range params {
				typeParams[i] = Id(fmt.Sprintf("_Unknown_Type_%s_", p.Name))
			}
			id.Types(typeParams...)
		}

		return id

	case schema.StructType:
		fields := make([]Code, len(typ.Fields))
		for i, field := range typ.Fields {
			typExpr := TypeToJen(field.Type)
			var f *Statement
			if field.IsAnonymous() {
				f = typExpr
			} else {
				f = Id(field.Name.MustGet()).Add(typExpr)
			}

			tags := make(map[string]string)
			for _, tag := range field.Tag.Tags() {
				tags[tag.Key] = tag.String()
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

	case schema.MapType:
		key := TypeToJen(typ.Key)
		value := TypeToJen(typ.Value)
		return Map(key).Add(value)

	case schema.ListType:
		value := TypeToJen(typ.Elem)
		return Index().Add(value)

	case schema.BuiltinType:
		return BuiltinToJen(typ.Kind)

	case schema.PointerType:
		return Op("*").Add(TypeToJen(typ.Elem))

	case schema.TypeParamRefType:
		typeParam := typ.Decl.TypeParameters()[typ.Index]
		return Id(typeParam.Name)

	default:
		panic(fmt.Sprintf("TypeToJen doesn't support type: %T", typ))
	}
}

// BuiltinToJen converts a schema.BuiltinKind to a jen statement which represents the type.
func BuiltinToJen(builtin schema.BuiltinKind) *Statement {
	switch builtin {
	case schema.Any:
		return Any()
	case schema.Bool:
		return Bool()
	case schema.Int:
		return Int()
	case schema.Int8:
		return Int8()
	case schema.Int16:
		return Int16()
	case schema.Int32:
		return Int32()
	case schema.Int64:
		return Int64()
	case schema.Uint:
		return Uint()
	case schema.Uint8:
		return Uint8()
	case schema.Uint16:
		return Uint16()
	case schema.Uint32:
		return Uint32()
	case schema.Uint64:
		return Uint64()
	case schema.Float32:
		return Float32()
	case schema.Float64:
		return Float64()
	case schema.String:
		return String()
	case schema.Bytes:
		return Index().Byte()
	case schema.Time:
		return Qual("time", "Time")
	case schema.UUID:
		return Qual("encore.dev/types/uuid", "UUID")
	case schema.JSON:
		return Qual("encoding/json", "RawMessage")
	case schema.UserID:
		return Qual("encore.dev/beta/auth", "UID")
	default:
		panic(fmt.Sprintf("BuiltinToJen doesn't support builtin: %v", builtin))
	}
}
