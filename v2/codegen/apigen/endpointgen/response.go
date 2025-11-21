package endpointgen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
)

// responseDesc describes the generated response type.
type responseDesc struct {
	gu *genutil.Helper
	ep *api.Endpoint
}

func (d *responseDesc) TypeName() string {
	return "EncoreInternal_" + d.ep.Name + "Resp"
}

func (d *responseDesc) Type() *Statement {
	return Id(d.TypeName())
}

func (d *responseDesc) TypeDecl() *Statement {
	return Type().Id(d.TypeName()).Op("=").Do(func(s *Statement) {
		if d.ep.Response != nil {
			s.Add(d.gu.Type(d.ep.Response))
		} else {
			s.Add(apiQ("Void"))
		}
	})
}

func (d *responseDesc) ZeroType() *Statement {
	if d.ep.Response != nil {
		return d.gu.Zero(d.ep.Response)
	} else {
		return apiQ("Void").Values()
	}
}

func (d *responseDesc) EncodeResponse() *Statement {
	if d.ep.Raw {
		return Nil()
	}

	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
		Id("resp").Add(d.Type()),
		Id("status").Int(),
	).Params(Err().Error()).BlockFunc(func(g *Group) {
		if d.ep.Response == nil {
			g.Return(Nil())
			return
		}

		resp := apienc.DescribeResponse(d.gu.Errs, d.ep.Response)
		if len(resp.BodyParameters) > 0 {
			g.Id("respData").Op(":=").Index().Byte().Parens(Lit("null\n"))
		} else {
			g.Id("respData").Op(":=").Index().Byte().Values(LitRune('\n'))
		}
		if len(resp.HeaderParameters) > 0 {
			g.Var().Id("headers").Map(String()).Index().String()
		}

		responseEncoder := CustomFunc(Options{Separator: "\n"}, func(g *Group) {
			if len(resp.BodyParameters) > 0 {
				g.Comment("Encode JSON body")
				g.List(Id("respData"), Err()).Op("=").Qual("encore.dev/appruntime/shared/serde", "SerializeJSONFunc").Call(
					Id("json"),
					Func().Params(
						Id("ser").Op("*").Qual("encore.dev/appruntime/shared/serde", "JSONSerializer"),
					).BlockFunc(
						func(g *Group) {
							for _, f := range resp.BodyParameters {
								g.Add(Id("ser").Dot("WriteField").Call(Lit(f.WireName), Id("resp").Dot(f.SrcName), Lit(f.OmitEmpty)))
							}
						}))
				g.If(Err().Op("!=").Nil()).Block(
					Return(Err()),
				)
				g.Id("respData").Op("=").Append(Id("respData"), LitRune('\n'))
			}

			if len(resp.HeaderParameters) > 0 {
				g.Line().Comment("Encode headers")
				g.Id("headers").Op("=").Map(String()).Index().String().Values(DictFunc(func(dict Dict) {
					for _, f := range resp.HeaderParameters {
						encExpr, ok := genutil.MarshalQueryOrHeader(f.Type, Id("resp").Dot(f.SrcName))
						if !ok {
							d.gu.Errs.Addf(f.Type.ASTExpr().Pos(), "unsupported type in header: %s", d.gu.TypeToString(f.Type))
							continue
						}
						dict[Lit(f.WireName)] = encExpr
					}
				}))
			}
		})

		// If response is a ptr we need to check it's not nil
		if schemautil.IsPointer(d.ep.Response) {
			g.If(Id("resp").Op("!=").Nil()).Block(responseEncoder)
		} else {
			g.Add(responseEncoder)
		}

		if len(resp.HeaderParameters) > 0 {
			g.Line().Comment("Set response headers")
			g.For(List(Id("k"), Id("vs")).Op(":=").Range().Id("headers")).Block(
				For(List(Id("_"), Id("v")).Op(":=").Range().Id("vs")).Block(
					Id("w").Dot("Header").Call().Dot("Add").Call(Id("k"), Id("v")),
				),
			)
		}

		g.Line().Comment("Set HTTP status code")
		if resp.HTTPStatusParameter != nil {
			g.Id("statusCode").Op(":=").Id("status")

			var statusFieldCond *Statement
			if schemautil.IsPointer(d.ep.Response) {
				statusFieldCond = Id("resp").Op("!=").Nil().Op("&&").Id("resp").Dot(resp.HTTPStatusParameter.SrcName).Op("!=").Lit(0)
			} else {
				statusFieldCond = Id("resp").Dot(resp.HTTPStatusParameter.SrcName).Op("!=").Lit(0)
			}

			g.If(statusFieldCond).Block(
				Id("statusCode").Op("=").Int().Call(Id("resp").Dot(resp.HTTPStatusParameter.SrcName)),
			)

			g.If(Id("statusCode").Op("!=").Lit(0)).Block(
				Id("w").Dot("WriteHeader").Call(Id("statusCode")),
			)
		} else {
			g.If(Id("status").Op("!=").Lit(0)).Block(
				Id("w").Dot("WriteHeader").Call(Id("status")),
			)
		}

		g.Line().Comment("Write response body")
		g.Id("w").Dot("Write").Call(Id("respData"))
		g.Return(Nil())
	})
}

func (d *responseDesc) DecodeExternalResp() *Statement {
	if d.ep.Raw {
		// TODO(andre) support
		return Nil()
	}

	return Func().Params(
		Id("httpResp").Op("*").Qual("net/http", "Response"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
	).Params(
		Id("resp").Add(d.Type()),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		if d.ep.Response == nil {
			g.Return(d.ZeroType(), Nil())
			return
		}

		g.Id("resp").Op("=").Add(d.gu.Initialize(d.ep.Response))

		enc := d.ep.ResponseEncoding()
		dec := d.gu.NewTypeUnmarshaller("dec")
		g.Add(dec.Init())

		if enc.HTTPStatusParameter != nil {
			g.Line().Comment("Set HTTP status field")
			statusType := d.gu.Type(enc.HTTPStatusParameter.Type)
			g.Id("resp").Dot(enc.HTTPStatusParameter.SrcName).Op("=").Add(statusType).Call(Id("httpResp").Dot("StatusCode"))
		}

		apigenutil.DecodeHeaders(g, Id("httpResp").Dot("Header"), Id("resp"), dec, enc.HeaderParameters)
		apigenutil.DecodeBody(g, Id("httpResp").Dot("Body"), Id("resp"), dec, enc.BodyParameters)

		g.If(Err().Op(":=").Add(dec.Err()), Err().Op("!=").Nil()).Block(
			Return(d.ZeroType(), Err()),
		)
		g.Return(Id("resp"), Nil())
	})
}

// httpRespExpr returns an expression to access the HTTP response writer variable.
func (d *requestDesc) httpRespExpr() *Statement {
	return Id("httpResp")
}

// respDataExpr returns the expression to access the respData variable.
func (d *responseDesc) respDataExpr() *Statement {
	return Id("respData")
}

// respDataPayloadExpr returns an expression for accessing the payload
// in the reqData variable.
func (d *responseDesc) respDataPayloadExpr() *Statement {
	return d.respDataExpr()
}

// zero returns an expression representing the zero value
// of the response type.
func (d *responseDesc) zero() *Statement {
	if d.ep.Response != nil {
		return d.gu.Zero(d.ep.Response)
	} else {
		return apiQ("Void").Values()
	}
}

// Clone returns the function literal to clone the request.
func (d *responseDesc) Clone() *Statement {
	const recv = "r"
	return Func().Params(Id(recv).Add(d.Type())).Params(d.Type(), Error()).BlockFunc(func(g *Group) {
		// We could optimize the clone operation if there are no reference types (pointers, maps, slices)
		// in the struct. For now, simply serialize it as JSON and back.
		g.Var().Id("clone").Add(d.Type())
		g.List(Id("bytes"), Id("err")).Op(":=").Qual(jsonIterPkg, "ConfigDefault").Dot("Marshal").Call(Id(recv))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual(jsonIterPkg, "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)
		g.Return(Id("clone"), Err())
	})
}
