package secretsgen

import (
	"testing"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/codegentest"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/secrets"
)

func TestCodegen(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		all := parser.Resources[*secrets.Secrets](desc.Parse)
		svc, _ := desc.ServiceForPath(all[0].File.Pkg.FSPath)
		Gen(gen, option.AsOptional(svc), all[0].File.Pkg, all)
	}
	codegentest.Run(t, fn)
}

// TestCodegenTestBuild covers a test build, where the test support import goes
// into the same file as the secrets import.
func TestCodegenTestBuild(t *testing.T) {
	fn := func(gen *codegen.Generator, desc *app.Desc) {
		all := parser.Resources[*secrets.Secrets](desc.Parse)
		svc, _ := desc.ServiceForPath(all[0].File.Pkg.FSPath)
		Gen(gen, option.AsOptional(svc), all[0].File.Pkg, all)
		gen.InsertTestSupport(all[0].File.Pkg)
	}
	codegentest.RunDir(t, "testdata/testbuild", fn)
}
