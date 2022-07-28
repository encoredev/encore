package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
)

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
	)
	b.f.Add(handler)
}

func (b *Builder) serviceStructName(ss *est.ServiceStruct) string {
	return fmt.Sprintf("EncoreInternal_%sService", ss.Name)
}
