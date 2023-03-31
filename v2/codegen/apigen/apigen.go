package apigen

import (
	"golang.org/x/exp/maps"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/authhandlergen"
	"encr.dev/v2/codegen/apigen/endpointgen"
	"encr.dev/v2/codegen/apigen/maingen"
	"encr.dev/v2/codegen/apigen/middlewaregen"
	"encr.dev/v2/codegen/apigen/servicestructgen"
	"encr.dev/v2/codegen/apigen/userfacinggen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
)

type Params struct {
	Gen        *codegen.Generator
	Desc       *app.Desc
	MainModule *pkginfo.Module

	CompilerVersion string
	AppRevision     string
	AppUncommitted  bool

	Test option.Option[codegen.TestConfig]

	ExecScriptMainPkg option.Option[paths.Pkg]
}

func Process(p Params) {
	gp := maingen.GenParams{
		Gen:               p.Gen,
		Desc:              p.Desc,
		MainModule:        p.MainModule,
		Test:              p.Test,
		ExecScriptMainPkg: p.ExecScriptMainPkg,

		CompilerVersion: p.CompilerVersion,
		AppRevision:     p.AppRevision,
		AppUncommitted:  p.AppUncommitted,

		APIHandlers:    make(map[*api.Endpoint]*codegen.VarDecl),
		Middleware:     make(map[*middleware.Middleware]*codegen.VarDecl),
		ServiceStructs: make(map[*app.Service]*codegen.VarDecl),

		// Set below
		AuthHandler: option.None[*codegen.VarDecl](),
	}

	if fw, ok := p.Desc.Framework.Get(); ok {

		svcStructBySvc := make(map[string]*codegen.VarDecl)

		for _, svc := range p.Desc.Services {
			var svcStruct option.Option[*codegen.VarDecl]
			if svcDesc, ok := svc.Framework.Get(); ok {
				if ss, ok := svcDesc.ServiceStruct.Get(); ok {
					decl := servicestructgen.Gen(p.Gen, svc, ss)
					gp.ServiceStructs[svc] = decl
					svcStruct = option.Some(decl)
					svcStructBySvc[svc.Name] = decl
				}

				mws := middlewaregen.Gen(p.Gen, svcDesc.Middleware, svcStruct)
				maps.Copy(gp.Middleware, mws)
			}

			eps := endpointgen.Gen(p.Gen, p.Desc.Parse, svc, svcStruct)
			maps.Copy(gp.APIHandlers, eps)

			// Generate user-facing code with the implementation in place.
			userfacinggen.Gen(p.Gen, svc, svcStruct)
		}

		gp.AuthHandler = option.Map(fw.AuthHandler, func(ah *authhandler.AuthHandler) *codegen.VarDecl {
			var svcStruct option.Option[*codegen.VarDecl]
			if svc, ok := p.Desc.ServiceForPath(ah.Decl.File.FSPath); ok {
				svcStruct = option.AsOptional(svcStructBySvc[svc.Name])
			}
			return authhandlergen.Gen(p.Gen, p.Desc, ah, svcStruct)
		})

		mws := middlewaregen.Gen(p.Gen, fw.GlobalMiddleware, option.None[*codegen.VarDecl]())
		maps.Copy(gp.Middleware, mws)
	}

	maingen.Gen(gp)
}
