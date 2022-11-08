package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func (b *Builder) Etype() (f *File, err error) {
	defer b.errors.HandleBailout(&err)
	f = NewFile("etype")
	b.registerImports(f, "")
	b.marshaller.WriteToFile(f)
	return f, b.errors.Err()
}
func (b *Builder) builtinType(t schema.Builtin) *Statement {
	switch t {
	case schema.Builtin_STRING:
		return String()
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
	case schema.Builtin_INT:
		return Int()
	case schema.Builtin_UINT8:
		return Uint8()
	case schema.Builtin_UINT16:
		return Uint16()
	case schema.Builtin_UINT32:
		return Uint32()
	case schema.Builtin_UINT64:
		return Uint64()
	case schema.Builtin_UINT:
		return Uint()
	case schema.Builtin_UUID:
		return Qual("encore.dev/types/uuid", "UUID")
	default:
		panic(fmt.Sprintf("unexpected builtin type %v", t))
	}
}

func (b *Builder) namedType(f *File, param *est.Param) *Statement {
	if named := param.Type.GetNamed(); named != nil {
		decl := b.res.App.Decls[named.Id]
		f.ImportName(decl.Loc.PkgPath, decl.Loc.PkgName)
	}

	return b.typeName(param, false)
}
