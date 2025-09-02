package endpointgen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/middleware"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, svc *app.Service, svcStruct option.Option[*codegen.VarDecl], svcMiddleware map[*middleware.Middleware]*codegen.VarDecl) map[*api.Endpoint]*codegen.VarDecl {
	epMap := make(map[*api.Endpoint]*codegen.VarDecl)

	if fw, ok := svc.Framework.Get(); ok {
		f := gen.File(fw.RootPkg, "api")

		var handlers []*handlerDesc
		for _, ep := range fw.Endpoints {
			handler := genAPIDesc(gen, f, appDesc, svc, svcStruct, fw, ep, svcMiddleware)
			rewriteAPICalls(gen, appDesc.Parse, svc, ep, handler)
			epMap[ep] = handler.desc
			handlers = append(handlers, handler)
		}

		registerHandlers(appDesc, f, handlers)
	}

	return epMap
}

func genAPIDesc(
	gen *codegen.Generator, f *codegen.File, appDesc *app.Desc, svc *app.Service, svcStruct option.Option[*codegen.VarDecl],
	fw *apiframework.ServiceDesc, ep *api.Endpoint, svcMiddleware map[*middleware.Middleware]*codegen.VarDecl,
) *handlerDesc {
	// Use client package types for registration
	clientPkgPath := paths.Pkg(appDesc.MainModule.Path).JoinSlash("clients", paths.RelSlash(svc.Name))
	
	// Create RequestDesc and ResponseDesc for the handler
	reqDesc := apigenutil.NewRequestDesc(gen.Util, ep)
	respDesc := apigenutil.NewResponseDesc(gen.Util, ep)
	
	handler := &handlerDesc{
		gu:        gen.Util,
		ep:        ep,
		svcStruct: svcStruct,
		req:       reqDesc,
		resp:      respDesc,
	}

	// Don't generate our own APIDesc - the client package will handle this
	// Just register the endpoint to use the client's APIDesc
	// We need to reference the client's APIDesc instead of creating our own
	
	// Generate a reference to the client's APIDesc
	clientApiDescName := fmt.Sprintf("EncoreInternal_%sclient_api_APIDesc_%s", svc.Name, ep.Name)
	clientApiDescVar := Qual(clientPkgPath.String(), clientApiDescName)
	
	// Create a simple descriptor reference that points to the client's APIDesc
	desc := f.VarDecl("APIDesc", ep.Name).Value(clientApiDescVar)
	handler.desc = desc
	return handler
}


func registerHandlers(appDesc *app.Desc, file *codegen.File, handlers []*handlerDesc) {
	f := file.Jen
	f.Func().Id("init").Params().BlockFunc(func(g *Group) {
		for _, h := range handlers {
			g.Qual("encore.dev/appruntime/apisdk/api", "RegisterEndpoint").CallFunc(func(g *Group) {
				// The APIDesc is now generated in the client package
				g.Add(h.desc.Qual())
				g.Lit(h.ep.Name)
			})
		}
	})
}

