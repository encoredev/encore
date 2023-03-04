package configgen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/eerror"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/resource/config"
)

func Gen(gen *codegen.Generator, pkg *pkginfo.Package, loads []*config.Load) {
	f := gen.File(pkg, "config_unmarshal")

	builder := &configUnmarshalersBuilder{
		errs:         gen.Errs,
		gu:           gen.Util,
		f:            f.Jen,
		unmarshalers: make([]*Statement, 0),
	}
	f.Jen.ImportAlias("github.com/json-iterator/go", "jsoniter")

	f.Jen.Comment(`These functions are automatically generated and maintained by Encore to allow config values
to be unmarshalled into the correct types. They are not intended to be used directly. They
are automatically updated by Encore whenever you change the data types used within your
calls to config.Load[T]().`)

	// Find all the types to write, and then write unmarshalers for them
	typesToWrite := builder.FindAllDecls(loads)

	// Write load functions for generic types
	for _, load := range loads {
		builder.generateConcreteUnmarshalers(load.Type)
	}
	if len(builder.unmarshalers) > 0 {
		builder.f.Line()
		builder.f.Comment("// Concrete unmarshalers for all config.Load calls, including those using generic types.")
		builder.f.Comment("// These instances are used directly by calls to `config.Load[T]()`.")
		builder.f.Var().DefsFunc(func(f *Group) {
			for _, typ := range builder.unmarshalers {
				f.Add(typ)
			}
		})
	}

	// Write unmarshalers for all the types needed for config
	for _, decl := range typesToWrite {
		if err := builder.WriteTypeUnmarshaler(decl); err != nil {
			gen.Errs.Addf(decl.AST.Pos(), "failed to generate config unmarshaler for %s: %v", decl.Name, err)
		}
	}

	// Rewrite load functions to inject marshallers
	for _, load := range loads {
		rw := gen.Rewrite(load.File)
		var buf bytes.Buffer
		buf.WriteString(strconv.Quote("SERVICE")) // TODO(andre) used to be service name
		buf.WriteString(", ")
		buf.WriteString(ConfigUnmarshalFuncName(gen.Util, load.Type))
		ep := gen.FS.Position(load.FuncCall.Rparen)
		_, _ = fmt.Fprintf(&buf, "/*line :%d:%d*/", ep.Line, ep.Column)
		rw.Replace(load.FuncCall.Lparen+1, load.FuncCall.Rparen, buf.Bytes())
	}
}

type configUnmarshalersBuilder struct {
	errs *perr.List
	gu   *genutil.Helper
	f    *File

	unmarshalers []*Statement
}

// FindAllDecls returns a list of all decl's used within a list of config loads.
func (cb *configUnmarshalersBuilder) FindAllDecls(loads []*config.Load) []*schema.TypeDecl {
	typesToWrite := make(map[*schema.TypeDecl]struct{})

	// Walk the config load calls in this service and find all the named types used
	for _, load := range loads {
		schemautil.Walk(load.Type, func(node schema.Type) bool {
			switch n := node.(type) {
			case schema.NamedType:
				// Ignore config.Foo types
				if n.DeclInfo.File.Pkg.ImportPath != "encore.dev/config" {
					typesToWrite[n.Decl()] = struct{}{}
				}
			}
			return true
		})
	}

	// Now convert the list of decls into a sorted list of nodes
	decls := make([]*schema.TypeDecl, 0, len(typesToWrite))
	for decl := range typesToWrite {
		decls = append(decls, decl)
	}
	slices.SortFunc(decls, func(a, b *schema.TypeDecl) bool {
		// Sort first by pkg path and then by name
		if a.File.Pkg.ImportPath != b.File.Pkg.ImportPath {
			return a.File.Pkg.ImportPath < b.File.Pkg.ImportPath
		}
		return a.Name < b.Name
	})

	return decls
}

// WriteTypeUnmarshaler writes a function which will unmarshal the given decl from JSON to the instance of the
// Go type. If the decl takes type parameters, then the function generated will also be generic and will require
// unmarshalers for the type parameters to be passed in.
func (cb *configUnmarshalersBuilder) WriteTypeUnmarshaler(decl *schema.TypeDecl) (err error) {
	unmarshalerName, _ := cb.typeUnmarshalerName(decl)

	cb.f.Line()
	cb.f.Commentf(
		"// %s will unmarshal the JSON representation into the given type, taking account for "+
			"\n// the `config.Value` dynamic functions.",
		unmarshalerName,
	)
	f := cb.f.Func().Id(unmarshalerName)
	rtnType := Id("obj").Id(decl.Name)

	// This is the function body (plus arguments) needed for config.Unmarshaler
	unmarshalBody := Params(
		Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
		Id("path").Index().String(),
	).Params(
		rtnType,
	).BlockFunc(func(f *Group) {
		struc, ok := decl.Type.(schema.StructType)
		if !ok {
			err = eerror.New("codegen", "can only unmarshal struct types", nil)
			return
		}
		cb.readStruct(f, struc)
		f.Return()
	})

	// If this is a generic type, we add an additional wrapper so we can provide the generic parameters
	if len(decl.TypeParams) > 0 {
		typeParams := make([]Code, len(decl.TypeParams))
		outerParams := make([]Code, len(decl.TypeParams))
		returnTypeParams := make([]Code, len(decl.TypeParams))
		for i, param := range decl.TypeParams {
			typeParams[i] = Id(param.Name).Any()
			outerParams[i] = Id(cb.typeParamUnmarshalerName(param)).Qual("encore.dev/config", "Unmarshaler").Types(Id(param.Name))
			returnTypeParams[i] = Id(param.Name)
		}
		f = f.Types(typeParams...)
		rtnType = rtnType.Types(returnTypeParams...)

		// Then the outer function needs to return a concrete instance of an unmarshaler which
		// uses the unmarshal body
		f = f.Params(outerParams...).
			Params(Id("concreteUnmarshaler").Qual("encore.dev/config", "Unmarshaler").Types(Id(decl.Name).Types(returnTypeParams...))).
			Block(Return(Func().Add(unmarshalBody)))
	} else {
		// If this isn't generic, we can just use the unmarshal body directly
		f.Add(unmarshalBody)
	}

	return
}

// readType generates code to read a single instance of the given type directly from `iter` which should be a pointer
// to a jsoniter.Iterator.
//
// The `reader` code returned by this function is expected to be used as the right hand side of a single assignment
// statement.
// i.e. `var x = readType(...)` => `var x = iter.ReadBool()`
//
// The second return value is the identifier of the type from the first return value.
// ie. `bool`
func (cb *configUnmarshalersBuilder) readType(typ schema.Type, pathElement Code) (reader Code, rtnTyp *Statement) {
	switch t := typ.(type) {
	case schema.NamedType:
		if t.DeclInfo.File.Pkg.ImportPath == "encore.dev/config" {
			// The config type is the dynamic values which can be changed at runtime
			// by unit tests
			if underlying, isList := cb.resolveValueType(t); isList {
				code, _ := cb.readType(schema.ListType{Elem: underlying}, pathElement)
				_, returnType := cb.readType(underlying, pathElement)
				return Qual("encore.dev/config", "CreateValueList").Call(code, Append(Id("path"), pathElement)), Qual("encore.dev/config", "Values").Types(returnType)
			} else {
				code, returnType := cb.readType(underlying, pathElement)
				return Qual("encore.dev/config", "CreateValue").Types(returnType).Call(code, Append(Id("path"), pathElement)), Qual("encore.dev/config", "Value").Types(returnType)
			}
		}

		funcRef, returnType := cb.typeUnmarshalerFunc(typ)

		return funcRef.Call(
			Id("itr"),
			Append(Id("path"), pathElement),
		), returnType

	case schema.StructType:
		var returnType = Nil()
		block := BlockFunc(func(f *Group) {
			returnType = cb.readStruct(f, t)
			f.Return()
		})
		return Func().Params().Params(Id("obj").Add(returnType)).Add(block).Call(), returnType

	case schema.MapType:
		_, keyType := cb.readType(t.Key, nil)
		valueUnmarshaler, valueType := cb.readType(t.Value, Id("keyAsString"))
		rtnType := Map(keyType).Add(valueType)

		// Call a helper method in the runtime package, which requires us to pass a callback
		// which returns the unmarshalled keys and values.
		return Qual("encore.dev/config", "ReadMap").Types(keyType, valueType).Call(
				Id("itr"),
				Func().Params(
					Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
					Id("keyAsString").String(),
				).Params(keyType, valueType).BlockFunc(func(f *Group) {
					// Note because _all_ keys in JSON objects are strings, we use
					// the etype unmarshaler to unmarshal the key to the underlying datatype
					f.Comment("Decode the map key from the JSON string to the underlying type it needs to be")
					u := cb.gu.NewTypeUnmarshaller("keyDecoder")
					f.Add(u.Init())
					builtin, ok := t.Key.(schema.BuiltinType)
					if !ok {
						cb.errs.Addf(t.Key.ASTExpr().Pos(), "map keys must be builtins")
						return
					}
					f.Id("key").Op(":=").Add(u.UnmarshalBuiltin(builtin.Kind, "keyAsString", Id("keyAsString"), true))
					f.If(Err().Op(":=").Add(u.Err()), Err().Op("!=").Nil()).Block(
						Panic(Qual("fmt", "Sprintf").Call(Lit("unable to decode the config: %v"), Err())))

					// Then we can just return the key and value
					f.Return(Id("key"), valueUnmarshaler)
				}),
			),
			rtnType

	case schema.ListType:
		unmarshaler, returnType := cb.readType(t.Elem, Qual("strconv", "Itoa").Call(Id("idx")))

		// We'll call a helper method in the runtime package, which requires us to pass
		// an unmarshaler for the list type and builds the return slice
		return Qual("encore.dev/config", "ReadArray").Types(returnType).Call(
				Id("itr"),
				Func().Params(
					Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
					Id("idx").Int(),
				).Add(returnType).Block(
					Return(unmarshaler),
				),
			),
			Index().Add(returnType)

	case schema.BuiltinType:
		return cb.readBuiltin(t.Kind)

	case schema.PointerType:
		reader, returnType := cb.readType(t.Elem, pathElement)

		return Func().Params().Op("*").Add(returnType).Block(
			Comment("// If the value is null, we return nil"),
			If(Id("itr").Dot("ReadNil").Call()).Block(
				Return(Nil()),
			),
			Line(),
			Comment("// Otherwise we unmarshal the value and return a pointer to it"),
			Id("obj").Op(":=").Add(reader),
			Return(Op("&").Id("obj")),
		).Call(), Op("*").Add(returnType)

	case schema.TypeParamRefType:
		typeParam := t.Decl.TypeParameters()[t.Index]
		funcName := cb.typeParamUnmarshalerName(typeParam)

		return Id(funcName).Call(Id("itr"), Append(Id("path"), pathElement)), Id(typeParam.Name)

	default:
		panic(fmt.Sprintf("unsupported type for config unmarshalling: %T", t))
	}
}

// readStruct generates code to read a struct from the given iterator. The code generated by this function
// expects a zero value of the struct to be present in the variable `obj`. This code will be written into the
// group `f` passed in as a argument.
//
// The returned type will be the type definition of the struct.
func (cb *configUnmarshalersBuilder) readStruct(f *Group, struc schema.StructType) (returnType *Statement) {
	fieldTypes := make([]Code, len(struc.Fields))

	f.Id("itr").Dot("ReadObjectCB").Call(Func().Params(
		Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
		Id("field").String(),
	).Bool().Block(
		Switch(Id("field")).BlockFunc(func(f *Group) {
			for i, field := range struc.Fields {
				if field.IsAnonymous() {
					continue // TODO(andre) should this be an error?
				}

				fieldName := field.Name.MustGet()
				jsonName := fieldName
				for _, tag := range field.Tag.Tags() {
					if tag.Key == "json" {
						if tag.Name != "" {
							jsonName = tag.Name
						}
					}
				}

				rhs, returnType := cb.readType(field.Type, Lit(jsonName))
				f.Case(Lit(jsonName)).Block(Id("obj").Dot(fieldName).Op("=").Add(rhs))

				fieldTypes[i] = Id(fieldName).Add(returnType)

			}

			f.Default().Block(Id("itr").Dot("Skip").Call())
		}),
		Return(True()),
	))

	return Struct(fieldTypes...)
}

// resolveValueType resolves a config.Value[T] or config.Values[T] (or one of their aliases, like config.Bool)
// to the underlying type T.
func (cb *configUnmarshalersBuilder) resolveValueType(t schema.NamedType) (underlying schema.Type, isList bool) {
	if t.DeclInfo.Name == "Values" {
		if len(t.TypeArgs) == 0 {
			// Invalid use of config.Values[T]
			cb.errs.Add(t.AST.Pos(), "invalid use of config.Values[T]: no type arguments provided")
			return schema.BuiltinType{Kind: schema.Invalid}, true
		}

		return t.TypeArgs[0], true
	} else if t.DeclInfo.Name == "Value" {
		if len(t.TypeArgs) == 0 {
			// Invalid use of config.Value[T]
			cb.errs.Add(t.AST.Pos(), "invalid use of config.Value[T]: no type arguments provided")
			return schema.BuiltinType{Kind: schema.Invalid}, false
		}
		return t.TypeArgs[0], false
	} else {
		// Use of some helper type alias, like config.Bool
		decl := t.Decl()
		if named, ok := decl.Type.(schema.NamedType); ok && named.DeclInfo.Name == "Value" {
			if len(named.TypeArgs) == 0 {
				// Invalid use of config.Value[T]
				cb.errs.Add(t.AST.Pos(), "invalid use of config.Value[T]: no type arguments provided")
				return schema.BuiltinType{Kind: schema.Invalid}, false
			}
			return named.TypeArgs[0], false
		} else {
			// Invalid use of config.Value[T]
			cb.errs.Addf(t.AST.Pos(), "unrecognized config type: %s", t.DeclInfo.Name)
			return schema.BuiltinType{Kind: schema.Invalid}, false
		}
	}
}

// typeUnmarshalerFunc returns a `f` function which can be used to read the given value of `typ` and the type
// that function f returns.
//
// The returned function will either be an inline function or an identifier for function defined in the package
// and is expected to comply with the `config.Unmarshaler[T]` type defined in the runtime.
func (cb *configUnmarshalersBuilder) typeUnmarshalerFunc(typ schema.Type) (f *Statement, returnType *Statement) {
	switch t := typ.(type) {
	case schema.NamedType:
		name, returnType := cb.typeUnmarshalerName(t.Decl())

		if len(t.TypeArgs) == 0 {
			return Id(name), returnType
		} else {
			returnTypes := make([]Code, len(t.TypeArgs))
			call := CallFunc(func(f *Group) {
				for i, arg := range t.TypeArgs {
					funcToCall, returnType := cb.typeUnmarshalerFunc(arg)
					returnTypes[i] = returnType
					f.Add(funcToCall)
				}
			})

			f := Id(name).Types(returnTypes...).Add(call)

			return f, returnType.Types(returnTypes...)
		}
	default:
		unmarshaler, returnType := cb.readType(typ, nil)

		return Func().Params(
			Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
			Id("path").Index().String(),
		).Params(
			returnType,
		).Block(
			Return(unmarshaler),
		), returnType

	}
}

// typeUnmarshalerName returns a generated name for the unmarshaler function for the given type and the type that
// the decl is.
func (cb *configUnmarshalersBuilder) typeUnmarshalerName(decl *schema.TypeDecl) (reader string, rtnTyp *Statement) {
	return fmt.Sprintf("encoreInternalTypeConfigUnmarshaler_%s", decl.Name), Id(decl.Name)
}

// readBuiltin returns reader code for reading the built in type from `itr` (a `*jsonitor.Iterator`) and the
// Go type for the built in value.
func (cb *configUnmarshalersBuilder) readBuiltin(builtin schema.BuiltinKind) (reader Code, rtnTyp *Statement) {
	switch builtin {
	case schema.Bool:
		return Id("itr").Dot("ReadBool").Call(), Bool()
	case schema.Int:
		return Id("itr").Dot("ReadInt").Call(), Int()
	case schema.Int8:
		return Id("itr").Dot("ReadInt8").Call(), Int8()
	case schema.Int16:
		return Id("itr").Dot("ReadInt16").Call(), Int16()
	case schema.Int32:
		return Id("itr").Dot("ReadInt32").Call(), Int32()
	case schema.Int64:
		return Id("itr").Dot("ReadInt64").Call(), Int64()
	case schema.Uint:
		return Id("itr").Dot("ReadUint").Call(), Uint()
	case schema.Uint8:
		return Id("itr").Dot("ReadUint8").Call(), Uint8()
	case schema.Uint16:
		return Id("itr").Dot("ReadUint16").Call(), Uint16()
	case schema.Uint32:
		return Id("itr").Dot("ReadUint32").Call(), Uint32()
	case schema.Uint64:
		return Id("itr").Dot("ReadUint64").Call(), Uint64()
	case schema.Float32:
		return Id("itr").Dot("ReadFloat32").Call(), Float32()
	case schema.Float64:
		return Id("itr").Dot("ReadFloat64").Call(), Float64()
	case schema.String:
		return Id("itr").Dot("ReadString").Call(), String()
	case schema.Bytes, schema.Time, schema.UUID, schema.JSON, schema.UserID:
		var rtnTyp *Statement
		switch builtin {
		case schema.Bytes:
			rtnTyp = Index().Byte()
		case schema.Time:
			rtnTyp = Qual("time", "Time")
		case schema.UUID:
			rtnTyp = Qual("encore.dev/types/uuid", "UUID")
		case schema.JSON:
			rtnTyp = Qual("encoding/json", "RawMessage")
		case schema.UserID:
			rtnTyp = Qual("encore.dev/beta/auth", "UID")
		}

		return Func().Params().Params(Id("rtn").Add(rtnTyp)).BlockFunc(func(g *Group) {
			u := cb.gu.NewTypeUnmarshaller("decoder")
			g.Add(u.Init())
			g.Id("rtn").Op("=").Add(u.UnmarshalBuiltin(
				builtin,
				"value",
				Id("itr").Dot("ReadString").Call(),
				true,
			))
			g.If(Err().Op(":=").Add(u.Err()), Err().Op("!=").Nil()).Block(
				Panic(Qual("fmt", "Sprintf").Call(Lit("unable to decode the config: %v"), Err())),
			)
			g.Return()
		}), rtnTyp
	default:
		panic(fmt.Sprintf("unsupported builtin type: %v", builtin))
	}
}

// typeParamUnmarshalerName generates a name for an unmarshaler function given as a argument to a generic unmarshaler
// function.
func (cb *configUnmarshalersBuilder) typeParamUnmarshalerName(param schema.DeclTypeParam) string {
	return fmt.Sprintf("_%s_unmarshaler", param.Name)
}

// generateConcreteUnmarshaler generates a function that unmarshals a concrete type, taking into account any
// type arguments passed to the given type
func (cb *configUnmarshalersBuilder) generateConcreteUnmarshalers(typ schema.Type) {
	funcBody, _ := cb.typeUnmarshalerFunc(typ)
	funcName := ConfigUnmarshalFuncName(cb.gu, typ)

	cb.unmarshalers = append(cb.unmarshalers, Id(funcName).Op("=").Add(funcBody))
}

// ConfigUnmarshalFuncName returns a unique name for an unmarshal function fo a concrete
// instance of a type. For example the following types will result in the given names:
//
// - `int` -> `encoreInternal_LoadConfig_int`
// - `ConfigType` -> `encoreInternal_LoadConfig_ConfigType`
// - `ConfigType[int, string]` -> `encoreInternal_LoadConfig_ConfigType_int_string_`
func ConfigUnmarshalFuncName(gu *genutil.Helper, typ schema.Type) string {
	typeAsString := gu.TypeToString(typ)
	typeAsString = strings.NewReplacer(
		"*", "ptr_",
		"[", "_",
		"]", "_",
		",", "_",
		".", "_",
		" ", "",
		"\t", "",
		"\n", "",
	).Replace(typeAsString)

	return fmt.Sprintf("encoreInternalConfigUnmarshaler_%s", typeAsString)
}
