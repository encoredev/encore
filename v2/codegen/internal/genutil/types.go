package genutil

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

func NewGenerator(errs *perr.List) *Generator {
	return &Generator{Errs: errs}
}

type Generator struct {
	Errs *perr.List
}

// Type generates a Go type from a schema type.
func (g *Generator) Type(typ schema.Type) *Statement {
	switch typ := typ.(type) {
	case schema.NamedType:
		return g.named(typ)
	case schema.StructType:
		return g.struct_(typ)
	case schema.MapType:
		return Map(g.Type(typ.Key)).Add(g.Type(typ.Value))
	case schema.ListType:
		return Index().Add(g.Type(typ.Elem))
	case schema.PointerType:
		return Op("*").Add(g.Type(typ.Elem))
	case schema.BuiltinType:
		return g.builtin(typ)
	case schema.InterfaceType:
		g.Errs.Add(typ.AST.Pos(), "unexpected interface type")
		return Any()
	case schema.TypeParamRefType:
		g.Errs.Add(typ.AST.Pos(), "unexpected type parameter reference")
		return Any()
	default:
		g.Errs.Addf(typ.ASTExpr().Pos(), "unexpected schema type %T", typ)
		return Any()
	}
}

func (g *Generator) named(named schema.NamedType) *Statement {
	st := Q(named.DeclInfo)

	if len(named.TypeArgs) > 0 {
		types := fns.Map(named.TypeArgs, func(arg schema.Type) Code {
			return g.Type(arg)
		})
		st = st.Types(types...)
	}

	return st
}

func (g *Generator) struct_(st schema.StructType) *Statement {
	fields := make([]Code, len(st.Fields))

	for i, field := range st.Fields {
		statement := Id(field.Name.GetOrDefault("")).Add(g.Type(field.Type))

		// Add field tags
		if field.Tag.Len() > 0 {
			tagMap := make(map[string]string)
			for _, tag := range field.Tag.Tags() {
				tagMap[tag.Key] = tag.Value()
			}
			statement = statement.Tag(tagMap)
		}

		// Add doc comment
		statement = statement.Comment(field.Doc)
		fields[i] = statement
	}

	return Struct(fields...)
}

func (g *Generator) builtin(typ schema.BuiltinType) *Statement {
	switch typ.Kind {
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
	case schema.JSON:
		return Qual("encoding/json", "RawMessage")
	case schema.UUID:
		return Qual("encore.dev/types/uuid", "UUID")
	case schema.UserID:
		return Qual("encore.dev/beta/auth", "UID")
	case schema.Error:
		return Error()
	default:
		g.Errs.Addf(typ.AST.Pos(), "unsupported builtin kind: %v", typ.Kind)
		return Id("unsupported")
	}
}

// Q returns a qualified name (using [jen.Qual]) for the given info.
func Q(info *pkginfo.PkgDeclInfo) *Statement {
	return Qual(info.File.Pkg.ImportPath.String(), info.Name)
}
