package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
)

func (b *Builder) ServiceHandlers(svc *est.Service) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFilePathName(svc.Root.ImportPath, svc.Name)
	b.registerImports(f, svc.Root.ImportPath)

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it will be imported under its proper name.
	f.Anon("encore.dev/appruntime/app/appinit")

	if svc.Struct != nil {
		f.Line()
		b.buildServiceStructHandler(f, svc.Struct)
	}

	for _, rpc := range svc.RPCs {
		f.Line()
		b.buildRPC(f, svc, rpc)
	}

	if ah := b.res.App.AuthHandler; ah != nil && ah.Svc == svc {
		f.Line()
		b.buildAuthHandler(f, ah)
	}

	return f, b.errors.Err()
}

func (b *Builder) buildServiceStructHandler(f *File, ss *est.ServiceStruct) {
	bb := &serviceStructHandlerBuilder{
		Builder: b,
		f:       f,
		svc:     ss.Svc,
		ss:      ss,
	}
	bb.Write()
}

type serviceStructHandlerBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	ss  *est.ServiceStruct
}

func (b *serviceStructHandlerBuilder) Write() {
	initFuncName := Nil()
	if b.ss.Init != nil {
		initFuncName = Id(b.ss.Init.Name.Name)
	}

	setupDefLoc := 0
	if b.ss.Init != nil {
		setupDefLoc = int(b.res.Nodes[b.svc.Root][b.ss.Init].Id)
	}

	handler := Var().Id(b.serviceStructName(b.ss)).Op("=").Op("&").Qual("encore.dev/appruntime/service", "Decl").Types(
		Id(b.ss.Name),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Name").Op(":").Lit(b.ss.Name),
		Id("Setup").Op(":").Add(initFuncName),
		Id("SetupDefLoc").Op(":").Lit(setupDefLoc),
	)
	b.f.Add(handler)
}

func (b *Builder) serviceStructName(ss *est.ServiceStruct) string {
	return fmt.Sprintf("EncoreInternal_%sService", ss.Name)
}
