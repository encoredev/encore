package endpointgen

import (
	"testing"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/servicestructgen"
	"encr.dev/v2/codegen/apigen/userfacinggen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		svc := desc.Services[0]
		var svcStruct option.Option[*codegen.VarDecl]
		if fw, ok := svc.Framework.Get(); ok {
			if ss, ok := fw.ServiceStruct.Get(); ok {
				decl := servicestructgen.Gen(gen, svc, ss)
				svcStruct = option.Some(decl)

			}

			userfacinggen.Gen(gen, svc, svcStruct)
		}
		Gen(gen, desc.Parse, svc, svcStruct)
	}
	codegentest.Run(t, fn)
}
