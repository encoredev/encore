package apis

import (
	"go/ast"

	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
)

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
