package maingen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, mainModule *pkginfo.Module) {
	mainPkgDir := mainModule.RootDir.Join("__encore", "main")
	mainPkgPath := paths.Pkg(mainModule.Path).JoinSlash("__encore", "main")

	file := gen.InjectFile(mainPkgPath, "main", mainPkgDir, "main")

	f := file.Jen

	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialised.
	for _, svc := range appDesc.Services {
		if svc.Framework.IsPresent() {
			rootPkg := svc.Framework.MustGet().RootPkg
			if rootPkg.ImportPath != mainPkgPath {
				f.Anon(rootPkg.ImportPath.String())
			}
		}
	}

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       Nil(),   // TODO
			Id("EncoreCompiler"): Lit(""), // TODO
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(""),    // TODO
				Id("Uncommitted"): Lit(false), // TODO
			}),
			Id("CORSAllowHeaders"):  Nil(), // TODO
			Id("CORSExposeHeaders"): Nil(), // TODO
			Id("PubsubTopics"):      Nil(), // TODO
			Id("Testing"):           False(),
			Id("TestService"):       Lit(""),
			Id("BundledServices"):   Nil(), // TODO
		})

		g.Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Nil(), // TODO
			Id("ServiceInit"): Nil(), // TODO
			Id("AuthHandler"): Nil(), // TODO
		}))
	})

	f.Func().Id("main").Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)
}
