package authhandlergen

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		ah := desc.Framework.MustGet().AuthHandler.MustGet()
		Gen(gen, desc, ah)
	}

	codegentest.Run(t, fn)
}
