package servicestructgen

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		svc := desc.Services[0]
		Gen(gen, svc, svc.Framework.MustGet().ServiceStruct.MustGet())
	}

	codegentest.Run(t, fn)
}
