package genutil

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/schema"
)

// MarshalBuiltin generates the code to marshal a builtin type.
// The resulting code if an expression of type string.
func MarshalBuiltin(kind schema.BuiltinKind, value *Statement) Code {
	return Qual("encore.dev/appruntime/etype", "MarshalOne").Call(
		Qual("encore.dev/appruntime/etype", "Marshal"+builtinToName(kind)),
		value.Clone(),
	)
}

// MarshalBuiltinList generates the code to marshal a list of builtins.
// The resulting code is an expression of type []string.
func MarshalBuiltinList(kind schema.BuiltinKind, value *Statement) Code {
	return Qual("encore.dev/appruntime/etype", "MarshalList").Call(
		Qual("encore.dev/appruntime/etype", "Marshal"+builtinToName(kind)),
		value.Clone(),
	)
}

type TypeUnmarshaller struct {
	errs *perr.List

	// unmarshallerExpr is the code gen expression to resolve the unmarshaller.
	// It's typically just an identifier, but it can be more complex if need be.
	unmarshallerExpr *Statement
}

func (g *Generator) NewTypeUnmarshaller(objName string) *TypeUnmarshaller {
	expr := Id(objName)
	return &TypeUnmarshaller{errs: g.Errs, unmarshallerExpr: expr}
}

// UnmarshallerTypeName returns a type expression for the etype.Unmarshaller type
// in the runtime, in the form "*etype.Unmarshaller".
func UnmarshallerTypeName() *Statement {
	return Op("*").Qual("encore.dev/appruntime/etype", "Unmarshaller")
}

func (u *TypeUnmarshaller) Init() *Statement {
	return u.unmarshallerExpr.Clone().Op(":=").New(Qual("encore.dev/appruntime/etype", "Unmarshaller"))
}

func (u *TypeUnmarshaller) Err() *Statement {
	return u.unmarshallerExpr.Clone().Dot("Error")
}

func (u *TypeUnmarshaller) HasError() *Statement {
	return u.unmarshallerExpr.Clone().Dot("Error").Op("!=").Nil()
}

func (u *TypeUnmarshaller) UnmarshalBuiltin(kind schema.BuiltinKind, fieldName string, value *Statement, required bool) *Statement {
	return Qual("encore.dev/appruntime/etype", "UnmarshalOne").Call(
		u.unmarshallerExpr.Clone(),
		Qual("encore.dev/appruntime/etype", "Unmarshal"+builtinToName(kind)),
		Lit(fieldName),
		value.Clone(),
		Lit(required),
	)
}

// UnmarshalBuiltinList unmarshals a list of builtins.
func (u *TypeUnmarshaller) UnmarshalBuiltinList(kind schema.BuiltinKind, fieldName string, value *Statement, required bool) *Statement {
	return Qual("encore.dev/appruntime/etype", "UnmarshalList").Call(
		u.unmarshallerExpr.Clone(),
		Qual("encore.dev/appruntime/etype", "Unmarshal"+builtinToName(kind)),
		Lit(fieldName),
		value.Clone(),
		Lit(required),
	)
}

// UnmarshalSingleOrList returns the code to unmarshal a supported type.
// The type must be a builtin or a list of builtins.
func (u *TypeUnmarshaller) UnmarshalSingleOrList(typ schema.Type, fieldName string, singleValue, listOfValues *Statement, required bool) *Statement {
	if builtin, ok := typ.(schema.BuiltinType); ok {
		return u.UnmarshalBuiltin(builtin.Kind, fieldName, singleValue, required)
	} else if list, ok := typ.(schema.ListType); ok {
		if builtin, ok := list.Elem.(schema.BuiltinType); ok {
			return u.UnmarshalBuiltinList(builtin.Kind, fieldName, listOfValues, required)
		}
	}
	u.errs.Addf(typ.ASTExpr().Pos(), "cannot unmarshal string to type %s", typ)
	return Null()
}

// ReadBody returns an expression to read the full request body into a []byte.
func (u *TypeUnmarshaller) ReadBody(bodyExpr *Statement) *Statement {
	return u.unmarshallerExpr.Clone().Id("ReadBody").Call(bodyExpr.Clone())
}

// ParseJSON returns an expression to parse json.
// It uses the iterator accessed through the given iteratorExpr to parse JSON into the given dstExpr.
// The dstExpr must be a pointer value.
// The field name is only used for error reporting.
func (u *TypeUnmarshaller) ParseJSON(fieldName string, iteratorExpr *Statement, dstExpr *Statement) *Statement {
	return u.unmarshallerExpr.Clone().Id("ParseJSON").Call(Lit(fieldName), iteratorExpr, dstExpr)
}

// builtinToName returns the string name of the builtin.
//
// Each kind's name corresponds with the functions in etype.
// That is, if this function returns "Foo" it expects the etype package
// in the runtime to contain "MarshalFoo and UnmarshalFoo".
func builtinToName(kind schema.BuiltinKind) string {
	return kind.String()
}
