package middlewaregen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/middleware"
)

func Gen(gen *codegen.Generator, mws []*middleware.Middleware, svcStruct option.Option[*codegen.VarDecl]) map[*middleware.Middleware]*codegen.VarDecl {
	mwMap := make(map[*middleware.Middleware]*codegen.VarDecl)

	pkgMap := make(map[*pkginfo.Package][]*middleware.Middleware)
	for _, mw := range mws {
		pkgMap[mw.File.Pkg] = append(pkgMap[mw.File.Pkg], mw)
	}

	for pkg, mws := range pkgMap {
		f := gen.File(pkg, "middleware")
		for _, mw := range mws {
			mwMap[mw] = genMiddleware(gen, f, mw, svcStruct)
		}
	}

	return mwMap
}

func genMiddleware(gen *codegen.Generator, f *codegen.File, mw *middleware.Middleware, svcStruct option.Option[*codegen.VarDecl]) *codegen.VarDecl {
	invoke := Qual(mw.File.Pkg.ImportPath.String(), mw.Decl.Name)
	if !mw.Global && mw.Recv.Present() && svcStruct.Present() {
		invoke = Func().Params(
			Id("req").Qual("encore.dev/middleware", "Request"),
			Id("next").Qual("encore.dev/middleware", "Next"),
		).Params(Qual("encore.dev/middleware", "Response")).BlockFunc(func(g *Group) {
			g.List(Id("svc"), Err()).Op(":=").Add(svcStruct.MustGet().Qual()).Dot("Get").Call()
			g.If(Err().Op("!=").Nil()).Block(
				Return(Qual("encore.dev/middleware", "Response").Values(Dict{
					Id("Err"):        Err(),
					Id("HTTPStatus"): Qual("encore.dev/beta/errs", "HTTPStatus").Call(Err()),
				})),
			)
			g.Return(Id("svc").Dot(mw.Decl.Name).Call(Id("req"), Id("next")))
		})
	}

	decl := f.VarDecl("middleware", mw.Decl.Name).Value(Op("&").Qual("encore.dev/appruntime/apisdk/api", "Middleware").Values(Dict{
		Id("ID"):      Lit(mw.ID()),
		Id("PkgName"): Lit(mw.File.Pkg.Name),
		Id("Name"):    Lit(mw.Decl.Name),
		Id("Global"):  Lit(mw.Global),
		Id("DefLoc"):  Lit(gen.TraceNodes.Middleware(mw)),
		Id("Invoke"):  invoke,
	}))

	if mw.Global {
		f.Jen.Func().Id("init").Params().Block(
			Qual("encore.dev/appruntime/apisdk/api", "RegisterGlobalMiddleware").Call(decl.Qual()),
		)
	}

	return decl
}
