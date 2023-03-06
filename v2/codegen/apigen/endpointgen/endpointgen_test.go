package endpointgen

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	tests := []codegentest.Case{
		{
			Name:    "basic",
			Imports: []string{"context"},
			Code: `
//encore:api public
func Foo(ctx context.Context) error { return nil }
`,
		},
	}

	fn := func(gen *codegen.Generator, desc *app.Desc) {
		Gen(gen, desc.Services[0])
	}

	codegentest.Run(t, tests, fn)
}
