package servicestructgen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/parser/apis/servicestruct"
)

func Gen(gen *codegen.Generator, svc *app.Service, s *servicestruct.ServiceStruct) {
	initFuncName := Nil()
	if s.Init.IsPresent() {
		initFuncName = Id(s.Init.MustGet().Name)
	}

	f := gen.File(s.Decl.File.Pkg, "svcstruct")
	f.VarDecl("svcstruct", s.Decl.Name).Value(Op("&").Qual("encore.dev/appruntime/service", "Decl").Types(
		Id(s.Decl.Name),
	).Values(Dict{
		Id("Service"):     Lit(svc.Name),
		Id("Name"):        Lit(s.Decl.Name),
		Id("Setup"):       initFuncName,
		Id("SetupDefLoc"): Lit(0), // TODO
	}))
}
