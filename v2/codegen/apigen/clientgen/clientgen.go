package clientgen

import (
	"fmt"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/namealloc"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis/api"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, svc *app.Service, withImpl bool) option.Option[*codegen.File] {
	if fw, ok := svc.Framework.Get(); ok {
		clientPkgPath := paths.Pkg(appDesc.MainModule.Path).JoinSlash("clients", paths.RelSlash(svc.Name))
		clientPkgDir := appDesc.MainModule.RootDir.Join("clients", svc.Name)
		f := gen.InjectFile(
			clientPkgPath,
			svc.Name+"client",
			clientPkgDir,
			"encore.gen.go",
			svc.Name+"client",
		)

		// Interface and struct names
		interfaceName := strings.Title(svc.Name) + "Client"
		implName := svc.Name + "ClientImpl"

		// Generate the interface
		f.Jen.Comment(fmt.Sprintf("%s is the client interface for the %s service.", interfaceName, svc.Name))
		f.Jen.Type().Id(interfaceName).InterfaceFunc(func(g *Group) {
			for _, ep := range fw.Endpoints {
				genEndpointSignature(gen, g, svc, ep)
			}
		})

		f.Jen.Line()

		// Generate the implementation struct
		f.Jen.Comment(fmt.Sprintf("%s implements the %s interface.", implName, interfaceName))
		f.Jen.Type().Id(implName).Struct(
		// Empty for now, but could add fields later
		)

		f.Jen.Line()

		// Generate constructor
		constructorName := "New" + strings.Title(svc.Name) + "Client"
		f.Jen.Comment(fmt.Sprintf("%s creates a new client for the %s service.", constructorName, svc.Name))
		f.Jen.Func().Id(constructorName).Params().Id(interfaceName).Block(
			Return(Op("&").Id(implName).Values()),
		)

		f.Jen.Line()

		// Generate methods for each endpoint
		for _, ep := range fw.Endpoints {
			genEndpointMethod(gen, f, svc, ep, withImpl, implName)
		}

		return option.Some(f)
	}

	return option.None[*codegen.File]()
}

// genEndpointSignature generates the method signature for the interface
func genEndpointSignature(gen *codegen.Generator, g *Group, svc *app.Service, ep *api.Endpoint) {
	gu := gen.Util

	// Add doc comment if present
	if ep.Doc != "" {
		for _, line := range strings.Split(strings.TrimSpace(ep.Doc), "\n") {
			g.Comment(line)
		}
	}

	// Name allocator to avoid conflicts
	var names namealloc.Allocator
	alloc := func(input string) string {
		return names.Get(input)
	}

	ctxName := alloc("ctx")

	// Build the method signature
	g.Id(ep.Name).ParamsFunc(func(g *Group) {
		// Context parameter
		g.Id(ctxName).Qual("context", "Context")

		// Path parameters
		for _, p := range ep.Path.Params() {
			paramName := alloc(p.Value)
			typ := gu.Builtin(p.Pos(), p.ValueType)
			g.Id(paramName).Add(typ)
		}

		// Request parameter
		if ep.Raw {
			reqName := alloc("req")
			g.Id(reqName).Op("*").Qual("net/http", "Request")
		} else if req := ep.Request; req != nil {
			reqParamName := alloc("req")
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
	})
}

// genEndpointMethod generates the method implementation on the struct
func genEndpointMethod(gen *codegen.Generator, f *codegen.File, svc *app.Service, ep *api.Endpoint, withImpl bool, implName string) {
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

	// Start building the method as a receiver function
	f.Jen.Func().Parens(Id("c").Op("*").Id(implName)).Id(ep.Name).ParamsFunc(func(g *Group) {
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
		if !withImpl {
			// Implementation is elided
			g.Comment("The implementation is elided here, and generated at compile-time by Encore.")
			if ep.Raw {
				g.Return(Nil(), Nil())
			} else if ep.Response != nil {
				g.Return(gu.Zero(ep.Response), Nil())
			} else {
				// Just an error return
				g.Return(Nil())
			}
			return
		}

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

		// Cast handler to the appropriate Callable interface
		callableVar := alloc("callable")
		reqTypeName := fmt.Sprintf("EncoreInternal_%sReq", ep.Name)
		respTypeName := fmt.Sprintf("EncoreInternal_%sResp", ep.Name)

		// Check if types are defined in the same package as the service
		useWrappers := shouldUseWrappersPackageForClient(fw, ep)
		var reqTypeQual, respTypeQual *Statement
		var wrappersPkgPath string

		if useWrappers {
			wrappersPkgPath = paths.Pkg(svcPkgPath).JoinSlash(paths.RelSlash(svc.Name + "wrappers")).String()
			reqTypeQual = Op("*").Qual(wrappersPkgPath, reqTypeName)
			respTypeQual = Qual(wrappersPkgPath, respTypeName)
		} else {
			wrappersPkgPath = string(svcPkgPath)
			reqTypeQual = Op("*").Qual(string(svcPkgPath), reqTypeName)
			respTypeQual = Qual(string(svcPkgPath), respTypeName)
		}

		// Type assert the handler to Callable interface
		g.List(Id(callableVar), Id("ok")).Op(":=").Id(handlerVar).Assert(Qual("encore.dev/appruntime/apisdk/api", "Callable").Types(
			reqTypeQual,
			respTypeQual,
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
		g.Id(reqVar).Op(":=").Op("&").Qual(wrappersPkgPath, reqTypeName).Values(DictFunc(func(d Dict) {
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

		// Create CallContext from the regular context
		callCtxVar := alloc("callCtx")
		g.Id(callCtxVar).Op(":=").Qual("encore.dev/appruntime/apisdk/api", "Singleton").Dot("NewCallContext").Call(Id(ctxName))

		// Call the Call method on the callable
		if resp := ep.Response; resp != nil {
			respVar := alloc("resp")
			g.List(Id(respVar), Err()).Op(":=").Id(callableVar).Dot("Call").Call(Id(callCtxVar), Id(reqVar))
			g.Return(Id(respVar), Err())
		} else {
			g.List(Id("_"), Err()).Op(":=").Id(callableVar).Dot("Call").Call(Id(callCtxVar), Id(reqVar))
			g.Return(Err())
		}
	})

	f.Jen.Line()
}

// shouldUseWrappersPackageForClient determines if we should use wrapper types from a separate package.
// This logic should match the logic in endpointgen.
func shouldUseWrappersPackageForClient(fw *apiframework.ServiceDesc, ep *api.Endpoint) bool {
	hasExternalTypes := false

	// Check if request type is from the same package
	if ep.Request != nil {
		if named, ok := ep.Request.(schema.NamedType); ok {
			if named.DeclInfo != nil && named.DeclInfo.File.Pkg.ImportPath == fw.RootPkg.ImportPath {
				// Type is from same package, can't use wrappers
				return false
			}
			// Type is external
			hasExternalTypes = true
		}
	}

	// Check if response type is from the same package
	if ep.Response != nil {
		if named, ok := ep.Response.(schema.NamedType); ok {
			if named.DeclInfo != nil && named.DeclInfo.File.Pkg.ImportPath == fw.RootPkg.ImportPath {
				// Type is from same package, can't use wrappers
				return false
			}
			// Type is external
			hasExternalTypes = true
		}
	}

	// Only use wrappers if we have external types
	return hasExternalTypes
}
