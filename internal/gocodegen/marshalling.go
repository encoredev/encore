package gocodegen

import (
	"strings"

	"github.com/cockroachdb/errors"
	. "github.com/dave/jennifer/jen"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

const (
	lastErrorField = "LastError"
)

// MarshallingCodeGenerator is used to generate a structure has methods for decoding various types, collecting the errors.
// It will only generate methods required for the given types.
type MarshallingCodeGenerator struct {
	structName          string
	used                bool
	encoreTypesAsString bool // true if  auth.UID and uuid.UUID should be treated as strings?

	builtins     []methodDescription
	seenBuiltins map[methodKey]methodDescription
}

type methodKey struct {
	fromString bool
	builtin    schema.Builtin
	slice      bool
}

type methodDescription struct {
	FromString bool
	Method     string
	Input      Code
	Result     Code
	IsList     bool
	Block      []Code
}

// MarshallingCodeWrapper is returned by NewPossibleInstance and tracks usage within a block
type MarshallingCodeWrapper struct {
	g            *MarshallingCodeGenerator
	instanceName string
	used         bool

	code []Code
}

func NewMarshallingCodeGenerator(structName string, forClientGen bool) *MarshallingCodeGenerator {
	return &MarshallingCodeGenerator{
		structName:          structName,
		builtins:            nil,
		seenBuiltins:        make(map[methodKey]methodDescription),
		encoreTypesAsString: forClientGen,
	}
}

// NewPossibleInstance Creates a statement to initialise a new encoding instance.
//
// Use the returned wrapper to convert FromStrings to the target types, adding any code you
// are generating to the wrapper using Add. Once you've finished generating all the code which
// may need type conversion with that _instance_ of the deserializer, call Finalize to generate the code full code
// including error handling.
//
// Once you've finished writing the whole app with all the code which uses this generator call WriteToFile to write
// the supporting struct and methods to the given file
func (g *MarshallingCodeGenerator) NewPossibleInstance(instanceName string) *MarshallingCodeWrapper {
	g.used = true
	return &MarshallingCodeWrapper{g: g, instanceName: instanceName}
}

// WriteToFile writes the full encoder type into the given file.
func (g *MarshallingCodeGenerator) WriteToFile(f *File) {
	if !g.used || len(g.builtins) == 0 {
		return
	}

	f.Commentf("%s is used to marshal requests to strings and unmarshal responses from strings", g.structName)
	f.Type().Id(g.structName).Struct(
		Id(lastErrorField).Error().Comment("The last error that occurred"),
	)

	for _, desc := range g.builtins {
		var params []Code
		if desc.FromString {
			params = []Code{Id("field").String(), Id("s").Add(desc.Input), Id("required").Bool()}
		} else {
			params = []Code{Id("s").Add(desc.Input)}
		}

		f.Func().Params(
			Id("e").Op("*").Id(g.structName),
		).Id(desc.Method).Params(params...).Params(Id("v").Add(desc.Result)).BlockFunc(func(g *Group) {
			if desc.FromString {
				// If we're dealing with a list of strings, we need to compare with len(s) == 0 instead of s == ""
				if desc.IsList {
					g.If(Op("!").Id("required").Op("&&").Len(Id("s")).Op("==").Lit(0)).Block(Return())
				} else {
					g.If(Op("!").Id("required").Op("&&").Id("s").Op("==").Lit("")).Block(Return())
				}
			}
			for _, s := range desc.Block {
				g.Add(s)
			}
		})
		f.Line()
	}

	f.Comment("setErr sets the last error within the object if one is not already set")
	f.Func().Params(Id("e").Op("*").Id(g.structName)).Id("setErr").Params(List(Id("msg"), Id("field")).String(), Err().Error()).Block(
		If(Err().Op("!=").Nil().Op("&&").Id("e").Dot(lastErrorField).Op("==").Nil()).Block(
			Id("e").Dot(lastErrorField).Op("=").Qual("fmt", "Errorf").Call(
				Lit("%s: %s: %w"),
				Id("field"),
				Id("msg"),
				Id("err"),
			),
		),
	)
	f.Line()
}

func (b *MarshallingCodeGenerator) builtinFromString(t schema.Builtin, slice bool) (string, error) {
	key := methodKey{builtin: t, slice: slice, fromString: true}
	if n, ok := b.seenBuiltins[key]; ok {
		return n.Method, nil
	} else if slice {
		k2 := methodKey{builtin: t, fromString: true}
		if _, err := b.builtinFromString(t, false); err != nil {
			return "", err
		}
		desc := b.seenBuiltins[k2]
		name := desc.Method + "List"
		fn := methodDescription{
			FromString: true,
			Method:     name,
			Input:      Index().String(),
			Result:     Index().Add(desc.Result),
			IsList:     true,
			Block: []Code{
				For(List(Id("_"), Id("x")).Op(":=").Range().Id("s")).Block(
					Id("v").Op("=").Append(Id("v"), Id("e").Dot(desc.Method).Call(Id("field"), Id("x"), Id("required"))),
				),
				Return(Id("v")),
			},
		}
		b.seenBuiltins[key] = fn
		b.builtins = append(b.builtins, fn)
		return fn.Method, nil
	}

	var fn methodDescription
	switch t {
	case schema.Builtin_STRING:
		fn = methodDescription{true, "ToStringSlice", String(), String(), false, []Code{Return(Id("s"))}}
	case schema.Builtin_BYTES:
		fn = methodDescription{true, "ToBytes", String(), Index().Byte(), false, []Code{
			List(Id("v"), Err()).Op(":=").Qual("encoding/base64", "URLEncoding").Dot("DecodeString").Call(Id("s")),
			Id("e").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_BOOL:
		fn = methodDescription{true, "ToBool", String(), Bool(), false, []Code{
			List(Id("v"), Err()).Op(":=").Qual("strconv", "ParseBool").Call(Id("s")),
			Id("e").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_UUID:
		fn = methodDescription{true, "ToUUID", String(), Qual("encore.dev/types/uuid", "UUID"), false, []Code{
			List(Id("v"), Err()).Op(":=").Qual("encore.dev/types/uuid", "FromString").Call(Id("s")),
			Id("e").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_TIME:
		fn = methodDescription{true, "ToTime", String(), Qual("time", "Time"), false, []Code{
			List(Id("v"), Err()).Op(":=").Qual("time", "Parse").Call(Qual("time", "RFC3339"), Id("s")),
			Id("e").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_USER_ID:
		fn = methodDescription{true, "ToUserID", String(), Qual("encore.dev/beta/auth", "UID"), false, []Code{
			Return(Qual("encore.dev/beta/auth", "UID").Call(Id("s"))),
		}}
	case schema.Builtin_JSON:
		fn = methodDescription{true, "ToJSON", String(), Qual("encoding/json", "RawMessage"), false, []Code{
			Return(Qual("encoding/json", "RawMessage").Call(Id("s"))),
		}}
	default:
		type kind int
		const (
			unsigned kind = iota + 1
			signed
			float
		)
		numTypes := map[schema.Builtin]struct {
			typ  string
			kind kind
			bits int
		}{
			schema.Builtin_INT8:    {"int8", signed, 8},
			schema.Builtin_INT16:   {"int16", signed, 16},
			schema.Builtin_INT32:   {"int32", signed, 32},
			schema.Builtin_INT64:   {"int64", signed, 64},
			schema.Builtin_INT:     {"int", signed, 64},
			schema.Builtin_UINT8:   {"uint8", unsigned, 8},
			schema.Builtin_UINT16:  {"uint16", unsigned, 16},
			schema.Builtin_UINT32:  {"uint32", unsigned, 32},
			schema.Builtin_UINT64:  {"uint64", unsigned, 64},
			schema.Builtin_UINT:    {"uint", unsigned, 64},
			schema.Builtin_FLOAT64: {"float64", float, 64},
			schema.Builtin_FLOAT32: {"float32", float, 32},
		}

		def, ok := numTypes[t]
		if !ok {
			return "", errors.Newf("unsupported type: %s", t)
		}

		cast := def.typ != "int64" && def.typ != "uint64" && def.typ != "float64"
		var err error
		fn = methodDescription{true, "To" + strings.Title(def.typ), String(), Id(def.typ), false, []Code{
			List(Id("x"), Err()).Op(":=").Do(func(s *Statement) {
				switch def.kind {
				case unsigned:
					s.Qual("strconv", "ParseUint").Call(Id("s"), Lit(10), Lit(def.bits))
				case signed:
					s.Qual("strconv", "ParseInt").Call(Id("s"), Lit(10), Lit(def.bits))
				case float:
					s.Qual("strconv", "ParseFloat").Call(Id("s"), Lit(def.bits))
				default:
					err = errors.Newf("unknown kind %v", def.kind)
				}
			}),
			Id("e").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			ReturnFunc(func(g *Group) {
				if cast {
					g.Id(def.typ).Call(Id("x"))
				} else {
					g.Id("x")
				}
			}),
		}}
		if err != nil {
			return "", err
		}
	}

	b.seenBuiltins[key] = fn
	b.builtins = append(b.builtins, fn)
	return fn.Method, nil
}

func (b *MarshallingCodeGenerator) builtinToString(t schema.Builtin, slice bool) (string, error) {
	key := methodKey{builtin: t, slice: slice, fromString: false}
	if fn, ok := b.seenBuiltins[key]; ok {
		return fn.Method, nil
	}

	if slice {
		k2 := methodKey{builtin: t, fromString: false}
		if _, err := b.builtinToString(t, false); err != nil {
			return "", err
		}
		desc := b.seenBuiltins[k2]
		name := desc.Method + "List"
		fn := methodDescription{
			FromString: false,
			Method:     name,
			Input:      Index().Add(desc.Input),
			Result:     Index().String(),
			IsList:     true,
			Block: []Code{
				For(List(Id("_"), Id("x")).Op(":=").Range().Id("s")).Block(
					Id("v").Op("=").Append(Id("v"), Id("e").Dot(desc.Method).Call(Id("x"))),
				),
				Return(Id("v")),
			},
		}
		b.seenBuiltins[key] = fn
		b.builtins = append(b.builtins, fn)
		return fn.Method, nil
	}

	var fn methodDescription
	switch t {
	case schema.Builtin_STRING:
		fn = methodDescription{false, "FromString", String(), String(), false, []Code{Return(Id("s"))}}
	case schema.Builtin_BYTES:
		fn = methodDescription{false, "FromBytes", Index().Byte(), String(), false, []Code{
			Return(Qual("encoding/base64", "URLEncoding").Dot("EncodeToString").Call(Id("s"))),
		}}
	case schema.Builtin_BOOL:
		fn = methodDescription{false, "FromBool", Bool(), String(), false, []Code{
			Return(Qual("strconv", "FormatBool").Call(Id("s"))),
		}}
	case schema.Builtin_UUID:
		fn = methodDescription{false, "FromUUID", Qual("encore.dev/types/uuid", "UUID"), String(), false, []Code{
			Return(Id("s").Dot("String").Call()),
		}}
	case schema.Builtin_TIME:
		fn = methodDescription{false, "FromTime", Qual("time", "Time"), String(), false, []Code{
			Return(Id("s").Dot("Format").Call(Qual("time", "RFC3339"))),
		}}
	case schema.Builtin_USER_ID:
		fn = methodDescription{false, "FromUserID", Qual("encore.dev/beta/auth", "UID"), String(), false, []Code{
			Return(String().Call(Id("s"))),
		}}
	case schema.Builtin_JSON:
		fn = methodDescription{false, "FromJSON", Qual("encoding/json", "RawMessage"), String(), false, []Code{
			Return(String().Call(Id("s"))),
		}}
	default:
		type kind int
		const (
			unsigned kind = iota + 1
			signed
			float
		)
		numTypes := map[schema.Builtin]struct {
			typ     string
			castTyp string
			kind    kind
			bits    int
		}{
			schema.Builtin_INT8:    {"int8", "int64", signed, 8},
			schema.Builtin_INT16:   {"int16", "int64", signed, 16},
			schema.Builtin_INT32:   {"int32", "int64", signed, 32},
			schema.Builtin_INT64:   {"int64", "int64", signed, 64},
			schema.Builtin_INT:     {"int", "int64", signed, 64},
			schema.Builtin_UINT8:   {"uint8", "uint64", unsigned, 8},
			schema.Builtin_UINT16:  {"uint16", "uint64", unsigned, 16},
			schema.Builtin_UINT32:  {"uint32", "uint64", unsigned, 32},
			schema.Builtin_UINT64:  {"uint64", "uint64", unsigned, 64},
			schema.Builtin_UINT:    {"uint", "uint64", unsigned, 64},
			schema.Builtin_FLOAT64: {"float64", "float64", float, 64},
			schema.Builtin_FLOAT32: {"float32", "float64", float, 32},
		}

		def, ok := numTypes[t]
		if !ok {
			return "", errors.Newf("unsupported type: %s", t)
		}

		var err error
		fn = methodDescription{false, "From" + strings.Title(def.typ), Id(def.typ), String(), false, []Code{
			Return(Do(func(s *Statement) {
				id := Id("s")
				if def.typ != def.castTyp {
					id = Id(def.castTyp).Call(id)
				}

				switch def.kind {
				case unsigned:
					s.Qual("strconv", "FormatUint").Call(id, Lit(10))
				case signed:
					s.Qual("strconv", "FormatInt").Call(id, Lit(10))
				case float:
					s.Qual("strconv", "FormatFloat").Call(id, Lit(byte('f')), Lit(-1), Lit(def.bits))
				default:
					err = errors.Newf("unknown kind %v", def.kind)
				}
			})),
		}}
		if err != nil {
			return "", err
		}
	}

	b.seenBuiltins[key] = fn
	b.builtins = append(b.builtins, fn)
	return fn.Method, nil
}

func (w *MarshallingCodeWrapper) LastError() Code {
	return Id(w.instanceName).Dot(lastErrorField)
}

// Add adds code into the wrapped block
func (w *MarshallingCodeWrapper) Add(c ...Code) {
	w.code = append(w.code, c...)
}

// Finalize returns the final code block including all wrapped code
func (w *MarshallingCodeWrapper) Finalize(ifErrorBlock ...Code) []Code {
	if !w.used {
		return w.code
	}

	code := []Code{Id(w.instanceName).Op(":=").Op("&").Id(w.g.structName).Values(), Line()}
	code = append(code, w.code...)
	code = append(code, If(Id(w.instanceName).Dot(lastErrorField).Op("!=").Nil()).Block(ifErrorBlock...), Line())
	return code
}

func (g *MarshallingCodeGenerator) shouldBeTreatedAsString(builtin schema.Builtin) bool {
	return builtin == schema.Builtin_STRING ||
		(g.encoreTypesAsString && builtin == schema.Builtin_UUID) ||
		(g.encoreTypesAsString && builtin == schema.Builtin_USER_ID)
}

// FromString will return either the original string or a call to the encoder
func (w *MarshallingCodeWrapper) FromString(targetType *schema.Type, fieldName string, getAsString Code, getAsStringSlice Code, required bool) (code Code, err error) {
	// get the method name for the target type
	funcName := ""
	srcCode := getAsString
	switch t := targetType.Typ.(type) {
	case *schema.Type_List:
		if bt, ok := t.List.Elem.Typ.(*schema.Type_Builtin); ok {
			// If the list is strings, we can just return the slice
			if w.g.shouldBeTreatedAsString(bt.Builtin) {
				return getAsStringSlice, nil
			}

			funcName, err = w.g.builtinFromString(bt.Builtin, true)
			srcCode = getAsStringSlice
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.Newf("unsupported list type %T", t.List.Elem.Typ)
		}
	case *schema.Type_Builtin:
		// If the list is strings, we can just return the slice
		if w.g.shouldBeTreatedAsString(t.Builtin) {
			return getAsString, nil
		}

		funcName, err = w.g.builtinFromString(t.Builtin, false)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Newf("unsupported type for deserialization: %T", t)
	}

	// mark this code wrapper as actually using the deserializer type
	w.used = true
	return Id(w.instanceName).Dot(funcName).Call(Lit(fieldName), srcCode, Lit(required)), nil
}

// ToStringSlice will return either the original string or a call to the encoder
func (w *MarshallingCodeWrapper) ToStringSlice(sourceType *schema.Type, sourceValue Code) (code Code, err error) {
	// get the method name for the target type
	funcName := ""
	switch t := sourceType.Typ.(type) {
	case *schema.Type_List:
		if bt, ok := t.List.Elem.Typ.(*schema.Type_Builtin); ok {
			// If the list is strings, we can just return the slice
			if w.g.shouldBeTreatedAsString(bt.Builtin) {
				return sourceValue, nil
			}

			funcName, err = w.g.builtinToString(bt.Builtin, true)
			if err != nil {
				return nil, err
			}

			w.used = true
			return Id(w.instanceName).Dot(funcName).Call(sourceValue), nil
		} else {
			return nil, errors.Newf("unsupported list type %T", t.List.Elem.Typ)
		}
	case *schema.Type_Builtin:
		// If the list is strings, we can just return the slice
		if w.g.shouldBeTreatedAsString(t.Builtin) {
			return Values(sourceValue), nil
		}

		funcName, err = w.g.builtinToString(t.Builtin, false)
		if err != nil {
			return nil, err
		}

		w.used = true
		return Values(Id(w.instanceName).Dot(funcName).Call(sourceValue)), nil
	default:
		return nil, errors.Newf("unsupported type for deserialization: %T", t)
	}
}
