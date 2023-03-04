package endpointgen

import (
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/internal/gen"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apipaths"
)

func Gen(gen *gen.Generator, pkg *pkginfo.Package, endpoints []*api.Endpoint) {
	f := gen.File(pkg, "api")
	for _, ep := range endpoints {
		genAPIDesc(gen, f, ep)
	}
}

func genAPIDesc(gen *gen.Generator, f *gen.File, ep *api.Endpoint) {
	gu := gen.Util
	desc := f.VarDecl("APIDesc", ep.Name)
	reqDesc := &requestDesc{gu: gen.Util, ep: ep}
	respDesc := &responseDesc{gu: gen.Util, ep: ep}
	handler := &handlerDesc{gu: gen.Util, ep: ep, req: reqDesc, resp: respDesc}

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
	desc.Value(Op("&").Add(apiQ("Desc")).Types(
		reqDesc.Type(),
		apiQ("Void"), // TODO(andre) fix
	).Values(Dict{
		Id("Service"):        Lit("SERVICE"), // TODO
		Id("ServiceNum"):     Lit(0),         // TODO
		Id("Endpoint"):       Lit(ep.Name),
		Id("Methods"):        gu.GoToJen(pos, methods),
		Id("Raw"):            Lit(ep.Raw),
		Id("Path"):           Lit(ep.Path.String()),
		Id("RawPath"):        Lit(rawPath(ep.Path)),
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
}

func apiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/api", name)
}

// rawPath creates a raw path representation, replacing path parameters
// with their indices to ensure all httprouter paths use consistent path param names,
// since otherwise httprouter reports path conflicts.
func rawPath(path *apipaths.Path) string {
	var b strings.Builder
	nParam := 0
	for _, s := range path.Segments {
		b.WriteByte('/')

		switch s.Type {
		case apipaths.Literal:
			b.WriteString(s.Value)
			continue

		case apipaths.Param:
			b.WriteByte(':')
		case apipaths.Wildcard:
			b.WriteByte('*')
		}
		b.WriteString(strconv.Itoa(nParam))
		nParam++
	}
	return b.String()
}

// pathParamNames yields a []string literal containing the names
// of the path parameters, in order.
func pathParamNames(path *apipaths.Path) Code {
	n := 0
	expr := Index().String().ValuesFunc(func(g *Group) {
		for _, s := range path.Segments {
			if s.Type != apipaths.Literal {
				n++
				g.Lit(s.Value)
			}
		}
	})
	if n > 0 {
		return expr
	}
	return Nil()
}
