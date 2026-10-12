package codegen_test

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestInsertTestSupport(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		for _, pkg := range desc.Parse.AppPackages() {
			gen.InsertTestSupport(pkg)
		}
	}
	codegentest.Run(t, fn)
}
