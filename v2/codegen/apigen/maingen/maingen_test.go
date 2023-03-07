package maingen_test

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/internal/codegentest"
	"encr.dev/v2/internal/pkginfo"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		loader := pkginfo.New(gen.Context)
		mainModule := loader.MainModule()
		apigen.Process(gen, desc, mainModule)
	}

	codegentest.Run(t, fn)
}
