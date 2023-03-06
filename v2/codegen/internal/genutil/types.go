package genutil

import (
	"fmt"
	gotoken "go/token"
	"reflect"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

func NewHelper(errs *perr.List) *Helper {
	return &Helper{Errs: errs}
}

type Helper struct {
	Errs *perr.List
}

// Type generates a Go type from a schema type.
func (g *Helper) Type(typ schema.Type) *Statement {
	switch typ := typ.(type) {
	case schema.NamedType:
		return g.named(typ)
	case schema.StructType:
		return g.struct_(typ)
	case schema.MapType:
		return Map(g.Type(typ.Key)).Add(g.Type(typ.Value))
	case schema.ListType:
		elem := g.Type(typ.Elem)
		if typ.Len != -1 {
			return Index(Lit(typ.Len)).Add(elem)
		}
		return Index().Add(elem)
	case schema.PointerType:
		return Op("*").Add(g.Type(typ.Elem))
	case schema.BuiltinType:
		return g.Builtin(typ.AST.Pos(), typ.Kind)
	case schema.InterfaceType:
		g.Errs.Add(typ.AST.Pos(), "unexpected interface type")
		return Any()
	case schema.TypeParamRefType:
		typeParam := typ.Decl.TypeParameters()[typ.Index]
		return Id(typeParam.Name)
	default:
		g.Errs.Addf(typ.ASTExpr().Pos(), "unexpected schema type %T", typ)
		return Any()
	}
}

func (g *Helper) named(named schema.NamedType) *Statement {
	st := Q(named.DeclInfo)

	if len(named.TypeArgs) > 0 {
		types := fns.Map(named.TypeArgs, func(arg schema.Type) Code {
			return g.Type(arg)
		})
		st = st.Types(types...)
	}

	return st
}

func (g *Helper) struct_(st schema.StructType) *Statement {
	fields := make([]Code, len(st.Fields))

	for i, field := range st.Fields {
		var f *Statement
		typExpr := g.Type(field.Type)
		if field.IsAnonymous() {
			f = typExpr
		} else {
			f = Id(field.Name.MustGet()).Add(typExpr)
		}

		// Add field tags
		if field.Tag.Len() > 0 {
			tagMap := make(map[string]string)
			for _, tag := range field.Tag.Tags() {
				tagMap[tag.Key] = tag.Value()
			}
			f = f.Tag(tagMap)
		}

		// Add doc comment
		if doc := strings.TrimSpace(field.Doc); doc != "" {
			f = f.Comment(doc)
		}
		fields[i] = f
	}

	return Struct(fields...)
}

func (g *Helper) Builtin(pos gotoken.Pos, kind schema.BuiltinKind) *Statement {
	switch kind {
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
		g.Errs.Addf(pos, "unsupported builtin kind: %v", kind)
		return Id("unsupported")
	}
}

// TypeToString converts a schema.Type to a string.
func (g *Helper) TypeToString(typ schema.Type) string {
	// We wrap the type before rendering in "var _ {type}" so Jen correctly formats,
	// then we strip the "var _" part.
	return fmt.Sprintf("%#v", Var().Id("_").Add(g.Type(typ)))[6:]
}

// Q returns a qualified name (using [jen.Qual]) for the given info.
func Q(info *pkginfo.PkgDeclInfo) *Statement {
	return Qual(info.File.Pkg.ImportPath.String(), info.Name)
}

func (g *Helper) GoToJen(pos gotoken.Pos, val any) *Statement {
	return g.goToJen(pos, reflect.ValueOf(val))
}

func (g *Helper) goToJen(pos gotoken.Pos, val reflect.Value) *Statement {
	switch val.Kind() {
	// All the types supported by jen.Lit can be passed directly.
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.Uintptr:
		return Lit(val.Interface())

	case reflect.Slice, reflect.Array:
		return g.goTypeToJen(pos, val.Type()).ValuesFunc(func(group *Group) {
			for i := 0; i < val.Len(); i++ {
				group.Add(g.goToJen(pos, val.Index(i)))
			}
		})
	case reflect.Map:
		return g.goTypeToJen(pos, val.Type()).ValuesFunc(func(group *Group) {
			iter := val.MapRange()
			for iter.Next() {
				group.Add(g.goToJen(pos, iter.Key())).Op(":").Add(g.goToJen(pos, iter.Value()))
			}
		})
	default:
		g.Errs.Addf(pos, "unsupported type: %T", val.Interface())
		return Null()
	}
}

func (g *Helper) goTypeToJen(pos gotoken.Pos, typ reflect.Type) *Statement {
	switch typ.Kind() {
	case reflect.Bool:
		return Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Uint()
	case reflect.Float32, reflect.Float64:
		return Float32()
	case reflect.String:
		return String()
	case reflect.Slice:
		return Index().Add(g.goTypeToJen(pos, typ.Elem()))
	case reflect.Pointer:
		return Op("*").Add(g.goTypeToJen(pos, typ.Elem()))
	case reflect.Array:
		return Index(Lit(typ.Len())).Add(g.goTypeToJen(pos, typ.Elem()))
	case reflect.Map:
		return Map(g.goTypeToJen(pos, typ.Key())).Add(g.goTypeToJen(pos, typ.Elem()))
	default:
		g.Errs.Addf(pos, "unsupported Go type in codegen: %v", typ)
		return Null()
	}
}

// Zero returns a jen expression representing the zero value
// for the given type. If the type is nil it returns "nil".
func (g *Helper) Zero(typ schema.Type) *Statement {
	isNillable := func(typ schema.Type) bool {
		switch typ.(type) {
		case nil, schema.PointerType, schema.ListType, schema.MapType, schema.FuncType, schema.InterfaceType:
			return true
		default:
			return false
		}
	}

	if isNillable(typ) {
		return Nil()
	} else if named, ok := typ.(schema.NamedType); ok {
		// If the type is a named type, we need to inspect the underlying type.
		if isNillable(named.Decl().Type) {
			return Nil()
		}
	} else if builtin, ok := typ.(schema.BuiltinType); ok {
		return g.builtinZero(builtin)
	}

	// Otherwise return Foo{}.
	return g.Type(typ).Values()
}

// Initialize returns a jen expression for initializing
// the given type. If the type is a pointer type it returns new(Foo),
// and make(Foo) for slices and maps.
//
// Certain types like function types and interfaces return "nil"
// as there is no canonical way to initialize them to a non-zero value.
func (g *Helper) Initialize(typ schema.Type) *Statement {
	switch typ := typ.(type) {
	case schema.PointerType:
		return New(g.Type(typ.Elem))
	case schema.ListType:
		return g.Type(typ).Values()
	case schema.MapType:
		return Make(g.Type(typ))
	case schema.BuiltinType:
		return g.Zero(typ)
	default:
		return Nil()
	}
}

func (g *Helper) builtinZero(builtin schema.BuiltinType) *Statement {
	switch builtin.Kind {
	case schema.Bool:
		return False()
	case schema.Int, schema.Int8, schema.Int16, schema.Int32, schema.Int64,
		schema.Uint, schema.Uint8, schema.Uint16, schema.Uint32, schema.Uint64:
		return Lit(0)
	case schema.Float32, schema.Float64:
		return Lit(0.0)
	case schema.String:
		return Lit("")
	case schema.Bytes:
		return Index().Byte().Values()
	case schema.Time:
		return Qual("time", "Time").Values()
	case schema.UUID:
		return Qual("encore.dev/types/uuid", "UUID").Values()
	case schema.JSON:
		return Parens(Qual("json", "RawMessage")).Call(Nil())
	case schema.UserID:
		return Qual("encore.dev/beta/auth", "UID").Call(Lit(""))
	case schema.Error:
		return Parens(Id("error")).Call(nil)
	default:
		g.Errs.Addf(builtin.ASTExpr().Pos(), "unsupported builtin type: %v", builtin.Kind)
		return Null()
	}
}
