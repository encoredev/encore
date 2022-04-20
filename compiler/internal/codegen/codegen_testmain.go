package codegen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
)

func (b *Builder) TestMain(pkg *est.Package, svcs []*est.Service) *File {
	// Define this package as an external foo_test package so that we can
	// work around potential import cycles between the foo package and
	// imports of the auth data type (necessary for calling reflect.TypeOf).
	//
	// Use a synthetic (and invalid) import path of "!test" to tell jennifer to
	// add import statements for the non-"_test" package.
	f := NewFilePathName(pkg.ImportPath+"!test", pkg.Name+"_test")
	f.ImportNames(importNames)
	for _, p := range b.res.App.Packages {
		f.ImportName(p.ImportPath, p.Name)
	}

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it it will be imported under its proper name.
	f.Anon("encore.dev/runtime")

	getEnv := func(name string) Code {
		return Qual("os", "Getenv").Call(Lit(name))
	}

	f.Anon("unsafe") // for go:linkname
	testSvc := ""
	if pkg.Service != nil {
		testSvc = pkg.Service.Name
	}
	f.Comment("//go:linkname loadConfig encore.dev/runtime/config.loadConfig")
	f.Func().Id("loadConfig").Params().Params(Op("*").Qual("encore.dev/runtime/config", "Config"), Error()).Block(
		Id("services").Op(":=").Index().Op("*").Qual("encore.dev/runtime/config", "Service").ValuesFunc(func(g *Group) {
			for _, svc := range b.res.App.Services {
				g.Values(Dict{
					Id("Name"):      Lit(svc.Name),
					Id("RelPath"):   Lit(svc.Root.RelPath),
					Id("Endpoints"): Nil(),
				})
			}
		}),
		Id("static").Op(":=").Op("&").Qual("encore.dev/runtime/config", "Static").Values(Dict{
			Id("Services"):    Id("services"),
			Id("AuthData"):    b.authDataType(),
			Id("Testing"):     True(),
			Id("TestService"): Lit(testSvc),
		}),
		Return(Op("&").Qual("encore.dev/runtime/config", "Config").Values(Dict{
			Id("Static"):  Id("static"),
			Id("Runtime"): Qual("encore.dev/runtime/config", "ParseRuntime").Call(getEnv("ENCORE_RUNTIME_CONFIG")),
			Id("Secrets"): Qual("encore.dev/runtime/config", "ParseSecrets").Call(getEnv("ENCORE_APP_SECRETS")),
		}), Nil()),
	)

	return f
}
