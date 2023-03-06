package apigen

import (
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/authhandlergen"
	"encr.dev/v2/codegen/apigen/endpointgen"
	"encr.dev/v2/codegen/apigen/middlewaregen"
)

func Process(gg *codegen.Generator, desc *app.Desc) {
	fw := desc.Framework.MustGet()
	for _, svc := range desc.Services {
		endpointgen.Gen(gg, svc)
		if svc.Framework.IsPresent() {
			middlewaregen.Gen(gg, svc.Framework.MustGet().Middleware)
		}
	}

	middlewaregen.Gen(gg, fw.GlobalMiddleware)
	if fw.AuthHandler.IsPresent() {
		authhandlergen.Gen(gg, fw.AuthHandler.MustGet())
	}
}
