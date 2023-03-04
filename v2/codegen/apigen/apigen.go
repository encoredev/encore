package apigen

import (
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/authhandlergen"
	"encr.dev/v2/codegen/apigen/endpointgen"
	"encr.dev/v2/codegen/apigen/middlewaregen"
	"encr.dev/v2/parser/apis"
)

func Process(gg *codegen.Generator, parseResults []*apis.ParseResult) {
	for _, res := range parseResults {
		endpointgen.Gen(gg, res.Pkg, res.Endpoints)
		authhandlergen.Gen(gg, res.Pkg, res.AuthHandlers)
		middlewaregen.Gen(gg, res.Pkg, res.Middleware)
	}
}
