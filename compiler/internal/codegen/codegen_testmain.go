package codegen

import (
	"encr.dev/parser/est"
	. "github.com/dave/jennifer/jen"
)

func (b *Builder) TestMain(pkg *est.Package, svcs []*est.Service) *File {
	f := NewFilePathName(pkg.ImportPath, pkg.Name)
	f.ImportNames(importNames)
	for _, p := range b.res.App.Packages {
		f.ImportName(p.ImportPath, p.Name)
	}

	f.Func().Id("TestMain").Params(Id("m").Op("*").Qual("testing", "M")).BlockFunc(func(g *Group) {
		g.Comment("Register the Encore services")
		g.Id("services").Op(":=").Index().Op("*").Qual("encore.dev/runtime/config", "Service").ValuesFunc(func(g *Group) {
			for _, svc := range svcs {
				usesSQLDB := false
			RefLoop:
				for _, pkg := range svc.Pkgs {
					for _, f := range pkg.Files {
						for _, ref := range f.References {
							if ref.Type == est.SQLDBNode {
								usesSQLDB = true
								break RefLoop
							}
						}
					}
				}

				g.Values(Dict{
					Id("Name"):      Lit(svc.Name),
					Id("RelPath"):   Lit(svc.Root.RelPath),
					Id("SQLDB"):     Lit(usesSQLDB),
					Id("Endpoints"): Nil(),
				})
			}
		})

		g.Line()

		g.Comment("Set up the Encore runtime")
		testSvc := ""
		if pkg.Service != nil {
			testSvc = pkg.Service.Name
		}
		g.Id("cfg").Op(":=").Op("&").Qual("encore.dev/runtime/config", "ServerConfig").Values(Dict{
			Id("Services"):    Id("services"),
			Id("Testing"):     True(),
			Id("TestService"): Lit(testSvc),
			Id("AuthData"):    b.authDataType(),
		})
		g.Qual("encore.dev/runtime", "Setup").Call(Id("cfg"))
		g.Qual("encore.dev/storage/sqldb", "Setup").Call(Id("cfg"))
		g.Qual("os", "Exit").Call(Id("m").Dot("Run").Call())
	})

	return f
}
