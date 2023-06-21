package endpointgen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen/apigen/apigenutil"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
)

const jsonIterPkg = "github.com/json-iterator/go"

// requestDesc describes the generated request type that contains the combined
// request data + path parameters for the request.
type requestDesc struct {
	gu *genutil.Helper
	ep *api.Endpoint
}

func (d *requestDesc) TypeName() string {
	return "EncoreInternal_" + d.ep.Name + "Req"
}

func (d *requestDesc) Type() *Statement {
	return Op("*").Id(d.TypeName())
}

func (d *requestDesc) TypeDecl() *Statement {
	return Type().Id(d.TypeName()).StructFunc(func(g *Group) {
		if d.ep.Request != nil {
			g.Id(d.reqDataPayloadName()).Add(d.gu.Type(d.ep.Request))
		}
		for i, seg := range d.ep.Path.Params() {
			g.Id(d.pathParamFieldName(i)).Add(d.gu.Builtin(d.ep.Decl.AST.Pos(), seg.ValueType))
		}
	})
}

func (d *requestDesc) DecodeRequest() *Statement {
	return Func().Params(
		d.httpReqExpr().Op("*").Qual("net/http", "Request"),
		d.pathParamsName().Add(apiQ("UnnamedParams")),
		Id("json").Qual(jsonIterPkg, "API"),
	).Params(
		d.reqDataExpr().Add(d.Type()),
		Id("pathParams").Add(apiQ("UnnamedParams")),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		g.Add(d.reqDataExpr()).Op("=").New(Id(d.TypeName()))

		if d.ep.Path.NumParams() == 0 && d.ep.Request == nil {
			// Nothing to do; return an empty struct
			g.Return(d.reqDataExpr(), Nil(), Nil())
			return
		}

		dec := d.gu.NewTypeUnmarshaller("dec")
		g.Add(dec.Init())
		d.renderPathDecoding(g, dec)
		d.renderRequestDecoding(g, dec)

		g.If(Err().Op(":=").Add(dec.Err()), Err().Op("!=").Nil()).Block(
			Return(Nil(), Nil(), Err()),
		)

		g.Return(d.reqDataExpr(), d.pathParamsName(), Nil())
	})
}

// HandlerArgs returns the list of arguments to pass to the handler.
func (d *requestDesc) HandlerArgs() []Code {
	numPathParams := d.ep.Path.NumParams()
	args := make([]Code, 0, 1+numPathParams)
	for i := 0; i < numPathParams; i++ {
		args = append(args, d.reqDataPathParamExpr(i))
	}
	if d.ep.Request != nil {
		args = append(args, d.reqDataPayloadExpr())
	}
	return args
}

// renderPathDecoding renders the code to decode the path parameters.
// The path parameters are accessible via the `pathParamsExpr` parameter.
//
// The generated code writes to the path segment fields in the request struct,
// which is accessed via the `reqDescExpr` parameter.
func (d *requestDesc) renderPathDecoding(g *Group, dec *genutil.TypeUnmarshaller) {
	// Collect all the non-literal path segments, and keep track of the wildcard segment, if any.
	segs := make([]resourcepaths.Segment, 0, len(d.ep.Path.Segments))
	seenWildcard := false
	wildcardIdx := 0
	for _, s := range d.ep.Path.Segments {
		if s.Type != resourcepaths.Literal {
			segs = append(segs, s)
		}
		if !seenWildcard {
			// Fallback is also considered a wildcard for these purposes.
			if s.Type == resourcepaths.Wildcard || s.Type == resourcepaths.Fallback {
				seenWildcard = true
			} else if s.Type == resourcepaths.Param {
				wildcardIdx++
			}
		}
	}

	if seenWildcard {
		g.Comment("Trim the leading slash from wildcard parameter, as Encore's semantics excludes it,")
		g.Comment("while the httprouter implementation includes it.")
		g.Add(d.pathSegmentValue(wildcardIdx)).Op("=").Qual("strings", "TrimPrefix").Call(
			d.pathSegmentValue(wildcardIdx), Lit("/"))
		g.Line()
	}

	// Decode the path params
	for segIdx, seg := range segs {
		pathSegmentValue := d.pathSegmentValue(segIdx)

		// If the segment type is a string, then we want to unescape it.
		switch seg.ValueType {
		case schema.String, schema.UUID:
			g.If(
				List(Id("value"), Err()).Op(":=").Qual("net/url", "PathUnescape").Call(pathSegmentValue),
				Err().Op("==").Nil().Block(
					d.pathSegmentValue(segIdx).Op("=").Id("value"),
				),
			)
		}

		g.Do(func(s *Statement) {
			// If it's a raw endpoint the params are not used, but validate them regardless.
			if d.ep.Raw {
				s.Id("_").Op("=")
			} else {
				s.Add(d.reqDataPathParamExpr(segIdx).Op("="))
			}
		}).Add(dec.UnmarshalBuiltin(seg.ValueType, seg.Value, pathSegmentValue, true))
	}
}

// httpReqExpr returns an expression to access the HTTP request variable
func (d *requestDesc) httpReqExpr() *Statement {
	return Id("httpReq")
}

// reqDataExpr returns the expression to access the reqData variable.
func (d *requestDesc) reqDataExpr() *Statement {
	return Id("reqData")
}

// reqDataPayloadName returns the name of the payload field in the reqData struct.
func (d *requestDesc) reqDataPayloadName() string {
	return "Payload"
}

// httpHeaderExpr returns an expression to access the HTTP header variable,
// for request encoding.
func (d *requestDesc) httpHeaderExpr() *Statement {
	return Id("httpHeader")
}

// queryStringExpr returns an expression to access the url.Values query string variable,
// for request encoding.
func (d *requestDesc) queryStringExpr() *Statement {
	return Id("queryString")
}

// jsonStream returns an expression to access the *jsoniter.Stream variable,
// for request encoding.
func (d *requestDesc) jsonStream() *Statement {
	return Id("stream")
}

// reqDataPayloadExpr returns an expression for accessing the payload
// in the reqData variable.
func (d *requestDesc) reqDataPayloadExpr() *Statement {
	return d.reqDataExpr().Dot(d.reqDataPayloadName())
}

// reqDataPathParamExpr returns an expression for accessing the i'th path parameter
// in the reqData variable.
func (d *requestDesc) reqDataPathParamExpr(i int) *Statement {
	return d.reqDataExpr().Dot(d.pathParamFieldName(i))
}

// reqDataPathParamName returns the field name for the i'th path parameter
// in the reqData struct.
func (d *requestDesc) pathParamFieldName(i int) string {
	return fmt.Sprintf("P%d", i)
}

// pathParamsName renders an expression for the name of the path parameters.
func (d *requestDesc) pathParamsName() *Statement {
	return Id("ps")
}

// pathSegmentValue renders an expression to retrieve the value (as a string) of the i'th path segment.
func (d *requestDesc) pathSegmentValue(i int) *Statement {
	return d.pathParamsName().Index(Lit(i))
}

func (d *requestDesc) renderRequestDecoding(g *Group, dec *genutil.TypeUnmarshaller) {
	if d.ep.Request == nil {
		return
	}

	if schemautil.IsPointer(d.ep.Request) {
		g.Id("params").Op(":=").Add(d.gu.Initialize(d.ep.Request))
		g.Add(d.reqDataPayloadExpr()).Op("=").Id("params")
	} else {
		g.Id("params").Op(":=").Op("&").Add(d.reqDataPayloadExpr())
	}

	// Parsing requests for HTTP methods without a body (GET, HEAD, DELETE) are handled by parsing the query string,
	// while other methods are parsed by reading the body and unmarshalling it as JSON.
	// If the same endpoint supports both, handle it with a switch.
	reqs := d.ep.RequestEncoding()
	if d.gu.Errs.Len() > 0 {
		return
	}
	g.Add(Switch(Id("m").Op(":=").Add(d.httpReqExpr()).Dot("Method"), Id("m")).BlockFunc(
		func(g *Group) {
			for _, r := range reqs {
				g.CaseFunc(func(g *Group) {
					for _, m := range r.HTTPMethods {
						g.Lit(m)
					}
				}).BlockFunc(func(g *Group) {
					d.decodeRequestParameters(g, dec, r)
				})
			}
			g.Default().Add(Id("panic").Call(Lit("HTTP method is not supported")))
		},
	))
}

func (d *requestDesc) decodeRequestParameters(g *Group, dec *genutil.TypeUnmarshaller, req *apienc.RequestEncoding) {
	apigenutil.DecodeHeaders(g, d.httpReqExpr().Dot("Header"), Id("params"), dec, req.HeaderParameters)
	apigenutil.DecodeQuery(g, d.httpReqExpr().Dot("URL").Dot("Query").Call(), Id("params"), dec, req.QueryParameters)
	apigenutil.DecodeBody(g, d.httpReqExpr().Dot("Body"), Id("params"), dec, req.BodyParameters)
}

// Clone returns the function literal to clone the request.
func (d *requestDesc) Clone() *Statement {
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

// ReqPath returns the function literal to compute the request path.
func (d *requestDesc) ReqPath() *Statement {
	return Func().Params(
		d.reqDataExpr().Add(d.Type()),
	).Params(
		String(),
		apiQ("UnnamedParams"),
		Error(),
	).BlockFunc(func(g *Group) {
		pathParams := d.ep.Path.Params()
		if len(pathParams) == 0 {
			g.Return(Lit(d.ep.Path.String()), Nil(), Nil())
			return
		}

		g.Id("params").Op(":=").Add(apiQ("UnnamedParams")).ValuesFunc(func(g *Group) {
			for paramIdx, f := range pathParams {
				g.Add(genutil.MarshalBuiltin(f.ValueType, d.reqDataPathParamExpr(paramIdx)))
			}
		})

		// Construct the path as an expression in the form
		//		"/foo" + params[N].Value + "/bar"
		pathExpr := CustomFunc(Options{
			Separator: " + ",
		}, func(g *Group) {
			idx := 0
			for _, seg := range d.ep.Path.Segments {
				if seg.Type == resourcepaths.Literal {
					g.Lit("/" + seg.Value)
				} else {
					g.Lit("/")
					g.Id("params").Index(Lit(idx))
					idx++
				}
			}
		})
		g.Return(pathExpr, Id("params"), Nil())
	})
}

// UserPayload returns the function literal to compute the user payload.
func (d *requestDesc) UserPayload() *Statement {
	return Func().Params(
		// input
		d.reqDataExpr().Add(d.Type()),
	).Params(
		// output
		Any(),
	).BlockFunc(func(g *Group) {
		if d.ep.Request == nil {
			g.Return(Nil())
		} else {
			g.Return(d.reqDataPayloadExpr())
		}
	})
}

func (d *requestDesc) EncodeExternalReq() *Statement {
	return Func().Params(
		d.reqDataExpr().Add(d.Type()),
		d.jsonStream().Add(Op("*").Qual(jsonIterPkg, "Stream")),
	).Params(
		d.httpHeaderExpr().Add(Qual("net/http", "Header")),
		d.queryStringExpr().Add(Qual("net/url", "Values")),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		if d.ep.Request == nil {
			// Nothing to do.
			g.Return(Nil(), Nil(), Nil())
			return
		}

		if schemautil.IsPointer(d.ep.Request) {
			g.Id("params").Op(":=").Add(d.reqDataPayloadExpr())

			g.If(Id("params").Op("==").Nil()).Block(
				Comment("If the payload is nil, we need to return an empty request body."),
				Return(d.httpHeaderExpr(), d.queryStringExpr(), Err()),
			)
		} else {
			g.Id("params").Op(":=").Op("&").Add(d.reqDataPayloadExpr())
		}

		encodings := d.ep.RequestEncoding()
		if len(encodings) == 0 {
			// This only happens in the case of an error, which has
			// then already been reported by RequestEncoding.
			g.Return(Nil(), Nil(), Nil())
			return
		}

		enc := encodings[0]

		apigenutil.EncodeHeaders(d.gu.Errs, g, d.httpHeaderExpr(), Id("params"), enc.HeaderParameters)
		apigenutil.EncodeQuery(d.gu.Errs, g, d.queryStringExpr(), Id("params"), enc.QueryParameters)
		apigenutil.EncodeBody(d.gu, g, d.jsonStream(), Id("params"), enc.BodyParameters)

		g.Return(d.httpHeaderExpr(), d.queryStringExpr(), Err())
	})
}
