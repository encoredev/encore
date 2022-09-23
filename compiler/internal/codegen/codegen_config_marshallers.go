package codegen

import (
	"fmt"

	"golang.org/x/exp/slices"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
	schema "encr.dev/proto/encore/parser/schema/v1"

	. "github.com/dave/jennifer/jen"
)

func (b *Builder) ConfigUnmarshallers(svc *est.Service) (f *File, err error) {
	builder := &configUnmarshallersBuilder{
		svc:        svc,
		f:          NewFilePathName(svc.Root.ImportPath, svc.Name),
		marshaller: b.marshaller,
		decls:      b.res.Meta.Decls,
	}

	builder.f.ImportAlias("github.com/json-iterator/go", "jsoniter")

	builder.f.Comment(`These functions are automatically generated and maintained by Encore to allow config values
to be unmarshalled into the correct types. They are not intended to be used directly. They
are automatically updated by Encore whenever you change the data types used within your
calls to config.Load[T]().`)

	// Find all the types to write, and then write unmarshallers for them
	typesToWrite, err := builder.FindAllTypes()
	if err != nil {
		return nil, err
	}

	for _, id := range typesToWrite {
		if err := builder.WriteTypeUnmarshaller(id); err != nil {
			return nil, err
		}
	}

	return builder.f, nil
}

type configUnmarshallersBuilder struct {
	f          *File
	svc        *est.Service
	decls      []*schema.Decl
	marshaller *gocodegen.MarshallingCodeGenerator
}

func (cb *configUnmarshallersBuilder) FindAllTypes() ([]uint32, error) {
	typesToWrite := make(map[uint32]struct{})

	// Walk the config load calls in this service and find all the named types used
	for _, load := range cb.svc.ConfigLoads {
		err := schema.Walk(cb.decls, load.ConfigStruct.Type, func(node any) error {
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

func (cb *configUnmarshallersBuilder) WriteTypeUnmarshaller(id uint32) (err error) {
	decl := cb.decls[id]

	unmarshallerName, _ := cb.typeUnmarshallerName(id)

	cb.f.Line()
	cb.f.Func().Id(unmarshallerName).Params(
		Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
		Id("path").Index().String(),
	).Params(
		Id("obj").Id(decl.Name),
	).BlockFunc(func(f *Group) {
		struc := decl.Type.GetStruct()
		if struc == nil {
			err = eerror.New("codegen", "can only unmarshal struct types", nil)
			return
		}
		cb.readStruct(f, struc)
		f.Return()
	})

	return
}

func (cb *configUnmarshallersBuilder) readType(typ *schema.Type, pathElement Code) (reader Code, rtnTyp Code) {
	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		functionName, returnType := cb.typeUnmarshallerName(t.Named.Id)
		return Id(functionName).Call(
			Id("itr"),
			Append(Id("path"), pathElement),
		), returnType

	case *schema.Type_Struct:
		var returnType Code = Nil()
		block := BlockFunc(func(f *Group) {
			returnType = cb.readStruct(f, t.Struct)
			f.Return()
		})
		return Func().Params().Params(Id("obj").Add(returnType)).Add(block).Call(), returnType

	case *schema.Type_Map:
		_, keyType := cb.readType(t.Map.Key, nil)
		valueUnmarshaller, valueType := cb.readType(t.Map.Value, Id("keyAsString"))
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
					// the etype unmarshaller to unmarshal the key to the underlying datatype
					f.Comment("Decode the map key from the JSON string to the underlying type it needs to be")
					instance := cb.marshaller.NewPossibleInstance("keyDecoder")
					code, err := instance.FromString(t.Map.Key, "keyAsString", Id("key"), Index().String().List(Id("key")), true)
					if err != nil {
						panic(err)
					}
					instance.Add(Id("key").Op(":=").Add(code))
					f.Add(instance.Finalize(
						Panic(Qual("fmt", "Sprintf").Call(Lit("unable to decode the config: %v"), instance.LastError())),
					)...)

					// Then we can just return the key and value
					f.Return(Id("key"), valueUnmarshaller)
				}),
			),
			rtnType

	case *schema.Type_List:
		unmarshaller, returnType := cb.readType(t.List.Elem, Qual("strconv", "Itoa").Call(Id("idx")))

		// We'll call a helper method in the runtime package, which requires us to pass
		// an unmarshaller for the list type and builds the return slice
		return Qual("encore.dev/config", "ReadArray").Types(returnType).Call(
				Id("itr"),
				Func().Params(
					Id("itr").Op("*").Qual("github.com/json-iterator/go", "Iterator"),
					Id("idx").Int(),
				).Add(returnType).Block(
					Return(unmarshaller),
				),
			),
			Index().Add(returnType)

	case *schema.Type_Builtin:
		return cb.readBuiltin(t.Builtin)

	case *schema.Type_Config:
		// The config type is the dynamic values which can be changed at runtime
		// by unit tests
		code, returnType := cb.readType(t.Config.Elem, pathElement)
		if t.Config.IsValuesList {
			return Qual("encore.dev/config", "CreateValueList").Call(code, Append(Id("path"), pathElement)), returnType
		} else {
			return Qual("encore.dev/config", "CreateValue").Types(returnType).Call(code, Append(Id("path"), pathElement)), returnType
		}
	default:
		return Nil().Comment("FIXME: This needs to be implemented"), Any() // FIXME implement all types
	}
}

func (cb *configUnmarshallersBuilder) readStruct(f *Group, struc *schema.Struct) (returnType Code) {
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
		}),
		Return(True()),
	))

	return Struct(fieldTypes...)
}

func (cb *configUnmarshallersBuilder) typeUnmarshallerName(id uint32) (reader string, rtnTyp Code) {
	return fmt.Sprintf("EncoreInternal_UnmarshalConfig_%s", cb.decls[id].Name), Id(cb.decls[id].Name)
}

func (cb *configUnmarshallersBuilder) readBuiltin(builtin schema.Builtin) (reader Code, rtnTyp Code) {
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
			panic(fmt.Sprintf("unable to create builtin unmarshaller: %v", err))
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
