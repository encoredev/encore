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
	importPath := pkg.ImportPath + "!test"
	f := NewFilePathName(importPath, pkg.Name+"_test")
	b.registerImports(f)
	f.Anon("unsafe") // for go:linkname

	testSvc := ""
	if pkg.Service != nil {
		testSvc = pkg.Service.Name
	}

	mwNames, mwCode := b.RenderMiddlewares(importPath)

	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").Block(
		Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):     b.authDataType(),
			Id("Testing"):      True(),
			Id("TestService"):  Lit(testSvc),
			Id("PubsubTopics"): b.computeStaticPubsubConfig(),
		}),
		Id("handlers").Op(":=").Add(b.computeHandlerRegistrationConfig(mwNames)),
		Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
		})),
	)

	for _, c := range mwCode {
		f.Line()
		f.Add(c)
	}

	return f
}
