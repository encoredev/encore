package apigen

import (
	"golang.org/x/exp/maps"

	"encr.dev/internal/paths"
	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/authhandlergen"
	"encr.dev/v2/codegen/apigen/endpointgen"
	"encr.dev/v2/codegen/apigen/maingen"
	"encr.dev/v2/codegen/apigen/middlewaregen"
	"encr.dev/v2/codegen/apigen/servicestructgen"
	"encr.dev/v2/codegen/apigen/userfacinggen"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
)

type Params struct {
	Gen        *codegen.Generator
	Desc       *app.Desc
	MainModule *pkginfo.Module

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

		APIHandlers:    make(map[*api.Endpoint]*codegen.VarDecl),
		Middleware:     make(map[*middleware.Middleware]*codegen.VarDecl),
		AuthHandler:    option.None[*codegen.VarDecl](),
		ServiceStructs: make(map[*app.Service]*codegen.VarDecl),
	}

	if fw, ok := p.Desc.Framework.Get(); ok {
		var svcStruct option.Option[*codegen.VarDecl]

		for _, svc := range p.Desc.Services {
			if svcDesc, ok := svc.Framework.Get(); ok {
				if ss, ok := svcDesc.ServiceStruct.Get(); ok {
					decl := servicestructgen.Gen(p.Gen, svc, ss)
					gp.ServiceStructs[svc] = decl
					svcStruct = option.Some(decl)
				}

				mws := middlewaregen.Gen(p.Gen, svcDesc.Middleware, svcStruct)
				maps.Copy(gp.Middleware, mws)
			}

			eps := endpointgen.Gen(p.Gen, p.Desc.Parse, svc, svcStruct)
			maps.Copy(gp.APIHandlers, eps)

			// Generate user-facing code with the implementation in place.
			userfacinggen.Gen(p.Gen, svc, svcStruct, true)
		}

		gp.AuthHandler = option.Map(fw.AuthHandler, func(ah *authhandler.AuthHandler) *codegen.VarDecl {
			return authhandlergen.Gen(p.Gen, p.Desc, ah, svcStruct)
		})

		mws := middlewaregen.Gen(p.Gen, fw.GlobalMiddleware, option.None[*codegen.VarDecl]())
		maps.Copy(gp.Middleware, mws)
	}

	maingen.Gen(gp)
}
