package maingen

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
	"encr.dev/v2/internal/pkginfo"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		loader := pkginfo.New(gen.Context)
		mainModule := loader.MainModule()
		Gen(gen, desc, mainModule)
	}

	codegentest.Run(t, fn)
}
