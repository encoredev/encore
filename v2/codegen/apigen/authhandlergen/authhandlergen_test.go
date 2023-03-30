package authhandlergen

import (
	"testing"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/servicestructgen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		ah := desc.Framework.MustGet().AuthHandler.MustGet()

		var svcStruct option.Option[*codegen.VarDecl]
		if len(desc.Services) > 0 {
			svc := desc.Services[0]
			if fw, ok := svc.Framework.Get(); ok {
				if ss, ok := fw.ServiceStruct.Get(); ok {
					decl := servicestructgen.Gen(gen, svc, ss)
					svcStruct = option.Some(decl)

				}
			}
		}
		Gen(gen, desc, ah, svcStruct)
	}

	codegentest.Run(t, fn)
}
