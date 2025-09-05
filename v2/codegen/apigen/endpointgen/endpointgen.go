package endpointgen

import (
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/selector"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc, svc *app.Service, svcStruct option.Option[*codegen.VarDecl], svcMiddleware map[*middleware.Middleware]*codegen.VarDecl) map[*api.Endpoint]*codegen.VarDecl {
	epMap := make(map[*api.Endpoint]*codegen.VarDecl)

	if fw, ok := svc.Framework.Get(); ok {
		f := gen.File(fw.RootPkg, "api")

		// Use a separate pkg for the types wrapping the request and response
		// unless they are defined within the service.
		// This so that one can import the client without importing the service.
		useWrappersPkg := !apigenutil.HasInternalTypes(fw)

		var handlers []*handlerDesc
		for _, ep := range fw.Endpoints {
			var wrappersFile *codegen.File
			var wrappersPkg paths.Pkg

			if useWrappersPkg {
				// Create the wrappers package
				wrappersPkg = paths.Pkg(fw.RootPkg.ImportPath).JoinSlash(paths.RelSlash(svc.Name + "wrappers"))
				wrappersPkgDir := fw.RootPkg.FSPath.Join(svc.Name + "wrappers")
				wrappersFile = gen.InjectFile(wrappersPkg, svc.Name+"wrappers", wrappersPkgDir, "wrappers.go", "wrappers")
			} else {
				// Use the same file as the API
				wrappersFile = f
				wrappersPkg = fw.RootPkg.ImportPath
			}

			handler := genAPIDesc(gen, f, wrappersFile, appDesc, svc, svcStruct, fw, ep, svcMiddleware, wrappersPkg)
			rewriteAPICalls(gen, appDesc.Parse, svc, ep, handler)
			epMap[ep] = handler.desc
			handlers = append(handlers, handler)
		}

		registerHandlers(appDesc, f, handlers)
	}

	return epMap
}

func genAPIDesc(
	gen *codegen.Generator, f *codegen.File, wrappersFile *codegen.File, appDesc *app.Desc, svc *app.Service, svcStruct option.Option[*codegen.VarDecl],
	fw *apiframework.ServiceDesc, ep *api.Endpoint, svcMiddleware map[*middleware.Middleware]*codegen.VarDecl, wrappersPkg paths.Pkg,
) *handlerDesc {
	gu := gen.Util
	reqDesc := &requestDesc{gu: gen.Util, ep: ep, wrappersPkg: wrappersPkg, fw: fw}
	respDesc := &responseDesc{gu: gen.Util, ep: ep, wrappersPkg: wrappersPkg, fw: fw}
	handler := &handlerDesc{
		gu:          gen.Util,
		ep:          ep,
		svcStruct:   svcStruct,
		req:         reqDesc,
		resp:        respDesc,
		wrappersPkg: wrappersPkg,
	}

	wrappersFile.Add(reqDesc.TypeDecl())
	wrappersFile.Add(respDesc.TypeDecl())

	methods := ep.HTTPMethods
	if len(methods) == 1 && methods[0] == "*" {
		// All methods, from https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods
		methods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
	}

	var access *Statement
	switch ep.Access {
	case api.Public:
		access = apiQ("Public")
	case api.Auth:
		access = apiQ("RequiresAuth")
	case api.Private:
		access = apiQ("Private")
	default:
		gen.Errs.Addf(ep.Decl.AST.Pos(), "unhandled access type %v", ep.Access)
	}

	pos := ep.Decl.AST.Pos()
	desc := f.VarDecl("APIDesc", ep.Name)
	// If we're using the same package, don't qualify the types
	var reqType, respType *Statement
	if wrappersPkg == fw.RootPkg.ImportPath {
		reqType = Op("*").Id(reqDesc.TypeName())
		respType = Id(respDesc.TypeName())
	} else {
		reqType = Op("*").Qual(wrappersPkg.String(), reqDesc.TypeName())
		respType = Qual(wrappersPkg.String(), respDesc.TypeName())
	}

	desc.Value(Op("&").Add(apiQ("Desc")).Types(
		reqType,
		respType,
	).Values(Dict{
		Id("Service"):        Lit(svc.Name),
		Id("SvcNum"):         Lit(svc.Num),
		Id("Endpoint"):       Lit(ep.Name),
		Id("Methods"):        gu.GoToJen(pos, methods),
		Id("Raw"):            Lit(ep.Raw),
		Id("Fallback"):       Lit(ep.Path.HasFallback()),
		Id("Path"):           Lit(ep.Path.String()),
		Id("RawPath"):        Lit(rawPath(ep.Path)),
		Id("DefLoc"):         Lit(gen.TraceNodes.Endpoint(ep)),
		Id("PathParamNames"): pathParamNames(ep.Path),
		Id("Tags"):           tagNames(ep.Tags),
		Id("Access"):         access,

		Id("DecodeReq"):      reqDesc.DecodeRequest(),
		Id("CloneReq"):       reqDesc.Clone(),
		Id("ReqPath"):        reqDesc.ReqPath(),
		Id("ReqUserPayload"): reqDesc.UserPayload(),

		Id("AppHandler"): handler.Typed(),
		Id("RawHandler"): handler.Raw(),
		Id("EncodeResp"): respDesc.EncodeResponse(),
		Id("CloneResp"):  respDesc.Clone(),

		Id("EncodeExternalReq"):  reqDesc.EncodeExternalReq(),
		Id("DecodeExternalResp"): respDesc.DecodeExternalResp(),

		Id("ServiceMiddleware"):   serviceMiddleware(ep, fw, svcMiddleware),
		Id("GlobalMiddlewareIDs"): globalMiddleware(appDesc, ep),
	}))

	handler.desc = desc

	// Add compile-time check that Desc implements Callable
	f.Add(Var().Id("_").Qual("encore.dev/appruntime/apisdk/api", "Callable").Types(reqType, respType).Op("=").Add(desc.Qual()))

	return handler
}

func serviceMiddleware(ep *api.Endpoint, fw *apiframework.ServiceDesc, svcMiddleware map[*middleware.Middleware]*codegen.VarDecl) *Statement {
	return Index().Op("*").Add(apiQ("Middleware")).ValuesFunc(func(g *Group) {
		for _, mw := range fw.Middleware {
			if mw.Target.ContainsAny(ep.Tags) {
				g.Add(svcMiddleware[mw].Qual())
			}
		}
	})
}

func globalMiddleware(appDesc *app.Desc, ep *api.Endpoint) *Statement {
	return Index().String().ValuesFunc(func(g *Group) {
		for _, mw := range appDesc.MatchingGlobalMiddleware(ep) {
			g.Add(Lit(mw.ID()))
		}
	})
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

func apiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/apisdk/api", name)
}

// rawPath creates a raw path representation, replacing path parameters
// with their indices to ensure all httprouter paths use consistent path param names,
// since otherwise httprouter reports path conflicts.
func rawPath(path *resourcepaths.Path) string {
	var b strings.Builder
	nParam := 0
	for _, s := range path.Segments {
		b.WriteByte('/')

		switch s.Type {
		case resourcepaths.Literal:
			b.WriteString(s.Value)
			continue

		case resourcepaths.Param:
			b.WriteByte(':')
		case resourcepaths.Wildcard:
			b.WriteByte('*')
		case resourcepaths.Fallback:
			// Fallback paths map to a wildcard route in httprouter terms.
			b.WriteByte('*')
		}
		b.WriteString(strconv.Itoa(nParam))
		nParam++
	}
	return b.String()
}

// pathParamNames yields a []string literal containing the names
// of the path parameters, in order.
func pathParamNames(path *resourcepaths.Path) Code {
	if path.NumParams() == 0 {
		return Nil()
	}
	return Index().String().ValuesFunc(func(g *Group) {
		for _, s := range path.Params() {
			g.Lit(s.Value)
		}
	})
}

// tagNames yields a []string literal containing the tag names.
func tagNames(tags selector.Set) Code {
	if tags.Len() == 0 {
		return Nil()
	}
	return Index().String().ValuesFunc(func(g *Group) {
		tags.ForEach(func(sel selector.Selector) {
			if sel.Type == selector.Tag {
				g.Lit(sel.Value)
			}
		})
	})
}
