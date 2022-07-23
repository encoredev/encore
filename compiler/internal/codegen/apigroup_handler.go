package codegen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
)

func (b *Builder) buildAPIGroupHandler(f *File, group *est.APIGroup) {
	bb := &apiGroupHandlerBuilder{
		Builder: b,
		f:       f,
		svc:     group.Svc,
		group:   group,
	}
	bb.Write()
}

type apiGroupHandlerBuilder struct {
	*Builder
	f     *File
	svc   *est.Service
	group *est.APIGroup
}

func (b *apiGroupHandlerBuilder) Write() {
	initFuncName := Nil()
	if b.group.Init != nil {
		initFuncName = Id(b.group.Init.Name.Name)
	}

	handler := Var().Id(b.apiGroupHandlerName(b.group)).Op("=").Op("&").Qual("encore.dev/appruntime/api", "Group").Types(
		Id(b.group.Name),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Name").Op(":").Lit(b.group.Name),
		Id("Setup").Op(":").Add(initFuncName),
	)
	b.f.Add(handler)
}

func (b *Builder) apiGroupHandlerName(group *est.APIGroup) string {
	return fmt.Sprintf("EncoreInternal_%sAPIGroup", group.Name)
}
