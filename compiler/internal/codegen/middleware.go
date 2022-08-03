package codegen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
	"encr.dev/pkg/namealloc"
)

func (b *Builder) RenderMiddlewares(pkgPath string) (map[*est.Middleware]*Statement, []Code) {
	nameMap := make(map[*est.Middleware]*Statement, len(b.res.App.Middleware))
	var names namealloc.Allocator
	var codes []Code
	for _, mw := range b.res.App.Middleware {
		name := names.Get("EncoreInternal_" + mw.Pkg.Name + mw.Name)
		nameMap[mw] = Qual(pkgPath, name)
		code := b.buildMiddleware(mw, name)
		codes = append(codes, code)
	}
	return nameMap, codes
}

func (b *Builder) buildMiddleware(mw *est.Middleware, name string) Code {
	bb := &middlewareBuilder{
		Builder: b,
		mw:      mw,
		name:    name,
	}
	return bb.Render()
}

type middlewareBuilder struct {
	*Builder
	mw   *est.Middleware
	name string
}

func (b *middlewareBuilder) Render() Code {
	invokeHandler := b.renderInvoke()
	defLoc := int(b.res.Nodes[b.mw.Pkg][b.mw.Func].Id)
	handler := Var().Id(b.name).Op("=").Op("&").Qual("encore.dev/appruntime/api", "Middleware").Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("PkgName").Op(":").Lit(b.mw.Pkg.Name),
		Id("Name").Op(":").Lit(b.mw.Name),
		Id("Global").Op(":").Lit(b.mw.Global),
		Id("DefLoc").Op(":").Lit(defLoc),
		Id("Invoke").Op(":").Add(invokeHandler),
	)
	return handler
}

func (b *middlewareBuilder) renderInvoke() *Statement {
	mw := b.mw
	if mw.SvcStruct == nil {
		return Qual(mw.Pkg.ImportPath, mw.Name)
	}

	ss := mw.SvcStruct
	return Func().Params(
		Id("req").Qual("encore.dev/middleware", "Request"),
		Id("next").Qual("encore.dev/middleware", "Next"),
	).Params(Qual("encore.dev/middleware", "Response")).BlockFunc(func(g *Group) {
		g.List(Id("svc"), Err()).Op(":=").Qual(ss.Svc.Root.ImportPath, b.serviceStructName(ss)).Dot("Get").Call()
		g.If(Err().Op("!=").Nil()).Block(
			Return(Qual("encore.dev/middleware", "Response").Values(Dict{
				Id("Err"): Err(),
			})),
		)
		g.Return(Id("svc").Dot(mw.Name).Call(Id("req"), Id("next")))
	})
}
