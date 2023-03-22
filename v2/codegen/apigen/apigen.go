package apigen

import (
	"golang.org/x/exp/maps"

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

func Process(gg *codegen.Generator, desc *app.Desc, mainModule *pkginfo.Module, testConfig option.Option[codegen.TestConfig]) {
	gp := maingen.GenParams{
		Gen:            gg,
		Desc:           desc,
		MainModule:     mainModule,
		APIHandlers:    make(map[*api.Endpoint]*codegen.VarDecl),
		Middleware:     make(map[*middleware.Middleware]*codegen.VarDecl),
		AuthHandler:    option.None[*codegen.VarDecl](),
		ServiceStructs: make(map[*app.Service]*codegen.VarDecl),
		Test:           testConfig,
	}

	if fw, ok := desc.Framework.Get(); ok {
		var svcStruct option.Option[*codegen.VarDecl]

		for _, svc := range desc.Services {
			if svcDesc, ok := svc.Framework.Get(); ok {
				if ss, ok := svcDesc.ServiceStruct.Get(); ok {
					decl := servicestructgen.Gen(gg, svc, ss)
					gp.ServiceStructs[svc] = decl
					svcStruct = option.Some(decl)
				}

				mws := middlewaregen.Gen(gg, svcDesc.Middleware, svcStruct)
				maps.Copy(gp.Middleware, mws)
			}

			eps := endpointgen.Gen(gg, desc.Parse, svc, svcStruct)
			maps.Copy(gp.APIHandlers, eps)

			// Generate user-facing code with the implementation in place.
			userfacinggen.Gen(gg, svc, svcStruct, true)
		}

		gp.AuthHandler = option.Map(fw.AuthHandler, func(ah *authhandler.AuthHandler) *codegen.VarDecl {
			return authhandlergen.Gen(gg, desc, ah, svcStruct)
		})

		mws := middlewaregen.Gen(gg, fw.GlobalMiddleware, option.None[*codegen.VarDecl]())
		maps.Copy(gp.Middleware, mws)
	}

	maingen.Gen(gp)
}
