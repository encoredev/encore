// Package schematest provides utilities for writing tests
// that make assertions about schema types and declarations.
package schematest

import (
	"go/token"

	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
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

func Error() schema.Type {
	return Builtin(schema.Error)
}
