package genutil

import (
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
)

// MarshalBuiltin generates the code to marshal a builtin type.
// The resulting code if an expression of type string.
func MarshalBuiltin(kind schema.BuiltinKind, value *Statement) Code {
	return Qual("encore.dev/appruntime/shared/etype", "MarshalOne").Call(
		Qual("encore.dev/appruntime/shared/etype", "Marshal"+builtinToName(kind)),
		value.Clone(),
	)
}

// MarshalBuiltinList generates the code to marshal a list of builtins.
// The resulting code is an expression of type []string.
func MarshalBuiltinList(kind schema.BuiltinKind, value *Statement) Code {
	return Qual("encore.dev/appruntime/shared/etype", "MarshalList").Call(
		Qual("encore.dev/appruntime/shared/etype", "Marshal"+builtinToName(kind)),
		value.Clone(),
	)
}

// MarshalQueryOrHeader generates the code to marshal a supported query value.
// The resulting code is an expression of type []string.
func MarshalQueryOrHeader(typ schema.Type, value *Statement) (code Code, ok bool) {
	if named, ok := typ.(schema.NamedType); ok {
		decl := named.Decl()
		if decl != nil && decl.Type != nil {
			return MarshalQueryOrHeader(decl.Type, castToUnderlying(decl.Type, value))
		}
		return nil, false
	}

	if list, ok := typ.(schema.ListType); ok {
		marshaller, ok := getQueryOrHeaderMarshaller(list.Elem)
		if !ok {
			return nil, false
		}

		return Qual("encore.dev/appruntime/shared/etype", "MarshalList").Call(
			marshaller,
			value.Clone(),
		), true
	}

	marshaller, ok := getQueryOrHeaderMarshaller(typ)
	if !ok {
		return nil, false
	}

	return Qual("encore.dev/appruntime/shared/etype", "MarshalOneAsList").Call(
		marshaller,
		value.Clone(),
	), true
}

// castToUnderlying casts a value to its underlying builtin type if needed.
func castToUnderlying(underlying schema.Type, value *Statement) *Statement {
	if b, ok := underlying.(schema.BuiltinType); ok {
		return builtinTypeExpr(b.Kind).Call(value.Clone())
	}
	return value.Clone()
}

func builtinTypeExpr(kind schema.BuiltinKind) *Statement {
	switch kind {
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
	default:
		return Id(strings.ToLower(kind.String()))
	}
}

func getQueryOrHeaderMarshaller(typ schema.Type) (s *Statement, ok bool) {
	switch typ := typ.(type) {
	case schema.BuiltinType:
		return Qual("encore.dev/appruntime/shared/etype", "Marshal"+builtinToName(typ.Kind)), true
	case schema.OptionType:
		inner, ok := getQueryOrHeaderMarshaller(typ.Value)
		return Qual("encore.dev/appruntime/shared/etype", "OptionMarshaller").Call(
			inner,
		), ok
	case schema.NamedType:
		if decl := typ.Decl(); decl != nil && decl.Type != nil {
			return getQueryOrHeaderMarshaller(decl.Type)
		}
		return Nil(), false
	default:
		return Nil(), false
	}
}

type TypeUnmarshaller struct {
	errs *perr.List

	// unmarshallerExpr is the code gen expression to resolve the unmarshaller.
	// It's typically just an identifier, but it can be more complex if need be.
	unmarshallerExpr *Statement
}

func (g *Helper) NewTypeUnmarshaller(objName string) *TypeUnmarshaller {
	expr := Id(objName)
	return &TypeUnmarshaller{errs: g.Errs, unmarshallerExpr: expr}
}

// UnmarshallerTypeName returns a type expression for the etype.Unmarshaller type
// in the runtime, in the form "*etype.Unmarshaller".
func UnmarshallerTypeName() *Statement {
	return Op("*").Qual("encore.dev/appruntime/shared/etype", "Unmarshaller")
}

func (u *TypeUnmarshaller) Init() *Statement {
	return u.unmarshallerExpr.Clone().Op(":=").New(Qual("encore.dev/appruntime/shared/etype", "Unmarshaller"))
}

func (u *TypeUnmarshaller) Err() *Statement {
	return u.unmarshallerExpr.Clone().Dot("Error")
}

func (u *TypeUnmarshaller) HasError() *Statement {
	return u.unmarshallerExpr.Clone().Dot("Error").Op("!=").Nil()
}

// NumNonEmptyValues returns an integer expression that reports
// the number of non-empty values the unmarshaller has processed.
func (u *TypeUnmarshaller) NumNonEmptyValues() *Statement {
	return u.unmarshallerExpr.Clone().Dot("NonEmptyValues")
}

// IncNonEmpty returns a statement to increment the number of
// non-empty values the unmarshaller has processed.
func (u *TypeUnmarshaller) IncNonEmpty() *Statement {
	return u.unmarshallerExpr.Clone().Dot("IncNonEmpty").Call()
}

func (u *TypeUnmarshaller) UnmarshalBuiltin(kind schema.BuiltinKind, fieldName string, value *Statement, required bool) *Statement {
	return Qual("encore.dev/appruntime/shared/etype", "UnmarshalOne").Call(
		u.unmarshallerExpr.Clone(),
		Qual("encore.dev/appruntime/shared/etype", "Unmarshal"+builtinToName(kind)),
		Lit(fieldName),
		value.Clone(),
		Lit(required),
	)
}

// UnmarshalQueryOrHeader returns the code to unmarshal a supported type.
func (u *TypeUnmarshaller) UnmarshalQueryOrHeader(typ schema.Type, fieldName string, singleValue, listOfValues *Statement) *Statement {
	if !schemautil.IsValidHeaderType(typ) {
		u.errs.Addf(typ.ASTExpr().Pos(), "cannot unmarshal string to type %s", typ)
		return Null()
	}

	if named, ok := typ.(schema.NamedType); ok {
		decl := named.Decl()
		if decl != nil && decl.Type != nil {
			innerExpr := u.UnmarshalQueryOrHeader(decl.Type, fieldName, singleValue, listOfValues)
			if innerExpr == nil || innerExpr == Null() {
				return Null()
			}
			return Q(named.DeclInfo).Call(innerExpr)
		}
		return Null()
	}

	if list, ok := typ.(schema.ListType); ok {
		return Qual("encore.dev/appruntime/shared/etype", "UnmarshalList").Call(
			u.unmarshallerExpr.Clone(),
			u.getQueryOrHeaderUnmarshaller(list.Elem),
			Lit(fieldName),
			listOfValues,
			Lit(false), // not required
		)
	}

	return Qual("encore.dev/appruntime/shared/etype", "UnmarshalOne").Call(
		u.unmarshallerExpr.Clone(),
		u.getQueryOrHeaderUnmarshaller(typ),
		Lit(fieldName),
		singleValue,
		Lit(false), // not required
	)
}

func (u *TypeUnmarshaller) getQueryOrHeaderUnmarshaller(typ schema.Type) *Statement {
	switch typ := typ.(type) {
	case schema.BuiltinType:
		return Qual("encore.dev/appruntime/shared/etype", "Unmarshal"+builtinToName(typ.Kind))
	case schema.OptionType:
		return Qual("encore.dev/appruntime/shared/etype", "OptionUnmarshaller").Call(
			u.getQueryOrHeaderUnmarshaller(typ.Value),
		)
	case schema.NamedType:
		if decl := typ.Decl(); decl != nil && decl.Type != nil {
			return u.getQueryOrHeaderUnmarshaller(decl.Type)
		}
		u.errs.Addf(typ.ASTExpr().Pos(), "cannot unmarshal string to type %s", typ)
		return Null()
	default:
		u.errs.Addf(typ.ASTExpr().Pos(), "cannot unmarshal string to type %s", typ)
		return Null()
	}
}

// ReadBody returns an expression to read the full request body into a []byte.
func (u *TypeUnmarshaller) ReadBody(bodyExpr *Statement) *Statement {
	return u.unmarshallerExpr.Clone().Dot("ReadBody").Call(bodyExpr.Clone())
}

// ParseJSON returns an expression to parse json.
// It uses the iterator accessed through the given iteratorExpr to parse JSON into the given dstExpr.
// The dstExpr must be a pointer value.
// The field name is only used for error reporting.
func (u *TypeUnmarshaller) ParseJSON(fieldName string, iteratorExpr *Statement, dstExpr *Statement) *Statement {
	return u.unmarshallerExpr.Clone().Dot("ParseJSON").Call(Lit(fieldName), iteratorExpr, dstExpr)
}

// builtinToName returns the string name of the builtin.
//
// Each kind's name corresponds with the functions in etype.
// That is, if this function returns "Foo" it expects the etype package
// in the runtime to contain "MarshalFoo and UnmarshalFoo".
func builtinToName(kind schema.BuiltinKind) string {
	return kind.String()
}
