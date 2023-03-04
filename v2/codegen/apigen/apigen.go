package apigen

import (
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/authhandlergen"
	"encr.dev/v2/codegen/apigen/endpointgen"
	"encr.dev/v2/codegen/apigen/middlewaregen"
	"encr.dev/v2/parser/apis/apiframework"
)

func Process(gg *codegen.Generator, desc *apiframework.AppDesc) {
	for _, svc := range desc.Services {
		endpointgen.Gen(gg, svc)
	}
	if desc.AuthHandler.IsPresent() {
		authhandlergen.Gen(gg, desc.AuthHandler.MustGet())
	}
	middlewaregen.Gen(gg, desc.Middleware)
}
