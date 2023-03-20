package legacymeta

import (
	"fmt"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	schema "encr.dev/proto/encore/parser/schema/v1"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
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
	switch typ := typ.(type) {
	case nil:
		return nil

	case schemav2.BuiltinType:
		return &schema.Type{Typ: &schema.Type_Builtin{
			Builtin: b.builtinType(typ),
		}}

	case schemav2.NamedType:
		if typ.DeclInfo.File.Pkg.ImportPath == "encore.dev/config" {
			return b.configValue(typ)
		}
		return &schema.Type{Typ: &schema.Type_Named{
			Named: &schema.Named{
				Id:            b.decl(typ.Decl()),
				TypeArguments: b.schemaTypes(typ.TypeArgs...),
			},
		}}

	case schemav2.StructType:
		var fields []*schema.Field
		for _, f := range typ.Fields {
			if f.IsAnonymous() {
				continue // not supported by meta
			}
			fields = append(fields, b.structField(f))
		}

		return &schema.Type{Typ: &schema.Type_Struct{
			Struct: &schema.Struct{
				Fields: fields,
			},
		}}

	case schemav2.MapType:
		return &schema.Type{Typ: &schema.Type_Map{
			Map: &schema.Map{
				Key:   b.schemaType(typ.Key),
				Value: b.schemaType(typ.Value),
			},
		}}

	case schemav2.ListType:
		return &schema.Type{Typ: &schema.Type_List{
			List: &schema.List{
				Elem: b.schemaType(typ.Elem),
			},
		}}

	case schemav2.PointerType:
		return &schema.Type{Typ: &schema.Type_Pointer{
			Pointer: &schema.Pointer{
				Base: b.schemaType(typ.Elem),
			},
		}}

	case schemav2.TypeParamRefType:
		return &schema.Type{Typ: &schema.Type_TypeParameter{
			TypeParameter: &schema.TypeParameterRef{
				DeclId:   b.decl(typ.Decl),
				ParamIdx: uint32(typ.Index),
			},
		}}

	default:
		b.errs.Addf(typ.ASTExpr().Pos(), "unsupported schema type %T", typ)
	}

	return nil
}

func (b *builder) structField(f schemav2.StructField) *schema.Field {
	field := &schema.Field{
		Typ:             b.schemaType(f.Type),
		Name:            f.Name.MustGet(),
		Doc:             f.Doc,
		JsonName:        "",
		Optional:        false,
		QueryStringName: "",
		RawTag:          f.Tag.String(),
		Tags:            nil,
	}

	for _, tag := range f.Tag.Tags() {
		field.Tags = append(field.Tags, &schema.Tag{
			Key:     tag.Key,
			Name:    tag.Name,
			Options: tag.Options,
		})
	}

	if enc, _ := f.Tag.Get("encore"); enc != nil {
		ops := append([]string{enc.Name}, enc.Options...)
		for _, o := range ops {
			switch o {
			case "optional":
				field.Optional = true
			}
		}
	}

	if js, _ := f.Tag.Get("json"); js != nil {
		if v := js.Name; v != "" {
			field.JsonName = v
		}
	}

	if qs, _ := f.Tag.Get("qs"); qs != nil {
		if v := qs.Name; v != "" {
			field.QueryStringName = v
		}
	}
	if field.QueryStringName == "" {
		field.QueryStringName = idents.Convert(field.Name, idents.SnakeCase)
	}

	return field
}

func (b *builder) configValue(typ schemav2.NamedType) *schema.Type {
	switch typ.DeclInfo.Name {
	case "Value", "Values":
		isList := typ.DeclInfo.Name == "Values"
		return &schema.Type{Typ: &schema.Type_Config{
			Config: &schema.ConfigValue{
				Elem:         b.schemaType(typ.TypeArgs[0]),
				IsValuesList: isList,
			},
		}}

	default:
		// Should be a named config type, like "type String = Value[string]".
		if named, ok := typ.Decl().Type.(schemav2.NamedType); ok {
			return b.configValue(named)
		} else {
			b.errs.Addf(typ.ASTExpr().Pos(), "unsupported config type %q", typ.DeclInfo.Name)
			return nil
		}
	}
}

func (b *builder) schemaTypes(typs ...schemav2.Type) []*schema.Type {
	return fns.Map(typs, b.schemaType)
}

func (b *builder) declInfo(info *pkginfo.PkgDeclInfo) uint32 {
	return b.declKey(info.File.Pkg.ImportPath, info.Name)
}

func (b *builder) decl(decl schemav2.Decl) uint32 {
	typeDecl, ok := decl.(*schemav2.TypeDecl)
	if !ok {
		b.errs.Fatalf(decl.ASTNode().Pos(), "cannot add declaration %q of type %T", decl.String(), decl)
		return 0 // unreachable
	}

	pkgName, ok := typeDecl.PkgName().Get()
	if !ok {
		b.errs.Fatalf(decl.ASTNode().Pos(), "cannot add declaration %q that's not at package level", decl.String())
		return 0 // unreachable
	}

	// Do we already have this declaration added?
	k := declKey{pkgPath: decl.DeclaredIn().Pkg.ImportPath, pkgName: pkgName}
	if n, ok := b.decls[k]; ok {
		return n
	}
	// Otherwise add it.
	declIdx := uint32(len(b.md.Decls))
	b.decls[k] = declIdx

	typeParams := fns.Map(typeDecl.TypeParams, func(p schemav2.DeclTypeParam) *schema.TypeParameter {
		return &schema.TypeParameter{Name: p.Name}
	})

	b.md.Decls = append(b.md.Decls, &schema.Decl{
		Id:         declIdx,
		Name:       pkgName,
		Type:       b.schemaType(typeDecl.Type),
		TypeParams: typeParams,
		Doc:        typeDecl.Info.Doc,
		Loc:        nil, // TODO?
	})

	return declIdx
}

type declKey struct {
	pkgPath paths.Pkg
	pkgName string
}

func (b *builder) declKey(pkgPath paths.Pkg, pkgName string) uint32 {
	k := declKey{pkgPath: pkgPath, pkgName: pkgName}
	if n, ok := b.decls[k]; ok {
		return n
	}

	n := uint32(len(b.decls))
	b.decls[k] = n
	return n
}

func (b *builder) typeDeclRef(typ *schemav2.TypeDeclRef) *schema.Type {
	return b.schemaType(typ.ToType())
}
