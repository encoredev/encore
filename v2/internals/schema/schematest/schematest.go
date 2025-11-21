// Package schematest provides utilities for writing tests
// that make assertions about schema types and declarations.
package schematest

import (
	"go/token"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
)

func Ptr(elem schema.Type) schema.Type {
	return schema.PointerType{Elem: elem}
}

func Option(elem schema.Type) schema.Type {
	return schema.OptionType{Value: elem}
}

func Builtin(kind schema.BuiltinKind) schema.BuiltinType {
	return schema.BuiltinType{Kind: kind}
}

func Named(info *pkginfo.PkgDeclInfo) schema.Type {
	return schema.NamedType{DeclInfo: info}
}

func Slice(elem schema.Type) schema.Type {
	return schema.ListType{Elem: elem}
}

func TypeInfo(name string) *pkginfo.PkgDeclInfo {
	return &pkginfo.PkgDeclInfo{
		Name: name,
		Type: token.TYPE,
	}
}

func Param(typ schema.Type) schema.Param {
	return schema.Param{Type: typ}
}

func String() schema.BuiltinType {
	return Builtin(schema.String)
}

func Bool() schema.BuiltinType {
	return Builtin(schema.Bool)
}

func Int() schema.BuiltinType {
	return Builtin(schema.Int)
}

func Error() schema.BuiltinType {
	return Builtin(schema.Error)
}
