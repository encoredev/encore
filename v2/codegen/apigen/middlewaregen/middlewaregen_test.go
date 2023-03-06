package middlewaregen

import (
	"testing"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		allMiddleware := desc.Framework.MustGet().GlobalMiddleware
		for _, svc := range desc.Services {
			allMiddleware = append(allMiddleware, svc.Framework.MustGet().Middleware...)
		}
		Gen(gen, allMiddleware)
	}

	codegentest.Run(t, fn)
}
