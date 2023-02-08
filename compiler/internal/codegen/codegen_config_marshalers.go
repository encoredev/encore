package codegen

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"

	. "github.com/dave/jennifer/jen"
)

// ConfigUnmarshalers returns a generated Go file with all the code needed for a service to unmarshal
// its config.
func (b *Builder) ConfigUnmarshalers(svc *est.Service) (f *File, err error) {
	builder := &configUnmarshalersBuilder{
		svc:                  svc,
		f:                    NewFilePathName(svc.Root.ImportPath, svc.Name),
		marshaller:           b.marshaller,
		md:                   b.res.Meta,
		concreteUnmarshalers: make([]*Statement, 0),
	}

	builder.f.ImportAlias("github.com/json-iterator/go", "jsoniter")

	builder.f.Comment(`These functions are automatically generated and maintained by Encore to allow config values
to be unmarshalled into the correct types. They are not intended to be used directly. They
are automatically updated by Encore whenever you change the data types used within your
calls to config.Load[T]().`)

	// Find all the types to write, and then write unmarshalers for them
	typesToWrite, err := builder.FindAllTypes()
	if err != nil {
		return nil, err
	}

	// Write load functions for generic types
	for _, load := range svc.ConfigLoads {
		builder.generateConcreteUnmarshalers(load.ConfigStruct)
	}
	if len(builder.concreteUnmarshalers) > 0 {
		builder.f.Line()
		builder.f.Comment("// Concrete unmarshalers for all config.Load calls, including those using generic types.")
		builder.f.Comment("// These instances are used directly by calls to `config.Load[T]()`.")
		builder.f.Var().DefsFunc(func(f *Group) {
			for _, typ := range builder.concreteUnmarshalers {
				f.Add(typ)
			}
		})
	}

	// Write unmarshalers for all the types needed for config
	for _, id := range typesToWrite {
		if err := builder.WriteTypeUnmarshaler(id); err != nil {
			return nil, err
		}
	}

	return builder.f, nil
}

type configUnmarshalersBuilder struct {
	f                    *File
	svc                  *est.Service
	md                   *meta.Data
	marshaller           *gocodegen.MarshallingCodeGenerator
	concreteUnmarshalers []*Statement
}

// FindAllTypes returns a list of all decl's used within a services config.
func (cb *configUnmarshalersBuilder) FindAllTypes() ([]uint32, error) {
	typesToWrite := make(map[uint32]struct{})

	// Walk the config load calls in this service and find all the named types used
	for _, load := range cb.svc.ConfigLoads {
		err := schema.Walk(cb.md.Decls, load.ConfigStruct.Type, func(node any) error {
			switch n := node.(type) {
			case *schema.Named:
				typesToWrite[n.Id] = struct{}{}
			}
			return nil
		})
		if err != nil {
			return nil, eerror.Wrap(err, "codegen", "error walking config struct", nil)
		}
	}

	// Now convert the list of types into a sorted list of nodes
	types := make([]uint32, 0, len(typesToWrite))
	for id := range typesToWrite {
		types = append(types, id)
	}
	slices.Sort(types)

	return types, nil
}

// WriteTypeUnmarshaler writes a function which will unmarshal the given decl from JSON to the instance of the
// Go type. If the decl takes type parameters, then the function generated will also be generic and will require
// unmarshalers for the type parameters to be passed in.
func (cb *configUnmarshalersBuilder) WriteTypeUnmarshaler(id uint32) (err error) {
	decl := cb.md.Decls[id]

	unmarshalerName, _ := cb.typeUnmarshalerName(id)

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
		struc := decl.Type.GetStruct()
		if struc == nil {
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
func (cb *configUnmarshalersBuilder) readType(typ *schema.Type, pathElement Code) (reader Code, rtnTyp *Statement) {
	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		funcRef, returnType := cb.typeUnmarshalerFunc(typ)

		return funcRef.Call(
			Id("itr"),
			Append(Id("path"), pathElement),
		), returnType

	case *schema.Type_Struct:
		var returnType = Nil()
		block := BlockFunc(func(f *Group) {
			returnType = cb.readStruct(f, t.Struct)
			f.Return()
		})
		return Func().Params().Params(Id("obj").Add(returnType)).Add(block).Call(), returnType

	case *schema.Type_Map:
		_, keyType := cb.readType(t.Map.Key, nil)
		valueUnmarshaler, valueType := cb.readType(t.Map.Value, Id("keyAsString"))
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
					instance := cb.marshaller.NewPossibleInstance("keyDecoder")
					code, err := instance.FromString(t.Map.Key, "keyAsString", Id("keyAsString"), Index().String().List(Id("key")), true)
					if err != nil {
						panic(err)
					}
					instance.Add(Id("key").Op(":=").Add(code))
					f.Add(instance.Finalize(
						Panic(Qual("fmt", "Sprintf").Call(Lit("unable to decode the config: %v"), instance.LastError())),
					)...)

					// Then we can just return the key and value
					f.Return(Id("key"), valueUnmarshaler)
				}),
			),
			rtnType

	case *schema.Type_List:
		unmarshaler, returnType := cb.readType(t.List.Elem, Qual("strconv", "Itoa").Call(Id("idx")))

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

	case *schema.Type_Builtin:
		return cb.readBuiltin(t.Builtin)

	case *schema.Type_Pointer:
		reader, returnType := cb.readType(t.Pointer.Base, pathElement)

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

	case *schema.Type_Config:
		// The config type is the dynamic values which can be changed at runtime
		// by unit tests
		if t.Config.IsValuesList {
			code, _ := cb.readType(t.Config.Elem, pathElement)
			_, returnType := cb.readType(t.Config.Elem.GetList().Elem, pathElement)
			return Qual("encore.dev/config", "CreateValueList").Call(code, Append(Id("path"), pathElement)), Qual("encore.dev/config", "Values").Types(returnType)
		} else {
			code, returnType := cb.readType(t.Config.Elem, pathElement)
			return Qual("encore.dev/config", "CreateValue").Types(returnType).Call(code, Append(Id("path"), pathElement)), Qual("encore.dev/config", "Value").Types(returnType)
		}

	case *schema.Type_TypeParameter:
		typeParam := cb.md.Decls[t.TypeParameter.DeclId].TypeParams[t.TypeParameter.ParamIdx]
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
func (cb *configUnmarshalersBuilder) readStruct(f *Group, struc *schema.Struct) (returnType *Statement) {
	fieldTypes := make([]Code, len(struc.Fields))

	f.Id("itr").Dot("ReadObjectCB").Call(Func().Params(
		Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
		Id("field").String(),
	).Bool().Block(
		Switch(Id("field")).BlockFunc(func(f *Group) {
			for i, field := range struc.Fields {
				jsonName := field.Name
				for _, tag := range field.Tags {
					if tag.Key == "json" {
						if tag.Name != "" {
							jsonName = tag.Name
						}
					}
				}

				rhs, returnType := cb.readType(field.Typ, Lit(jsonName))
				f.Case(Lit(jsonName)).Block(Id("obj").Dot(field.Name).Op("=").Add(rhs))

				fieldTypes[i] = Id(field.Name).Add(returnType)

			}

			f.Default().Block(Id("itr").Dot("Skip").Call())
		}),
		Return(True()),
	))

	return Struct(fieldTypes...)
}

// typeUnmarshalerFunc returns a `f` function which can be used to read the given value of `typ` and the type
// that function f returns.
//
// The returned function will either be an inline function or an identifier for function defined in the package
// and is expected to comply with the `config.Unmarshaler[T]` type defined in the runtime.
func (cb *configUnmarshalersBuilder) typeUnmarshalerFunc(typ *schema.Type) (f *Statement, returnType *Statement) {
	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		name, returnType := cb.typeUnmarshalerName(t.Named.Id)

		if len(t.Named.TypeArguments) == 0 {
			return Id(name), returnType
		} else {
			returnTypes := make([]Code, len(t.Named.TypeArguments))
			call := CallFunc(func(f *Group) {
				for i, arg := range t.Named.TypeArguments {
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
func (cb *configUnmarshalersBuilder) typeUnmarshalerName(id uint32) (reader string, rtnTyp *Statement) {
	return fmt.Sprintf("encoreInternalTypeConfigUnmarshaler_%s", cb.md.Decls[id].Name), Id(cb.md.Decls[id].Name)
}

// readBuiltin returns reader code for reading the built in type from `itr` (a `*jsonitor.Iterator`) and the
// Go type for the built in value.
func (cb *configUnmarshalersBuilder) readBuiltin(builtin schema.Builtin) (reader Code, rtnTyp *Statement) {
	switch builtin {
	case schema.Builtin_BOOL:
		return Id("itr").Dot("ReadBool").Call(), Bool()
	case schema.Builtin_INT8:
		return Id("itr").Dot("ReadInt8").Call(), Int8()
	case schema.Builtin_INT16:
		return Id("itr").Dot("ReadInt16").Call(), Int16()
	case schema.Builtin_INT32:
		return Id("itr").Dot("ReadInt32").Call(), Int32()
	case schema.Builtin_INT64:
		return Id("itr").Dot("ReadInt64").Call(), Int64()
	case schema.Builtin_UINT8:
		return Id("itr").Dot("ReadUint8").Call(), Uint8()
	case schema.Builtin_UINT16:
		return Id("itr").Dot("ReadUint16").Call(), Uint16()
	case schema.Builtin_UINT32:
		return Id("itr").Dot("ReadUint32").Call(), Uint32()
	case schema.Builtin_UINT64:
		return Id("itr").Dot("ReadUint64").Call(), Uint64()
	case schema.Builtin_FLOAT32:
		return Id("itr").Dot("ReadFloat32").Call(), Float32()
	case schema.Builtin_FLOAT64:
		return Id("itr").Dot("ReadFloat64").Call(), Float64()
	case schema.Builtin_STRING:
		return Id("itr").Dot("ReadString").Call(), String()
	case schema.Builtin_BYTES, schema.Builtin_TIME, schema.Builtin_UUID, schema.Builtin_JSON, schema.Builtin_USER_ID:
		var rtnTyp *Statement
		switch builtin {
		case schema.Builtin_BYTES:
			rtnTyp = Index().Byte()
		case schema.Builtin_TIME:
			rtnTyp = Qual("time", "Time")
		case schema.Builtin_UUID:
			rtnTyp = Qual("encore.dev/types/uuid", "UUID")
		case schema.Builtin_JSON:
			rtnTyp = Qual("encoding/json", "RawMessage")
		case schema.Builtin_USER_ID:
			rtnTyp = Qual("encore.dev/beta/auth", "UID")
		}

		instance := cb.marshaller.NewPossibleInstance("decoder")
		code, err := instance.FromStringToBuiltin(
			builtin,
			"value",
			Id("itr").Dot("ReadString").Call(),
			true,
		)
		if err != nil {
			panic(fmt.Sprintf("unable to create builtin unmarshaler: %v", err))
		}
		instance.Add(Id("rtn").Op("=").Add(code))
		finalized := instance.Finalize(
			Panic(Qual("fmt", "Sprintf").Call(Lit("unable to decode the config: %v"), instance.LastError())),
		)

		finalized = append(finalized, Return())

		return Func().Params().Params(Id("rtn").Add(rtnTyp)).Block(finalized...).Call(), rtnTyp
	case schema.Builtin_INT:
		return Id("itr").Dot("ReadInt").Call(), Int()
	case schema.Builtin_UINT:
		return Id("itr").Dot("ReadUint").Call(), Uint()
	default:
		panic(fmt.Sprintf("unsupported builtin type: %v", builtin))
	}
}

// typeParamUnmarshalerName generates a name for an unmarshaler function given as a argument to a generic unmarshaler
// function.
func (cb *configUnmarshalersBuilder) typeParamUnmarshalerName(param *schema.TypeParameter) string {
	return fmt.Sprintf("_%s_unmarshaler", param.Name)
}

// generateConcreteUnmarshaler generates a function that unmarshals a concrete type, taking into account any
// type arguments passed to the given type
func (cb *configUnmarshalersBuilder) generateConcreteUnmarshalers(param *est.Param) {
	funcBody, _ := cb.typeUnmarshalerFunc(param.GetWithPointer())
	funcName := ConfigUnmarshalFuncName(param, cb.md)

	cb.concreteUnmarshalers = append(cb.concreteUnmarshalers, Id(funcName).Op("=").Add(funcBody))
}

// ConfigUnmarshalFuncName returns a unique name for an unmarshal function fo a concrete
// instance of a type. For example the following types will result in the given names:
//
// - `int` -> `encoreInternal_LoadConfig_int`
// - `ConfigType` -> `encoreInternal_LoadConfig_ConfigType`
// - `ConfigType[int, string]` -> `encoreInternal_LoadConfig_ConfigType_int_string_`
func ConfigUnmarshalFuncName(param *est.Param, md *meta.Data) string {
	typeAsString := gocodegen.ConvertSchemaTypeToString(param.GetWithPointer(), md)
	typeAsString = strings.NewReplacer(
		"*", "ptr_",
		"[", "_",
		"]", "_",
		",", "_",
		" ", "",
		"\t", "",
		"\n", "",
	).Replace(typeAsString)

	return fmt.Sprintf("encoreInternalConfigUnmarshaler_%s", typeAsString)
}
