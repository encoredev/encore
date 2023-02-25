// Package schematest provides utilities for writing tests
// that make assertions about schema types and declarations.
package schematest

import (
	"go/token"

	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
)

func Ptr(elem schema.Type) schema.Type {
	return schema.PointerType{Elem: elem}
}

func Builtin(kind schema.BuiltinKind) schema.Type {
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

func String() schema.Type {
	return Builtin(schema.String)
}

func Bool() schema.Type {
	return Builtin(schema.Bool)
}

func Int() schema.Type {
	return Builtin(schema.Int)
}

func Error() schema.Type {
	return Builtin(schema.Error)
}
