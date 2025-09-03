package clientgen

import (
	"fmt"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/namealloc"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/parser/apis/api"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, svc *app.Service) {
	if fw, ok := svc.Framework.Get(); ok {
		clientPkgPath := paths.Pkg(appDesc.MainModule.Path).JoinSlash("clients", paths.RelSlash(svc.Name))
		clientPkgDir := appDesc.MainModule.RootDir.Join("clients", svc.Name)
		f := gen.InjectFile(
			clientPkgPath,
			svc.Name+"client",
			clientPkgDir,
			fmt.Sprintf("encore_internal__%sclient.go", svc.Name),
			"apidesc",
		)

		for _, ep := range fw.Endpoints {
			genEndpointFunction(gen, f, svc, ep)
		}
	}
}

func genEndpointFunction(gen *codegen.Generator, f *codegen.File, svc *app.Service, ep *api.Endpoint) {
	gu := gen.Util

	// Add doc comment if present
	if ep.Doc != "" {
		for _, line := range strings.Split(strings.TrimSpace(ep.Doc), "\n") {
			f.Jen.Comment(line)
		}
	}

	// Name allocator to avoid conflicts
	var names namealloc.Allocator
	alloc := func(input string) string {
		return names.Get(input)
	}

	ctxName := alloc("ctx")
	var pathParamNames []string
	reqParamName := ""

	// Start building the function
	f.Jen.Func().Id(ep.Name).ParamsFunc(func(g *Group) {
		// Context parameter
		g.Id(ctxName).Qual("context", "Context")

		// Path parameters
		for _, p := range ep.Path.Params() {
			paramName := alloc(p.Value)
			pathParamNames = append(pathParamNames, paramName)
			typ := gu.Builtin(p.Pos(), p.ValueType)
			g.Id(paramName).Add(typ)
		}

		// Request parameter
		if ep.Raw {
			reqName := alloc("req")
			g.Id(reqName).Op("*").Qual("net/http", "Request")
		} else if req := ep.Request; req != nil {
			reqParamName = alloc("req")
			g.Id(reqParamName).Add(gu.Type(req))
		}
	}).ParamsFunc(func(g *Group) {
		// Return types
		if ep.Raw {
			g.Op("*").Qual("net/http", "Response")
			g.Error()
		} else if resp := ep.Response; resp != nil {
			g.Add(gu.Type(resp))
			g.Error()
		} else {
			// Just error
			g.Error()
		}
	}).BlockFunc(func(g *Group) {
		// For raw endpoints, we can't make service-to-service calls
		if ep.Raw {
			g.Return(Nil(), Qual("errors", "New").Call(Lit("cannot call raw endpoints in service-to-service calls")))
			return
		}

		// Look up the handler
		handlerVar := alloc("handler")
		handlerOk := alloc("ok")
		g.List(Id(handlerVar), Id(handlerOk)).Op(":=").Qual("encore.dev/appruntime/apisdk/api", "LookupEndpoint").Call(
			Lit(svc.Name),
			Lit(ep.Name),
		)

		// Check if handler exists
		g.If(Op("!").Id(handlerOk)).BlockFunc(func(g *Group) {
			// Return error saying no endpoint registered
			errMsg := fmt.Sprintf("no endpoint registered for %s.%s", svc.Name, ep.Name)
			if ep.Response != nil {
				g.Return(gu.Zero(ep.Response), Qual("errors", "New").Call(Lit(errMsg)))
			} else {
				g.Return(Qual("errors", "New").Call(Lit(errMsg)))
			}
		})

		// Get the service package path
		fw, _ := svc.Framework.Get()
		svcPkgPath := fw.RootPkg.ImportPath

		// Cast handler to the appropriate Desc type
		descVar := alloc("desc")
		reqTypeName := fmt.Sprintf("EncoreInternal_%sReq", ep.Name)
		respTypeName := fmt.Sprintf("EncoreInternal_%sResp", ep.Name)

		// Type assert the handler
		g.List(Id(descVar), Id("ok")).Op(":=").Id(handlerVar).Assert(Op("*").Qual("encore.dev/appruntime/apisdk/api", "Desc").Types(
			Op("*").Qual(string(svcPkgPath), reqTypeName),
			Qual(string(svcPkgPath), respTypeName),
		))
		g.If(Op("!").Id("ok")).BlockFunc(func(g *Group) {
			errMsg := fmt.Sprintf("handler for %s.%s has unexpected type", svc.Name, ep.Name)
			if ep.Response != nil {
				g.Return(gu.Zero(ep.Response), Qual("errors", "New").Call(Lit(errMsg)))
			} else {
				g.Return(Qual("errors", "New").Call(Lit(errMsg)))
			}
		})

		// Create request struct
		reqVar := alloc("reqData")
		g.Id(reqVar).Op(":=").Op("&").Qual(string(svcPkgPath), reqTypeName).Values(DictFunc(func(d Dict) {
			// Add path parameters
			for i := range ep.Path.Params() {
				fieldName := fmt.Sprintf("P%d", i)
				d[Id(fieldName)] = Id(pathParamNames[i])
			}

			// Add request payload if present
			if req := ep.Request; req != nil && reqParamName != "" {
				d[Id("Payload")] = Id(reqParamName)
			}
		}))

		// Make the call
		callCtx := alloc("callCtx")
		g.Id(callCtx).Op(":=").Qual("encore.dev/appruntime/apisdk/api", "NewCallContext").Call(Id(ctxName))

		if resp := ep.Response; resp != nil {
			respVar := alloc("resp")
			g.List(Id(respVar), Err()).Op(":=").Id(descVar).Dot("Call").Call(Id(callCtx), Id(reqVar))
			g.Return(Id(respVar), Err())
		} else {
			g.List(Id("_"), Err()).Op(":=").Id(descVar).Dot("Call").Call(Id(callCtx), Id(reqVar))
			g.Return(Err())
		}
	})

	f.Jen.Line()
}
