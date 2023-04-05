package maingen_test

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen"
	"encr.dev/v2/codegen/internal/codegentest"
	"encr.dev/v2/internals/pkginfo"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		loader := pkginfo.New(gen.Context)
		mainModule := loader.MainModule()
		params := apigen.Params{
			Gen:           gen,
			Desc:          desc,
			MainModule:    mainModule,
			RuntimeModule: loader.MainModule(),
		}
		apigen.Process(params)
	}

	codegentest.Run(t, fn)
}
