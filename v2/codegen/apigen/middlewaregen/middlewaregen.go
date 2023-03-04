package middlewaregen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis/middleware"
)

func Gen(gen *codegen.Generator, pkg *pkginfo.Package, mws []*middleware.Middleware) {
	f := gen.File(pkg, "middleware")
	for _, mw := range mws {
		genMiddleware(gen, f, mw)
	}
}

func genMiddleware(gen *codegen.Generator, f *codegen.File, mw *middleware.Middleware) {
	f.VarDecl("middleware", mw.Decl.Name).Value(Op("&").Qual("encore.dev/appruntime/api", "Middleware").Values(Dict{
		Id("PkgName"): Lit(mw.File.Pkg.Name),
		Id("Name"):    Lit(mw.Decl.Name),
		Id("Global"):  Lit(mw.Global),
		Id("DefLoc"):  Lit(0), // TODO
		// TODO(andre) Support service struct here
		Id("Invoke"): Qual(mw.File.Pkg.ImportPath.String(), mw.Decl.Name),
	}))
}
