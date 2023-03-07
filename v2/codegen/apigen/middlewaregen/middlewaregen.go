package middlewaregen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis/middleware"
)

func Gen(gen *codegen.Generator, mws []*middleware.Middleware) map[*middleware.Middleware]*codegen.VarDecl {
	mwMap := make(map[*middleware.Middleware]*codegen.VarDecl)

	pkgMap := make(map[*pkginfo.Package][]*middleware.Middleware)
	for _, mw := range mws {
		pkgMap[mw.File.Pkg] = append(pkgMap[mw.File.Pkg], mw)
	}

	for pkg, mws := range pkgMap {
		f := gen.File(pkg, "middleware")
		for _, mw := range mws {
			mwMap[mw] = genMiddleware(gen, f, mw)
		}
	}

	return mwMap
}

func genMiddleware(gen *codegen.Generator, f *codegen.File, mw *middleware.Middleware) *codegen.VarDecl {
	return f.VarDecl("middleware", mw.Decl.Name).Value(Op("&").Qual("encore.dev/appruntime/api", "Middleware").Values(Dict{
		Id("PkgName"): Lit(mw.File.Pkg.Name),
		Id("Name"):    Lit(mw.Decl.Name),
		Id("Global"):  Lit(mw.Global),
		Id("DefLoc"):  Lit(0), // TODO
		// TODO(andre) Support service struct here
		Id("Invoke"): Qual(mw.File.Pkg.ImportPath.String(), mw.Decl.Name),
	}))
}
