package endpointgen

import (
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
)

func Gen(gen *codegen.Generator, parse *parser.Result, svc *app.Service, svcStruct option.Option[*codegen.VarDecl]) map[*api.Endpoint]*codegen.VarDecl {
	epMap := make(map[*api.Endpoint]*codegen.VarDecl)
	if fw, ok := svc.Framework.Get(); ok {
		f := gen.File(fw.RootPkg, "api")
		for _, ep := range fw.Endpoints {
			handler := genAPIDesc(gen, f, svc, svcStruct, fw, ep)
			rewriteAPICalls(gen, parse, svc, ep, handler)

			epMap[ep] = handler.desc
		}

	}

	return epMap
}

func genAPIDesc(gen *codegen.Generator, f *codegen.File, svc *app.Service, svcStruct option.Option[*codegen.VarDecl], fw *apiframework.ServiceDesc, ep *api.Endpoint) *handlerDesc {
	gu := gen.Util
	reqDesc := &requestDesc{gu: gen.Util, ep: ep}
	respDesc := &responseDesc{gu: gen.Util, ep: ep}
	handler := &handlerDesc{
		gu:        gen.Util,
		ep:        ep,
		svcStruct: svcStruct,
		req:       reqDesc,
		resp:      respDesc,
	}

	f.Add(reqDesc.TypeDecl())
	f.Add(respDesc.TypeDecl())

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
	desc.Value(Op("&").Add(apiQ("Desc")).Types(
		reqDesc.Type(),
		respDesc.Type(),
	).Values(Dict{
		Id("Service"):        Lit(svc.Name),
		Id("SvcNum"):         Lit(fw.Num),
		Id("Endpoint"):       Lit(ep.Name),
		Id("Methods"):        gu.GoToJen(pos, methods),
		Id("Raw"):            Lit(ep.Raw),
		Id("Path"):           Lit(ep.Path.String()),
		Id("RawPath"):        Lit(rawPath(ep.Path)),
		Id("DefLoc"):         Lit(gen.TraceNodes.Endpoint(ep)),
		Id("PathParamNames"): pathParamNames(ep.Path),
		Id("Access"):         access,

		Id("DecodeReq"):      reqDesc.DecodeRequest(),
		Id("CloneReq"):       reqDesc.Clone(),
		Id("ReqPath"):        reqDesc.ReqPath(),
		Id("ReqUserPayload"): reqDesc.UserPayload(),

		Id("AppHandler"): handler.Typed(),
		Id("RawHandler"): handler.Raw(),
		Id("EncodeResp"): respDesc.EncodeResponse(),
		Id("CloneResp"):  respDesc.Clone(),
	}))

	handler.desc = desc
	return handler
}

func apiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/api", name)
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
