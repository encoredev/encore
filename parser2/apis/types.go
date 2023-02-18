package apis

import (
	"go/ast"

	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
)

type AuthHandler struct {
	Name   string
	Doc    string
	Func   *ast.FuncDecl
	File   *pkginfo.File
	Params schema.Type   // builtin string or named type
	Recv   *ast.FuncDecl // nil if not defined on a service struct

	// AuthData is the custom auth data type the app may specify
	// as part of the returns from the auth handler.
	// It is nil if no such auth data type is specified.
	AuthData schema.Type
}

type Middleware struct {
	Name   string
	Doc    string
	Global bool
	Target selector.Set

	Func *ast.FuncDecl
	File *pkginfo.File

	Pkg  *pkginfo.Package // pkg this middleware is defined in
	Recv *ast.FuncDecl    // nil if not defined on a receiver
}

type Receiver struct {
	Func *schema.TypeDecl
}
