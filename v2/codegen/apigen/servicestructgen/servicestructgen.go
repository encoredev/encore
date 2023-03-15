package servicestructgen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis/servicestruct"
)

func Gen(gen *codegen.Generator, svc *app.Service, s *servicestruct.ServiceStruct) *codegen.VarDecl {
	initFuncName := option.Map(s.Init, func(init *schema.FuncDecl) *Statement {
		return Id(init.Name)
	}).GetOrElse(Nil())

	f := gen.File(s.Decl.File.Pkg, "svcstruct")
	decl := f.VarDecl(s.Decl.Name).Value(Op("&").Qual("encore.dev/appruntime/service", "Decl").Types(
		Id(s.Decl.Name),
	).Values(Dict{
		Id("Service"):     Lit(svc.Name),
		Id("Name"):        Lit(s.Decl.Name),
		Id("Setup"):       initFuncName,
		Id("SetupDefLoc"): Lit(0), // TODO
	}))
	return decl
}
