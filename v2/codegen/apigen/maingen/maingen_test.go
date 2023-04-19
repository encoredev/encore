package maingen_test

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/apigen/maingen"
	"encr.dev/v2/codegen/internal/codegentest"
	"encr.dev/v2/internals/pkginfo"
)

func TestCodegen(t *testing.T) {
	maingen.GenerateForInternalPackageTests = true
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		loader := pkginfo.New(gen.Context)
		mainModule := loader.MainModule()
		params := apigen.Params{
			Gen:           gen,
			Desc:          desc,
			MainModule:    mainModule,
			RuntimeModule: loader.RuntimeModule(),
		}
		apigen.Process(params)
	}

	codegentest.Run(t, fn)
}
