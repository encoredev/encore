package maingen_test

import (
	"encoding/json"
	"fmt"
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
		staticConfig := apigen.Process(params)

		// Create a synthetic file for golden tests to catch config changes.
		f := gen.InjectFile("synthetic/static_config", "synthetic", mainModule.RootDir.Join("synthetic"), "static_config.go", "static_config")
		configData, _ := json.MarshalIndent(staticConfig, "", "\t")
		f.Jen.Comment(fmt.Sprintf("\nThis is a synthetic file describing the generated static config:\n\n%s\n", configData))
	}

	codegentest.Run(t, fn)
}
