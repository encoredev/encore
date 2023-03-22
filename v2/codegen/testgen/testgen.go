package testgen

import (
	. "github.com/dave/jennifer/jen"
	"golang.org/x/exp/slices"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/pkginfo"
)

func Process(gg *codegen.Generator, desc *app.Desc, mainModule *pkginfo.Module) {
	for _, pkg := range desc.Parse.AppPackages() {
		// If there are no test files, skip the package.
		isTestFile := func(f *pkginfo.File) bool { return f.TestFile }
		if slices.IndexFunc(pkg.Files, isTestFile) == -1 {
			continue
		}

		genTestMain(gg, pkg)
	}
}

func genTestMain(gg *codegen.Generator, pkg *pkginfo.Package) {
	// Define this package as an external foo_test package so that we can
	// work around potential import cycles between the foo package and
	// imports of the auth data type (necessary for calling reflect.TypeOf).
	//
	// Use a synthetic (and invalid) import path of "!test" to tell jennifer to
	// add import statements for the non-"_test" package.
	file := gg.InjectFile(pkg.ImportPath+"!test", pkg.Name+"_test", pkg.FSPath, "encore_internal__testmain.go", "testmain")
	f := file.Jen

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       authDataType(gen.Util, appDesc),
			Id("EncoreCompiler"): Lit(p.CompilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(p.AppRevision),
				Id("Uncommitted"): Lit(p.AppUncommitted),
			}),
			Id("CORSAllowHeaders"):  allowHeaders,
			Id("CORSExposeHeaders"): exposeHeaders,
			Id("PubsubTopics"):      pubsubTopics(appDesc),
			Id("Testing"):           False(),
			Id("TestService"):       Lit(""),
			Id("BundledServices"):   bundledServices(appDesc),
		})

		g.Id("handlers").Op(":=").Add(computeHandlerRegistrationConfig(appDesc, p.APIHandlers, p.Middleware))

		g.Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
			Id("ServiceInit"): serviceInitConfig(p.ServiceStructs),
			Id("AuthHandler"): authHandler,
		}))
	})

	f.Func().Id("main").Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)
}
