package endpointgen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/parser/apis/api"
)

type handlerDesc struct {
	gu *genutil.Generator
	ep *api.Endpoint

	req  *requestDesc
	resp *responseDesc
}

func (h *handlerDesc) Typed() *Statement {
	ep := h.ep
	if ep.Raw {
		return Nil()
	}

	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("req").Add(h.req.Type()),
	).Params(h.resp.Type(), Error()).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// TODO(andre) support service structs
		fnExpr = Id(ep.Name)

		//// If we have a service struct, initialize it first.
		//group := ep.SvcStruct
		//if group != nil {
		//	ss := ep.Svc.Struct
		//	g.List(Id("svc"), Id("initErr")).Op(":=").Id(h.serviceStructName(ss)).Dot("Get").Call()
		//	g.If(Id("initErr").Op("!=").Nil()).Block(
		//		Return(h.RespZeroValue(), Id("initErr")),
		//	)
		//	fnExpr = Id("svc").Dot(h.ep.Name)
		//} else {
		//	fnExpr = Id(h.ep.Name)
		//}

		g.Do(func(s *Statement) {
			if ep.Response != nil {
				s.List(Id("resp"), Err())
			} else {
				s.Err()
			}
		}).Op(":=").Add(fnExpr).CallFunc(func(g *Group) {
			g.Id("ctx")
			g.Add(h.req.HandlerArgs()...)
		})
		g.If(Err().Op("!=").Nil()).Block(Return(h.resp.zero(), Err()))

		if ep.Response != nil {
			g.Return(Id("resp"), Nil())
		} else {
			g.Return(h.resp.zero(), Nil())
		}
	})
}

func (h *handlerDesc) Raw() *Statement {
	ep := h.ep
	if !ep.Raw {
		return Nil()
	}

	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("req").Op("*").Qual("net/http", "Request"),
	).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// TODO(andre) support service structs
		fnExpr = Id(ep.Name)

		//// If we have a service struct, initialize it first.
		//group := ep.SvcStruct
		//if group != nil {
		//	ss := ep.Svc.Struct
		//	g.List(Id("svc"), Id("initErr")).Op(":=").Id(h.serviceStructName(ss)).Dot("Get").Call()
		//	g.If(Id("initErr").Op("!=").Nil()).Block(
		//		Qual("encore.dev/beta/errs", "HTTPErrorWithCode").Call(Id("w"), Id("initErr"), Lit(0)),
		//		Return(),
		//	)
		//	fnExpr = Id("svc").Dot(h.rpc.Name)
		//} else {
		//	fnExpr = Id(h.rpc.Name)
		//}

		g.Add(fnExpr).Call(Id("w"), Id("req"))
	})
}
