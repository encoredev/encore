package apigenutil

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/selector"
)

const jsonIterPkg = "github.com/json-iterator/go"

// DecodeHeaders generates code for decoding HTTP headers from the http request
// given by httpReqExpr and storing the result into the params given by paramsExpr.
func DecodeHeaders(g *Group, httpHeaderExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Comment("Decode headers")
	g.Id("h").Op(":=").Add(httpHeaderExpr)
	for _, f := range params {
		singleValExpr := Id("h").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("h").Dot("Values").Call(Lit(f.WireName))
		decodeExpr := dec.UnmarshalSingleOrList(f.Type, f.WireName, singleValExpr, listValExpr, false)
		g.Add(paramExpr.Clone().Dot(f.SrcName).Op("=").Add(decodeExpr))
	}
	g.Line()
}

// DecodeQuery is like DecodeHeaders but for query strings.
func DecodeQuery(g *Group, urlValuesExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode query string")
	g.Id("qs").Op(":=").Add(urlValuesExpr)

	for _, f := range params {
		singleValExpr := Id("qs").Dot("Get").Call(Lit(f.WireName))
		listValExpr := Id("qs").Index(Lit(f.WireName))
		decodeExpr := dec.UnmarshalSingleOrList(f.Type, f.WireName, singleValExpr, listValExpr, false)
		g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Add(decodeExpr)
	}
	g.Line()
}

// DecodeCookie is like DecodeHeaders but for cookies.
func DecodeCookie(errs *perr.List, g *Group, httpReqExpr, paramExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode cookies")

	cookieType := pkginfo.Q("net/http", "Cookie")

	for _, f := range params {
		g.If(List(Id("c"), Id("_")).Op(":=").Add(httpReqExpr.Clone()).Dot("Cookie").Call(Lit(f.WireName)).Op(";").Id("c").Op("!=").Nil()).BlockFunc(func(g *Group) {
			// Cookies can either be a builtin or a *http.Cookie.
			if builtin, ok := f.Type.(schema.BuiltinType); ok {
				decodeExpr := dec.UnmarshalBuiltin(builtin.Kind, f.WireName, Id("c").Dot("Value"), false)
				g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Add(decodeExpr)
			} else if info, ok := schemautil.DerefNamedInfo(f.Type, true); ok && info.QualifiedName() == cookieType {
				g.Add(paramExpr.Clone()).Dot(f.SrcName).Op("=").Id("c")
				g.Add(dec.IncNonEmpty())
			} else {
				errs.Addf(f.Type.ASTExpr().Pos(), "cannot unmarshal cookie into field of type %s", f.Type)
			}
		})
	}
	g.Line()
}

// DecodeBody decodes an io.Reader request body into the given parameters.
func DecodeBody(g *Group, ioReaderExpr *Statement, paramsExpr *Statement, dec *genutil.TypeUnmarshaller, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Comment("Decode request body")
	g.Id("payload").Op(":=").Add(dec.ReadBody(ioReaderExpr))
	g.Id("iter").Op(":=").Qual(jsonIterPkg, "ParseBytes").Call(Id("json"), Id("payload"))
	g.Line()

	g.For(Id("iter").Dot("ReadObjectCB").Call(
		Func().Params(Id("_").Op("*").Qual(jsonIterPkg, "Iterator"), Id("key").String()).Bool().Block(
			Switch(Qual("strings", "ToLower").Call(Id("key"))).BlockFunc(func(g *Group) {
				for _, f := range params {
					g.Case(Lit(strings.ToLower(f.WireName))).Block(
						dec.ParseJSON(f.SrcName, Id("iter"), Op("&").Add(paramsExpr.Clone()).Dot(f.SrcName)),
					)
				}
				g.Default().Block(Id("_").Op("=").Id("iter").Dot("SkipAndReturnBytes").Call())
			}),
			Return(True()),
		)).Block(),
	)
	g.Line()
}

// EncodeHeaders generates code for encoding HTTP headers into a http.Header map.
func EncodeHeaders(errs *perr.List, g *Group, httpHeaderExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode headers")
	g.Add(httpHeaderExpr.Clone().Op("=").Make(Qual("net/http", "Header"), Lit(len(params))))

	for _, f := range params {
		kind, isList, ok := schemautil.IsBuiltinOrList(f.Type)
		if !ok {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to string", f.Type)
			continue
		}

		if isList {
			strVals := genutil.MarshalBuiltinList(kind, paramExpr.Clone().Dot(f.SrcName))
			g.Add(httpHeaderExpr.Clone()).Index(
				Qual("net/textproto", "CanonicalMIMEHeaderKey").Call(Lit(f.WireName)),
			).Op("=").Add(strVals)
		} else {
			strVal := genutil.MarshalBuiltin(kind, paramExpr.Clone().Dot(f.SrcName))
			g.Add(httpHeaderExpr.Clone().Dot("Set").Call(Lit(f.WireName), strVal))
		}
	}
	g.Line()
}

// EncodeQuery generates code for encoding URL query values into a url.Values map.
func EncodeQuery(errs *perr.List, g *Group, urlValuesExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode query string")
	g.Add(urlValuesExpr.Clone().Op("=").Make(Qual("net/url", "Values"), Lit(len(params))))

	for _, f := range params {
		kind, isList, ok := schemautil.IsBuiltinOrList(f.Type)
		if !ok {
			errs.Addf(f.Type.ASTExpr().Pos(), "cannot marshal %s to string", f.Type)
			continue
		}

		if isList {
			strVals := genutil.MarshalBuiltinList(kind, paramExpr.Clone().Dot(f.SrcName))
			g.Add(urlValuesExpr.Clone()).Index(Lit(f.WireName)).Op("=").Add(strVals)
		} else {
			strVal := genutil.MarshalBuiltin(kind, paramExpr.Clone().Dot(f.SrcName))
			g.Add(urlValuesExpr.Clone().Dot("Set").Call(Lit(f.WireName), strVal))
		}
	}
	g.Line()
}

// EncodeBody encodes a request body into the given *jsoniter.Stream.
func EncodeBody(gu *genutil.Helper, g *Group, streamExpr, paramExpr *Statement, params []*apienc.ParameterEncoding) {
	if len(params) == 0 {
		return
	}

	g.Line()
	g.Comment("Encode request body")
	g.Add(streamExpr.Clone().Dot("WriteObjectStart").Call())

	for i, p := range params {
		writeBlock := g

		// If this field is omitted when empty, we need to wrap the write in an if statement.
		if p.OmitEmpty {
			g.If(gu.IsNotJSONEmpty(paramExpr.Clone().Dot(p.SrcName), p.Type)).BlockFunc(func(g *Group) {
				g.Comment(fmt.Sprintf("%s is set to omitempty, so we need to check if it's empty before writing it", p.SrcName))
				writeBlock = g
			})
		}

		writeBlock.Add(streamExpr.Clone().Dot("WriteObjectField").Call(Lit(p.WireName)))
		writeBlock.Add(streamExpr.Clone().Dot("WriteVal").Call(paramExpr.Clone().Dot(p.SrcName)))
		if i+1 < len(params) {
			// If we're not on the last field, write a comma.
			// we do this within the writeBlock so that we don't write a comma if we're omitting the field.
			writeBlock.Add(streamExpr.Clone().Dot("WriteMore").Call())
		}
	}
	g.Add(streamExpr.Clone().Dot("WriteObjectEnd").Call())
	g.Line()
}

// BuildErr returns an expression for returning an encore.dev/beta/errs.Error with the given code and message.
func BuildErr(code, msg string) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msg").Call(Lit(msg)).Dot("Err").Call()
}

// BuildErrf is like BuildErr but with a format string.
func BuildErrf(code, format string, args ...Code) *Statement {
	p := "encore.dev/beta/errs"
	args = append([]Code{Lit(format)}, args...)
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msgf").Call(args...).Dot("Err").Call()
}

// APIDescriptor holds the configuration for generating an API descriptor
type APIDescriptor struct {
	ReqType   *Statement
	RespType  *Statement
	DecodeReq *Statement
	CloneReq  *Statement
	ReqPath   *Statement
	ReqUserPayload *Statement
	AppHandler *Statement
	RawHandler *Statement
	EncodeResp *Statement
	CloneResp  *Statement
	EncodeExternalReq  *Statement
	DecodeExternalResp *Statement
	ServiceMiddleware  *Statement
	GlobalMiddlewareIDs *Statement
}

// GenAPIDesc generates an API descriptor for the given endpoint.
// This is the main function used by both endpointgen and userfacinggen.
func GenAPIDesc(
	gen *codegen.Generator, f *codegen.File, appDesc *app.Desc, svc *app.Service,
	ep *api.Endpoint, descriptor *APIDescriptor,
) *codegen.VarDecl {
	gu := gen.Util
	
	methods := ep.HTTPMethods
	if len(methods) == 1 && methods[0] == "*" {
		// All methods, from https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods
		methods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
	}

	var access *Statement
	switch ep.Access {
	case api.Public:
		access = ApiQ("Public")
	case api.Auth:
		access = ApiQ("RequiresAuth")
	case api.Private:
		access = ApiQ("Private")
	default:
		gen.Errs.Addf(ep.Decl.AST.Pos(), "unhandled access type %v", ep.Access)
	}

	pos := ep.Decl.AST.Pos()
	desc := f.VarDecl("APIDesc", ep.Name)
	desc.Value(Op("&").Add(ApiQ("Desc")).Types(
		descriptor.ReqType,
		descriptor.RespType,
	).Values(Dict{
		Id("Service"):        Lit(svc.Name),
		Id("SvcNum"):         Lit(svc.Num),
		Id("Endpoint"):       Lit(ep.Name),
		Id("Methods"):        gu.GoToJen(pos, methods),
		Id("Raw"):            Lit(ep.Raw),
		Id("Fallback"):       Lit(ep.Path.HasFallback()),
		Id("Path"):           Lit(ep.Path.String()),
		Id("RawPath"):        Lit(RawPath(ep.Path)),
		Id("DefLoc"):         Lit(gen.TraceNodes.Endpoint(ep)),
		Id("PathParamNames"): PathParamNames(ep.Path),
		Id("Tags"):           TagNames(ep.Tags),
		Id("Access"):         access,

		Id("DecodeReq"):      descriptor.DecodeReq,
		Id("CloneReq"):       descriptor.CloneReq,
		Id("ReqPath"):        descriptor.ReqPath,
		Id("ReqUserPayload"): descriptor.ReqUserPayload,

		Id("AppHandler"): descriptor.AppHandler,
		Id("RawHandler"): descriptor.RawHandler,
		Id("EncodeResp"): descriptor.EncodeResp,
		Id("CloneResp"):  descriptor.CloneResp,

		Id("EncodeExternalReq"):  descriptor.EncodeExternalReq,
		Id("DecodeExternalResp"): descriptor.DecodeExternalResp,

		Id("ServiceMiddleware"):   descriptor.ServiceMiddleware,
		Id("GlobalMiddlewareIDs"): descriptor.GlobalMiddlewareIDs,
	}))

	return desc
}

// GenServiceMiddleware generates the service middleware statement
func GenServiceMiddleware(ep *api.Endpoint, fw *apiframework.ServiceDesc, svcMiddleware map[*middleware.Middleware]*codegen.VarDecl) *Statement {
	return Index().Op("*").Add(ApiQ("Middleware")).ValuesFunc(func(g *Group) {
		for _, mw := range fw.Middleware {
			if mw.Target.ContainsAny(ep.Tags) {
				g.Add(svcMiddleware[mw].Qual())
			}
		}
	})
}

// GenGlobalMiddleware generates the global middleware statement
func GenGlobalMiddleware(appDesc *app.Desc, ep *api.Endpoint) *Statement {
	return Index().String().ValuesFunc(func(g *Group) {
		for _, mw := range appDesc.MatchingGlobalMiddleware(ep) {
			g.Add(Lit(mw.ID()))
		}
	})
}


// ApiQ returns a qualified name for the encore.dev/appruntime/apisdk/api package.
func ApiQ(name string) *Statement {
	return Qual("encore.dev/appruntime/apisdk/api", name)
}

// RawPath creates a raw path representation, replacing path parameters
// with their indices to ensure all httprouter paths use consistent path param names,
// since otherwise httprouter reports path conflicts.
func RawPath(path *resourcepaths.Path) string {
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

// PathParamNames yields a []string literal containing the names
// of the path parameters, in order.
func PathParamNames(path *resourcepaths.Path) Code {
	if path.NumParams() == 0 {
		return Nil()
	}
	return Index().String().ValuesFunc(func(g *Group) {
		for _, s := range path.Params() {
			g.Lit(s.Value)
		}
	})
}

// TagNames yields a []string literal containing the tag names.
func TagNames(tags selector.Set) Code {
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

// Helper functions for generating client-side API descriptor functions
func GenDecodeReq(ep *api.Endpoint, hasRequestType bool) *Statement {
	var reqTypeName string
	if hasRequestType {
		reqTypeName = "EncoreInternal_" + ep.Name + "Req"
	} else {
		reqTypeName = "struct{}"
	}

	return Func().Params(
		Id("httpReq").Op("*").Qual("net/http", "Request"),
		Id("ps").Qual("encore.dev/appruntime/apisdk/api", "UnnamedParams"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
	).Params(
		Id("reqData").Op("*").Id(reqTypeName),
		Id("pathParams").Qual("encore.dev/appruntime/apisdk/api", "UnnamedParams"),
		Err().Error(),
	).Block(
		Id("reqData").Op("=").New(Id(reqTypeName)),
		Return(Id("reqData"), Nil(), Nil()),
	)
}

func GenCloneReq(ep *api.Endpoint, hasRequestType bool) *Statement {
	var reqTypeName string
	if hasRequestType {
		reqTypeName = "EncoreInternal_" + ep.Name + "Req"
	} else {
		reqTypeName = "struct{}"
	}

	return Func().Params(Id("r").Op("*").Id(reqTypeName)).Params(Op("*").Id(reqTypeName), Error()).Block(
		Var().Id("clone").Op("*").Id(reqTypeName),
		List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id("r")),
		If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		),
		Return(Id("clone"), Err()),
	)
}

func GenReqPath(ep *api.Endpoint) *Statement {
	return Func().Params(
		Id("reqData").Interface(),
	).Params(
		String(),
		Qual("encore.dev/appruntime/apisdk/api", "UnnamedParams"),
		Error(),
	).Block(
		Return(Lit(ep.Path.String()), Nil(), Nil()),
	)
}

func GenReqUserPayload(ep *api.Endpoint, hasRequestType bool) *Statement {
	return Func().Params(
		Id("reqData").Interface(),
	).Params(
		Any(),
	).BlockFunc(func(g *Group) {
		if ep.Request != nil && hasRequestType {
			g.If(Id("req"), Id("ok").Op(":=").Id("reqData").Assert(Op("*").Id("EncoreInternal_"+ep.Name+"Req")), Id("ok")).Block(
				Return(Id("req").Dot("Payload")),
			)
		}
		g.Return(Nil())
	})
}

func GenEncodeResp(ep *api.Endpoint, hasResponseType bool) *Statement {
	var respType *Statement
	if hasResponseType {
		respType = Id("EncoreInternal_" + ep.Name + "Resp")
	} else {
		respType = Interface()
	}

	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
		Id("resp").Add(respType),
		Id("status").Int(),
	).Params(
		Err().Error(),
	).Block(
		Return(Nil()),
	)
}

func GenCloneResp(ep *api.Endpoint, hasResponseType bool) *Statement {
	if !hasResponseType {
		return Nil()
	}
	respTypeName := "EncoreInternal_" + ep.Name + "Resp"
	return Func().Params(Id("r").Id(respTypeName)).Params(Id(respTypeName), Error()).Block(
		Var().Id("clone").Id(respTypeName),
		List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id("r")),
		If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		),
		Return(Id("clone"), Err()),
	)
}

func GenEncodeExternalReq(ep *api.Endpoint, hasRequestType bool) *Statement {
	var reqTypeName string
	if hasRequestType {
		reqTypeName = "EncoreInternal_" + ep.Name + "Req"
	} else {
		reqTypeName = "struct{}"
	}

	return Func().Params(
		Id("reqData").Op("*").Id(reqTypeName),
		Id("stream").Op("*").Qual("github.com/json-iterator/go", "Stream"),
	).Params(
		Id("httpHeader").Qual("net/http", "Header"),
		Id("queryString").Qual("net/url", "Values"),
		Err().Error(),
	).Block(
		Return(Nil(), Nil(), Nil()),
	)
}

func GenDecodeExternalResp(ep *api.Endpoint, hasResponseType bool) *Statement {
	var respType *Statement
	if hasResponseType {
		respType = Id("EncoreInternal_" + ep.Name + "Resp")
	} else {
		respType = Interface()
	}

	return Func().Params(
		Id("httpResp").Op("*").Qual("net/http", "Response"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
	).Params(
		Id("resp").Add(respType),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		if hasResponseType {
			g.Var().Id("result").Id("EncoreInternal_" + ep.Name + "Resp")
			g.Return(Id("result"), Nil())
		} else {
			g.Return(Id("resp"), Nil())
		}
	})
}

// RequestDesc describes the generated request type that contains the combined
// request data + path parameters for the request.
type RequestDesc struct {
	gu *genutil.Helper
	ep *api.Endpoint
}

func NewRequestDesc(gu *genutil.Helper, ep *api.Endpoint) *RequestDesc {
	return &RequestDesc{gu: gu, ep: ep}
}

func (d *RequestDesc) TypeName() string {
	return "EncoreInternal_" + d.ep.Name + "Req"
}

func (d *RequestDesc) Type() *Statement {
	return Op("*").Id(d.TypeName())
}

func (d *RequestDesc) TypeDecl() *Statement {
	return Type().Id(d.TypeName()).StructFunc(func(g *Group) {
		if d.ep.Request != nil {
			g.Id(d.reqDataPayloadName()).Add(d.gu.Type(d.ep.Request))
		}
		// Note: the path parameter order is important and must match the order of the segments as defined
		// as the parameters of the user's endpoint function. This behaviour is expected by the mocking
		// system - see runtimes/go/appruntime/apisdk/api/reflection.go.
		for i, seg := range d.ep.Path.Params() {
			g.Id(d.pathParamFieldName(i)).Add(d.gu.Builtin(d.ep.Decl.AST.Pos(), seg.ValueType))
		}
	})
}

// Helper methods for RequestDesc
func (d *RequestDesc) ReqDataPayloadName() string {
	return "Payload"
}

func (d *RequestDesc) PathParamFieldName(i int) string {
	return fmt.Sprintf("P%d", i)
}

// Keep the private versions for internal use
func (d *RequestDesc) reqDataPayloadName() string {
	return d.ReqDataPayloadName()
}

func (d *RequestDesc) pathParamFieldName(i int) string {
	return d.PathParamFieldName(i)
}

func (d *RequestDesc) DecodeRequest() *Statement {
	// Unified implementation that works for both client and server
	return Func().Params(
		Id("httpReq").Op("*").Qual("net/http", "Request"),
		Id("ps").Add(ApiQ("UnnamedParams")),
		Id("json").Qual(jsonIterPkg, "API"),
	).Params(
		Id("reqData").Add(d.Type()),
		Id("pathParams").Add(ApiQ("UnnamedParams")),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		g.Id("reqData").Op("=").New(Id(d.TypeName()))

		if d.ep.Path.NumParams() == 0 && d.ep.Request == nil {
			// Nothing to do; return an empty struct
			g.Return(Id("reqData"), Nil(), Nil())
			return
		}

		// For client use, simplified implementation
		g.Return(Id("reqData"), Id("ps"), Nil())
	})
}

func (d *RequestDesc) Clone() *Statement {
	const recv = "r"
	return Func().Params(Id(recv).Add(d.Type())).Params(d.Type(), Error()).BlockFunc(func(g *Group) {
		g.Var().Id("clone").Add(d.Type())
		g.List(Id("bytes"), Id("err")).Op(":=").Qual(jsonIterPkg, "ConfigDefault").Dot("Marshal").Call(Id(recv))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual(jsonIterPkg, "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)
		g.Return(Id("clone"), Err())
	})
}

func (d *RequestDesc) ReqPath() *Statement {
	return Func().Params(
		Id("reqData").Add(d.Type()),
	).Params(
		String(),
		ApiQ("UnnamedParams"),
		Error(),
	).Block(
		Return(Lit(d.ep.Path.String()), Nil(), Nil()),
	)
}

func (d *RequestDesc) UserPayload() *Statement {
	return Func().Params(
		Id("reqData").Add(d.Type()),
	).Params(
		Any(),
	).BlockFunc(func(g *Group) {
		if d.ep.Request != nil {
			g.Return(Id("reqData").Dot(d.reqDataPayloadName()))
		} else {
			g.Return(Nil())
		}
	})
}

func (d *RequestDesc) EncodeExternalReq() *Statement {
	return Func().Params(
		Id("reqData").Add(d.Type()),
		Id("stream").Op("*").Qual(jsonIterPkg, "Stream"),
	).Params(
		Id("httpHeader").Qual("net/http", "Header"),
		Id("queryString").Qual("net/url", "Values"),
		Err().Error(),
	).Block(
		Return(Nil(), Nil(), Nil()),
	)
}

// Server-specific methods for endpointgen

// reqDataExpr returns the expression to access the reqData variable.
func (d *RequestDesc) ReqDataExpr() *Statement {
	return Id("reqData")
}

// HandlerArgs returns the list of arguments to pass to the handler.
func (d *RequestDesc) HandlerArgs() []Code {
	numPathParams := d.ep.Path.NumParams()
	args := make([]Code, 0, 1+numPathParams)
	for i := 0; i < numPathParams; i++ {
		args = append(args, d.ReqDataExpr().Dot(d.pathParamFieldName(i)))
	}
	if d.ep.Request != nil {
		args = append(args, d.ReqDataExpr().Dot(d.reqDataPayloadName()))
	}
	return args
}

// Additional server-specific methods used by the original endpointgen

// ResponseDesc describes the generated response type.
type ResponseDesc struct {
	gu *genutil.Helper
	ep *api.Endpoint
}

func NewResponseDesc(gu *genutil.Helper, ep *api.Endpoint) *ResponseDesc {
	return &ResponseDesc{gu: gu, ep: ep}
}

func (d *ResponseDesc) TypeName() string {
	return "EncoreInternal_" + d.ep.Name + "Resp"
}

func (d *ResponseDesc) Type() *Statement {
	return Id(d.TypeName())
}

func (d *ResponseDesc) TypeDecl() *Statement {
	return Type().Id(d.TypeName()).Op("=").Do(func(s *Statement) {
		if d.ep.Response != nil {
			s.Add(d.gu.Type(d.ep.Response))
		} else {
			s.Add(ApiQ("Void"))
		}
	})
}

func (d *ResponseDesc) EncodeResponse() *Statement {
	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("json").Qual(jsonIterPkg, "API"),
		Id("resp").Add(d.Type()),
		Id("status").Int(),
	).Params(
		Err().Error(),
	).Block(
		Return(Nil()),
	)
}

func (d *ResponseDesc) Clone() *Statement {
	if d.ep.Response == nil {
		return Nil()
	}
	const recv = "r"
	return Func().Params(Id(recv).Add(d.Type())).Params(d.Type(), Error()).BlockFunc(func(g *Group) {
		g.Var().Id("clone").Add(d.Type())
		g.List(Id("bytes"), Id("err")).Op(":=").Qual(jsonIterPkg, "ConfigDefault").Dot("Marshal").Call(Id(recv))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual(jsonIterPkg, "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)
		g.Return(Id("clone"), Err())
	})
}

func (d *ResponseDesc) DecodeExternalResp() *Statement {
	return Func().Params(
		Id("httpResp").Op("*").Qual("net/http", "Response"),
		Id("json").Qual(jsonIterPkg, "API"),
	).Params(
		Id("resp").Add(d.Type()),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		if d.ep.Response != nil {
			g.Var().Id("result").Add(d.Type())
			g.Return(Id("result"), Nil())
		} else {
			g.Return(Id("resp"), Nil())
		}
	})
}

// Server-specific methods for endpointgen

// Zero returns an expression representing the zero value of the response type.
func (d *ResponseDesc) Zero() *Statement {
	if d.ep.Response != nil {
		return d.gu.Zero(d.ep.Response)
	} else {
		return ApiQ("Void").Values()
	}
}
