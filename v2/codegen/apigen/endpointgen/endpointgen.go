package endpointgen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
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
	reqDesc := apigenutil.NewRequestDesc(gen.Util, ep)
	respDesc := apigenutil.NewResponseDesc(gen.Util, ep)
	handler := &handlerDesc{
		gu:        gen.Util,
		ep:        ep,
		svcStruct: svcStruct,
		req:       reqDesc,
		resp:      respDesc,
	}

	f.Add(reqDesc.TypeDecl())
	f.Add(respDesc.TypeDecl())

	descriptor := &apigenutil.APIDescriptor{
		ReqType:               reqDesc.Type(),
		RespType:              respDesc.Type(),
		DecodeReq:             reqDesc.DecodeRequest(),
		CloneReq:              reqDesc.Clone(),
		ReqPath:               reqDesc.ReqPath(),
		ReqUserPayload:        reqDesc.UserPayload(),
		AppHandler:            handler.Typed(),
		RawHandler:            handler.Raw(),
		EncodeResp:            respDesc.EncodeResponse(),
		CloneResp:             respDesc.Clone(),
		EncodeExternalReq:     reqDesc.EncodeExternalReq(),
		DecodeExternalResp:    respDesc.DecodeExternalResp(),
		ServiceMiddleware:     apigenutil.GenServiceMiddleware(ep, fw, svcMiddleware),
		GlobalMiddlewareIDs:   apigenutil.GenGlobalMiddleware(appDesc, ep),
	}

	desc := apigenutil.GenAPIDesc(gen, f, appDesc, svc, ep, descriptor)
	handler.desc = desc
	return handler
}


func registerHandlers(appDesc *app.Desc, file *codegen.File, handlers []*handlerDesc) {
	f := file.Jen
	f.Func().Id("init").Params().BlockFunc(func(g *Group) {
		for _, h := range handlers {
			g.Qual("encore.dev/appruntime/apisdk/api", "RegisterEndpoint").CallFunc(func(g *Group) {
				g.Add(h.desc.Qual())

				g.Add(Id(h.ep.Name))
			})
		}
	})
}

