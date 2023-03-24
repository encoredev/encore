package maingen

import (
	gotoken "go/token"

	. "github.com/dave/jennifer/jen"

	"encr.dev/internal/paths"
	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/middleware"
)

type GenParams struct {
	Gen        *codegen.Generator
	Desc       *app.Desc
	MainModule *pkginfo.Module

	// CompilerVersion is the version of the compiler to embed in the generated code.
	CompilerVersion string
	// AppRevision is the revision of the app to embed in the generated code.
	AppRevision string
	// AppUncommitted tracks whether there were uncommitted changes in the app
	// at the time of build.
	AppUncommitted bool

	APIHandlers    map[*api.Endpoint]*codegen.VarDecl
	AuthHandler    option.Option[*codegen.VarDecl]
	Middleware     map[*middleware.Middleware]*codegen.VarDecl
	ServiceStructs map[*app.Service]*codegen.VarDecl

	// Test contains configuration for generating test code.
	Test option.Option[codegen.TestConfig]

	// ExecScriptMainPkg is the main package to build for an ExecScript execution.
	ExecScriptMainPkg option.Option[paths.Pkg]
}

func Gen(p GenParams) {
	if test, ok := p.Test.Get(); ok {
		genTestConfigs(p, test)
	} else if execScript, ok := p.ExecScriptMainPkg.Get(); ok {
		genExecScriptMain(p, execScript)
	} else {
		genMain(p)
	}
}

func genMain(p GenParams) {
	mainPkgDir := p.MainModule.RootDir.Join("__encore", "main")
	mainPkgPath := paths.Pkg(p.MainModule.Path).JoinSlash("__encore", "main")

	file := p.Gen.InjectFile(mainPkgPath, "main", mainPkgDir, "main.go", "main")
	f := file.Jen

	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialized.
	for _, svc := range p.Desc.Services {
		svc.Framework.ForAll(func(svcDesc *apiframework.ServiceDesc) {
			rootPkg := svcDesc.RootPkg
			if rootPkg.ImportPath != mainPkgPath {
				f.Anon(rootPkg.ImportPath.String())
			}
		})
	}

	genLoadApp(p, f, option.None[testParams]())
	f.Func().Id("main").Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)
}

func genExecScriptMain(p GenParams, mainPkgPath paths.Pkg) {
	mainPkgDir, ok := p.MainModule.FSPathToPkg(mainPkgPath)
	if !ok {
		p.Desc.Errs.Addf(gotoken.NoPos, "cannot find package %s in module %s",
			mainPkgPath, p.MainModule.Path)
		return
	}

	file := p.Gen.InjectFile(mainPkgPath, "main", mainPkgDir, "encore_internal__execscript.go", "execscript")
	f := file.Jen

	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialized.
	for _, svc := range p.Desc.Services {
		svc.Framework.ForAll(func(svcDesc *apiframework.ServiceDesc) {
			rootPkg := svcDesc.RootPkg
			if rootPkg.ImportPath != mainPkgPath {
				f.Anon(rootPkg.ImportPath.String())
			}
		})
	}

	genLoadApp(p, f, option.None[testParams]())
}
