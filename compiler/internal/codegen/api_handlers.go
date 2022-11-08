package codegen

import (
	"fmt"
	gotoken "go/token"
	"path"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	"encr.dev/pkg/namealloc"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

var importNames = map[string]string{
	"github.com/felixge/httpsnoop":        "httpsnoop",
	"github.com/json-iterator/go":         "jsoniter",
	"github.com/julienschmidt/httprouter": "httprouter",

	"encore.dev/appruntime/api":         "__api",
	"encore.dev/appruntime/app":         "__app",
	"encore.dev/appruntime/app/appinit": "__appinit",
	"encore.dev/appruntime/config":      "__config",
	"encore.dev/appruntime/model":       "__model",
	"encore.dev/appruntime/serde":       "__serde",
	"encore.dev/appruntime/service":     "__service",
	"encore.dev/beta/errs":              "errs",
	"encore.dev/storage/sqldb":          "sqldb",
	"encore.dev/types/uuid":             "uuid",
}

func (b *Builder) registerImports(f *File, fromPkgPath string) {
	for pkgPath, alias := range importNames {
		f.ImportAlias(pkgPath, alias)
	}

	f.ImportAlias("encoding/json", "stdjson")

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it will be imported under its proper name.
	f.Anon("encore.dev/appruntime/app/appinit")

	for _, pkg := range b.res.App.Packages {
		f.ImportName(pkg.ImportPath, pkg.Name)
	}

	f.ImportName(path.Join(b.res.Meta.ModulePath, "__encore", "etype"), "etype")
}

func (b *Builder) buildRPC(f *File, svc *est.Service, rpc *est.RPC) {
	bb := &rpcBuilder{
		Builder:  b,
		f:        f,
		svc:      svc,
		rpc:      rpc,
		reqType:  newStructDesc("p"),
		respType: newStructDesc("p"),
	}
	bb.Write(f)
}

func (b *Builder) rpcHandlerName(rpc *est.RPC) string {
	return fmt.Sprintf("EncoreInternal_%sHandler", rpc.Name)
}

type rpcBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	rpc *est.RPC

	reqType  *structDesc
	respType *structDesc
}

func (b *rpcBuilder) Write(f *File) {
	decodeReq := b.renderDecodeReq()
	encodeResp := b.renderEncodeResp()
	reqDesc := b.renderRequestStructDesc(b.ReqTypeName(), b.reqType)
	respDesc := b.renderResponseStructDesc()

	rpc := b.rpc

	var access *Statement
	switch rpc.Access {
	case est.Public:
		access = Qual("encore.dev/appruntime/api", "Public")
	case est.Auth:
		access = Qual("encore.dev/appruntime/api", "RequiresAuth")
	case est.Private:
		access = Qual("encore.dev/appruntime/api", "Private")
	default:
		b.errors.Addf(rpc.Func.Pos(), "unhandled access type %v", rpc.Access)
	}

	// From https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods
	allMethods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}

	methods := Index().String().ValuesFunc(func(g *Group) {
		methods := rpc.HTTPMethods
		// Do we have a wildcard?
		for _, m := range methods {
			if m == "*" {
				methods = allMethods
				break
			}
		}

		for _, m := range methods {
			g.Lit(m)
		}
	})

	defLoc := int(b.res.Nodes[rpc.Svc.Root][rpc.Func].Id)
	handler := Var().Id(b.rpcHandlerName(rpc)).Op("=").Op("&").Qual("encore.dev/appruntime/api", "Desc").Types(
		Op("*").Id(b.ReqTypeName()),
		b.RespType(),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Endpoint").Op(":").Lit(rpc.Name),
		Id("Methods").Op(":").Add(methods),
		Id("Raw").Op(":").Lit(rpc.Raw),
		Id("Path").Op(":").Lit(rpc.Path.String()),
		Id("RawPath").Op(":").Lit(rawPath(rpc.Path)),
		Id("PathParamNames").Op(":").Add(pathParamNames(rpc.Path)),
		Id("DefLoc").Op(":").Lit(defLoc),
		Id("Access").Op(":").Add(access),

		Id("DecodeReq").Op(":").Add(decodeReq),
		Id("CloneReq").Op(":").Add(reqDesc.Clone),
		Id("ReqPath").Op(":").Add(reqDesc.Path),
		Id("ReqUserPayload").Op(":").Add(reqDesc.UserPayload),

		Id("AppHandler").Op(":").Add(b.AppHandlerFunc()),
		Id("RawHandler").Op(":").Add(b.RawHandlerFunc()),
		Id("EncodeResp").Op(":").Add(encodeResp),
		Id("CloneResp").Op(":").Add(respDesc.Clone),
	)

	for _, part := range [...]Code{
		reqDesc.TypeDecl,
		respDesc.TypeDecl,
		handler,
	} {
		f.Add(part)
		f.Line()
	}

	if !rpc.Raw {
		caller := b.renderCaller()
		f.Add(caller)
	}
}

func newStructDesc(recv string) *structDesc {
	desc := &structDesc{}
	desc.recvName = desc.names.Get(recv)
	return desc
}

type structDesc struct {
	fields   []structField
	names    namealloc.Allocator
	recvName string
}

type fieldKind int

const (
	pathParam fieldKind = iota
	payload
	other
)

type structField struct {
	kind         fieldKind
	fieldName    string
	originalName string
	goType       *Statement

	builtin schema.Builtin // only set for fieldKind == pathParam
}

func (f *structField) paramName() string {
	return strings.ToLower(f.fieldName[:1]) + f.fieldName[1:]
}

func (r *structDesc) AddField(kind fieldKind, name string, goType *Statement, builtin schema.Builtin) string {
	fieldName := r.names.Get(name)
	r.fields = append(r.fields, structField{
		kind:         kind,
		fieldName:    fieldName,
		originalName: name,
		goType:       goType,
		builtin:      builtin,
	})
	return fieldName
}

// renderDecodeReq renders the DecodeReq code as a func literal.
func (b *rpcBuilder) renderDecodeReq() *Statement {
	return Func().Params(
		Id("req").Op("*").Qual("net/http", "Request"),
		Id("ps").Qual("encore.dev/appruntime/api", "UnnamedParams"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
	).Params(
		Id("reqData").Op("*").Id(b.ReqTypeName()),
		Id("pathParams").Qual("encore.dev/appruntime/api", "UnnamedParams"),
		Err().Error(),
	).BlockFunc(func(g *Group) {
		g.Id("reqData").Op("=").Op("&").Id(b.ReqTypeName()).Values()

		// Decode the path
		segs := make([]paths.Segment, 0, len(b.rpc.Path.Segments))
		seenWildcard := false
		wildcardIdx := 0
		for _, s := range b.rpc.Path.Segments {
			if s.Type != paths.Literal {
				segs = append(segs, s)
			}
			if !seenWildcard {
				if s.Type == paths.Wildcard {
					seenWildcard = true
				} else if s.Type == paths.Param {
					wildcardIdx++
				}
			}
		}

		if len(segs) == 0 && b.rpc.Request == nil {
			// Nothing to do; return an empty struct
			g.Return(Id("reqData"), Nil(), Nil())
			return
		}

		if seenWildcard {
			g.Comment("Trim the leading slash from wildcard parameter, as Encore's semantics excludes it,")
			g.Comment("while the httprouter implementation includes it.")
			g.Id("ps").Index(Lit(wildcardIdx)).Op("=").Qual("strings", "TrimPrefix").Call(Id("ps").Index(Lit(wildcardIdx)), Lit("/"))
			g.Line()
		}

		dec := b.marshaller.NewPossibleInstance("dec")
		g.Add(dec.WithFunc(func(g *Group) {
			// Decode path params
			for i, seg := range segs {
				pathSegmentValue := Id("ps").Index(Lit(i))

				// If the segment type is a string, then we want to unescape it
				switch seg.ValueType {
				case schema.Builtin_STRING, schema.Builtin_UUID:
					g.If(
						List(Id("value"), Err()).Op(":=").Qual("net/url", "PathUnescape").Call(pathSegmentValue),
						Err().Op("==").Nil().
							Block(
								Id("ps").Index(Lit(i)).Op("=").Id("value"),
							))
				}

				decodeCall, err := dec.FromStringToBuiltin(seg.ValueType, seg.Value, pathSegmentValue, true)
				if err != nil {
					b.errors.Addf(b.rpc.Func.Pos(), "could not create decoder for path param, %v", err)
				}

				g.Do(func(s *Statement) {
					// If it's a raw endpoint the params are not used, but validate them regardless.
					if b.rpc.Raw {
						s.Id("_").Op("=")
					} else {
						fieldName := strings.ToUpper(seg.Value[:1]) + seg.Value[1:]
						field := b.reqType.AddField(pathParam, fieldName, b.schemaBuiltInToGoType(seg.ValueType), seg.ValueType)
						s.Id("reqData").Dot(field).Op("=")
					}
				}).Add(decodeCall)
			}

			if b.rpc.Request != nil {
				// Parsing requests for HTTP methods without a body (GET, HEAD, DELETE) are handled by parsing the query string,
				// while other methods are parsed by reading the body and unmarshalling it as JSON.
				// If the same endpoint supports both, handle it with a switch.
				reqs, err := encoding.DescribeRequest(b.res.Meta, b.rpc.Request.Type, nil, b.rpc.HTTPMethods...)
				if err != nil {
					b.errors.Addf(b.rpc.Func.Pos(), "failed to describe request: %v", err.Error())
				}
				g.Line()
				if b.rpc.Request.IsPointer() {
					field := b.reqType.AddField(payload, "Params", b.typeName(b.rpc.Request, false), schema.Builtin_ANY)
					g.Id("params").Op(":=").Op("&").Add(b.typeName(b.rpc.Request, true)).Values()
					g.Id("reqData").Dot(field).Op("=").Id("params")
				} else {
					field := b.reqType.AddField(payload, "Params", b.typeName(b.rpc.Request, false), schema.Builtin_ANY)
					g.Id("params").Op(":=").Op("&").Id("reqData").Dot(field)
				}

				g.Add(Switch(Id("m").Op(":=").Id("req").Dot("Method"), Id("m")).BlockFunc(
					func(g *Group) {
						for _, r := range reqs {
							g.CaseFunc(func(g *Group) {
								for _, m := range r.HTTPMethods {
									g.Lit(m)
								}
							}).BlockFunc(func(g *Group) {
								b.decodeRequestParameters(g, b.rpc, dec, r)
							})
						}
						g.Default().Add(Id("panic").Call(Lit("HTTP method is not supported")))
					},
				))
			}

		}, func(g *Group) {
			g.Return(Nil(), Nil(), dec.LastError())
		})...)

		g.Return(Id("reqData"), Id("ps"), Nil())
	})
}

func (b *rpcBuilder) AppHandlerFunc() *Statement {
	rpc := b.rpc
	if rpc.Raw {
		return Nil()
	}

	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("req").Op("*").Id(b.ReqTypeName()),
	).Params(b.RespType(), Error()).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// If we have a service struct, initialize it first.
		group := rpc.SvcStruct
		if group != nil {
			ss := rpc.Svc.Struct
			g.List(Id("svc"), Id("initErr")).Op(":=").Id(b.serviceStructName(ss)).Dot("Get").Call()
			g.If(Id("initErr").Op("!=").Nil()).Block(
				Return(b.RespZeroValue(), Id("initErr")),
			)
			fnExpr = Id("svc").Dot(b.rpc.Name)
		} else {
			fnExpr = Id(b.rpc.Name)
		}

		g.Do(func(s *Statement) {
			if rpc.Response != nil {
				s.List(Id("resp"), Err())
			} else {
				s.Err()
			}
		}).Op(":=").Add(fnExpr).CallFunc(func(g *Group) {
			g.Id("ctx")
			for _, f := range b.reqType.fields {
				g.Id("req").Dot(f.fieldName)
			}
		})
		g.If(Err().Op("!=").Nil()).Block(Return(b.RespZeroValue(), Err()))

		if rpc.Response != nil {
			g.Return(Id("resp"), Nil())
		} else {
			g.Return(b.RespZeroValue(), Nil())
		}
	})
}

func (b *rpcBuilder) RawHandlerFunc() *Statement {
	rpc := b.rpc
	if !rpc.Raw {
		return Nil()
	}

	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("req").Op("*").Qual("net/http", "Request"),
	).BlockFunc(func(g *Group) {
		// fnExpr is the expression for the function we want to call,
		// either just MyRPCName or svc.MyRPCName if we have a service struct.
		var fnExpr *Statement

		// If we have a service struct, initialize it first.
		group := rpc.SvcStruct
		if group != nil {
			ss := rpc.Svc.Struct
			g.List(Id("svc"), Id("initErr")).Op(":=").Id(b.serviceStructName(ss)).Dot("Get").Call()
			g.If(Id("initErr").Op("!=").Nil()).Block(
				Qual("encore.dev/beta/errs", "HTTPErrorWithCode").Call(Id("w"), Id("initErr"), Lit(0)),
				Return(),
			)
			fnExpr = Id("svc").Dot(b.rpc.Name)
		} else {
			fnExpr = Id(b.rpc.Name)
		}

		g.Add(fnExpr).Call(Id("w"), Id("req"))
	})
}

func (b *rpcBuilder) renderEncodeResp() *Statement {
	if b.rpc.Raw {
		return Nil()
	}

	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
		Id("resp").Add(b.RespType()),
	).Params(Err().Error()).BlockFunc(func(g *Group) {
		if b.rpc.Response == nil {
			g.Return(Nil())
			return
		}

		resp, err := encoding.DescribeResponse(b.res.Meta, b.rpc.Response.Type, nil)
		if err != nil {
			b.errors.Addf(b.rpc.Func.Pos(), "failed to describe response: %v", err.Error())
		}

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
				g.List(Id("respData"), Err()).Op("=").Qual("encore.dev/appruntime/serde", "SerializeJSONFunc").Call(Id("json"), Func().Params(Id("ser").Op("*").Qual("encore.dev/appruntime/serde", "JSONSerializer")).BlockFunc(
					func(g *Group) {
						for _, f := range resp.BodyParameters {
							g.Add(Id("ser").Dot("WriteField").Call(Lit(f.Name), Id("resp").Dot(f.SrcName), Lit(f.OmitEmpty)))
						}
					}))
				g.If(Err().Op("!=").Nil()).Block(
					Return(Err()),
				)
				g.Id("respData").Op("=").Append(Id("respData"), LitRune('\n'))
			}

			if len(resp.HeaderParameters) > 0 {
				headerEncoder := b.marshaller.NewPossibleInstance("headerEncoder")
				g.Line().Comment("Encode headers")
				headerEncoder.Add(Id("headers").Op("=").Map(String()).Index().String().ValuesFunc(
					func(g *Group) {
						for _, f := range resp.HeaderParameters {
							headerSlice, err := headerEncoder.ToStringSlice(f.Type, Id("resp").Dot(f.SrcName))
							if err != nil {
								b.errors.Addf(b.rpc.Func.Pos(), "failed to generate header serializers: %v", err.Error())
							}
							g.Add(Lit(f.Name).Op(":").Add(headerSlice))
						}
					}))
				g.Add(headerEncoder.Finalize(
					Return(headerEncoder.LastError()),
				)...)
			}
		})

		// If response is a ptr we need to check it's not nil
		if b.RespIsPtr() {
			g.If(Id("resp").Op("!=").Nil()).Block(responseEncoder)
		} else {
			g.Add(responseEncoder)
		}

		g.Line().Comment("Write response")
		if len(resp.HeaderParameters) > 0 {
			g.For(List(Id("k"), Id("vs")).Op(":=").Range().Id("headers")).Block(
				For(List(Id("_"), Id("v")).Op(":=").Range().Id("vs")).Block(
					Id("w").Dot("Header").Call().Dot("Add").Call(Id("k"), Id("v")),
				),
			)
		}
		g.Id("w").Dot("Write").Call(Id("respData"))
		g.Return(Nil())
	})
}

func (b *rpcBuilder) ReqTypeName() string {
	return fmt.Sprintf("EncoreInternal_%sReq", b.rpc.Name)
}

func (b *rpcBuilder) RespTypeName() string {
	return fmt.Sprintf("EncoreInternal_%sResp", b.rpc.Name)
}

func (b *rpcBuilder) RespType() *Statement {
	s := Id(b.RespTypeName())
	if b.RespIsPtr() {
		s = Op("*").Add(s)
	}
	return s
}

func (b *rpcBuilder) RespZeroValue() *Statement {
	if b.RespIsPtr() {
		return Nil()
	} else {
		return b.RespType().Values()
	}
}

func (b *rpcBuilder) RespIsPtr() bool {
	if b.rpc.Response != nil && b.rpc.Response.IsPointer() {
		return true
	}
	return false
}

type requestCodegen struct {
	structCodegen
	Path        *Statement
	UserPayload *Statement
}

func (b *rpcBuilder) renderRequestStructDesc(typName string, desc *structDesc) requestCodegen {
	var result requestCodegen
	result.structCodegen = b.renderStructDesc(typName, desc, false, b.rpc.Request)
	recv := desc.recvName

	result.Path = Func().Params(Id(recv).Op("*").Id(typName)).Params(
		String(),
		Qual("encore.dev/appruntime/api", "UnnamedParams"),
		Error(),
	).BlockFunc(func(g *Group) {
		var pathParamFields []structField

		enc := b.marshaller.NewPossibleInstance("enc")
		g.Add(enc.WithFunc(func(g *Group) {
			for _, f := range desc.fields {
				if f.kind == pathParam {
					pathParamFields = append(pathParamFields, f)
				}
			}
			if len(pathParamFields) == 0 {
				g.Return(Lit(b.rpc.Path.String()), Nil(), Nil())
				return
			}

			g.Id("params").Op(":=").Qual("encore.dev/appruntime/api", "UnnamedParams").ValuesFunc(func(g *Group) {
				for _, f := range pathParamFields {
					typ := &schema.Type{Typ: &schema.Type_Builtin{Builtin: f.builtin}}
					code, err := enc.ToString(typ, Id(recv).Dot(f.fieldName))
					if err != nil {
						b.errorf("api endpoint %s.%s: unable to convert path parameter %s to string: %v",
							b.svc.Name, b.rpc.Name, f.originalName, err)
						break
					}

					g.Add(code)
				}
			})
		}, func(g *Group) {
			g.Return(Lit(""), Nil(), enc.LastError())
		})...)

		// If we don't have any params we've already yielded a return statement above. We're done.
		if len(pathParamFields) == 0 {
			return
		}

		// Construct the path as an expression in the form
		//		"/foo" + params[N].Value + "/bar"
		pathExpr := CustomFunc(Options{
			Separator: " + ",
		}, func(g *Group) {
			idx := 0
			for _, seg := range b.rpc.Path.Segments {
				if seg.Type == paths.Literal {
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

	result.UserPayload = Func().Params(Id(recv).Op("*").Id(typName)).Params(Any()).BlockFunc(func(g *Group) {
		for _, f := range desc.fields {
			if f.kind == payload {
				g.Return(Id(recv).Dot(f.fieldName))
				return
			}
		}
		g.Return(Nil())
	})

	return result
}

type structCodegen struct {
	TypeDecl *Statement
	Clone    *Statement
}

func (b *Builder) renderStructDesc(typName string, desc *structDesc, allowAlias bool, payloadType *est.Param) structCodegen {
	if len(desc.fields) == 0 && allowAlias {
		return structCodegen{
			TypeDecl: Type().Id(typName).Op("=").Qual("encore.dev/appruntime/api", "Void"),
			Clone:    Qual("encore.dev/appruntime/api", "CloneVoid"),
		}
	}

	var result structCodegen
	recv := desc.recvName

	if len(desc.fields) == 1 && desc.fields[0].kind == payload && allowAlias {
		result.TypeDecl = Type().Id(typName).Op("=").Add(b.typeName(payloadType, true))
	} else {
		result.TypeDecl = Type().Id(typName).StructFunc(func(g *Group) {
			for _, f := range desc.fields {
				g.Id(f.fieldName).Add(f.goType.Clone())
			}
		})
	}

	result.Clone = Func().Params(Id(recv).Op("*").Id(typName)).Params(Op("*").Id(typName), Error()).BlockFunc(func(g *Group) {
		// We could optimize the clone operation if there are no reference types (pointers, maps, slices)
		// in the struct. For now, simply serialize it as JSON and back.
		g.Var().Id("clone").Id(typName)
		g.List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id(recv))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)
		g.Return(Op("&").Id("clone"), Err())
	})

	return result
}

func (b *rpcBuilder) renderResponseStructDesc() structCodegen {
	rpc := b.rpc
	if rpc.Response == nil {
		return structCodegen{
			TypeDecl: Type().Id(b.RespTypeName()).Op("=").Qual("encore.dev/appruntime/api", "Void"),
			Clone:    Qual("encore.dev/appruntime/api", "CloneVoid"),
		}
	}

	var result structCodegen

	result.TypeDecl = Type().Id(b.RespTypeName()).Op("=").Add(b.typeName(rpc.Response, true))

	result.Clone = Func().Params(
		Id("resp").Add(b.RespType()),
	).Params(
		b.RespType(),
		Error(),
	).BlockFunc(func(g *Group) {
		if b.RespIsPtr() {
			// If the response is nil, we should return nil as well.
			g.If(Id("resp").Op("==").Nil()).Block(
				Return(Nil(), Nil()),
			)
		}

		// We could optimize the clone operation if there are no reference types (pointers, maps, slices)
		// in the struct. For now, simply serialize it as JSON and back.
		g.Var().Id("clone").Id(b.RespTypeName())

		g.List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id("resp"))
		g.If(Err().Op("==").Nil()).Block(
			Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
		)

		retExpr := Id("clone")
		if b.RespIsPtr() {
			retExpr = Op("&").Add(retExpr)
		}
		g.Return(retExpr, Err())
	})

	return result
}

func (b *Builder) decodeHeaders(g *Group, pos gotoken.Pos, requestDecoder *gocodegen.MarshallingCodeWrapper, params []*encoding.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode headers")
	g.Id("h").Op(":=").Id("req").Dot("Header")
	for _, f := range params {
		decoder, err := requestDecoder.FromString(f.Type, f.Name, Id("h").Dot("Get").Call(Lit(f.Name)), Id("h").Dot("Values").Call(Lit(f.Name)), false)
		if err != nil {
			b.errors.Addf(pos, "could not create decoder for header: %v", err.Error())
		}
		g.Id("params").Dot(f.SrcName).Op("=").Add(decoder)
	}
	g.Line()
}

func (b *Builder) decodeQueryString(g *Group, pos gotoken.Pos, requestDecoder *gocodegen.MarshallingCodeWrapper, params []*encoding.ParameterEncoding) {
	if len(params) == 0 {
		return
	}
	g.Comment("Decode query string")
	g.Id("qs").Op(":=").Id("req").Dot("URL").Dot("Query").Call()

	for _, f := range params {
		decoder, err := requestDecoder.FromString(f.Type, f.Name, Id("qs").Dot("Get").Call(Lit(f.Name)), Id("qs").Index(Lit(f.Name)), false)
		if err != nil {
			b.errors.Addf(pos, "could not create decoder for query: %v", err.Error())
		}
		g.Id("params").Dot(f.SrcName).Op("=").Add(decoder)
	}
	g.Line()
}

func (b *rpcBuilder) decodeRequestParameters(g *Group, rpc *est.RPC, requestDecoder *gocodegen.MarshallingCodeWrapper, req *encoding.RequestEncoding) {
	b.decodeHeaders(g, rpc.Func.Pos(), requestDecoder, req.HeaderParameters)
	b.decodeQueryString(g, rpc.Func.Pos(), requestDecoder, req.QueryParameters)

	// Decode Body
	if len(req.BodyParameters) > 0 {
		g.Comment("Decode JSON body")
		g.Id("payload").Op(":=").Add(requestDecoder.Body(Id("req").Dot("Body")))
		g.Id("iter").Op(":=").Qual(JsonPkg, "ParseBytes").Call(Id("json"), Id("payload"))
		g.Line()

		g.For(Id("iter").Dot("ReadObjectCB").Call(
			Func().Params(Id("_").Op("*").Qual(JsonPkg, "Iterator"), Id("key").String()).Bool().Block(
				Switch(Qual("strings", "ToLower").Call(Id("key"))).BlockFunc(func(g *Group) {
					for _, f := range req.BodyParameters {
						valDecoder, err := requestDecoder.FromJSON(f.Type, f.Name, "iter", Id("params").Dot(f.SrcName))
						if err != nil {
							b.errorf("could not create parser for json type: %T", f.Type.Typ)
						}
						g.Case(Lit(strings.ToLower(f.Name))).Block(valDecoder)
					}
					g.Default().Block(Id("_").Op("=").Id("iter").Dot("SkipAndReturnBytes").Call())
				}),
				Return(True()),
			)).Block(),
		)
		g.Line()
	}
}

func (b *rpcBuilder) renderCaller() *Statement {
	rpc := b.rpc
	return Func().Id(fmt.Sprintf("EncoreInternal_Call%s", rpc.Name)).ParamsFunc(func(g *Group) {
		g.Id("ctx").Qual("context", "Context")
		for _, f := range b.reqType.fields {
			g.Id(f.paramName()).Add(f.goType.Clone())
		}
	}).ParamsFunc(func(g *Group) {
		if rpc.Response != nil {
			g.Add(b.namedType(b.f, rpc.Response))
		}
		g.Error()
	}).BlockFunc(func(g *Group) {
		g.ListFunc(func(g *Group) {
			if rpc.Response != nil {
				g.Id("resp")
			} else {
				g.Id("_")
			}
			g.Err()
		}).Op(":=").Id(b.rpcHandlerName(rpc)).Dot("Call").CallFunc(func(g *Group) {
			g.Qual("encore.dev/appruntime/api", "NewCallContext").Call(Id("ctx"))
			g.Op("&").Id(b.ReqTypeName()).ValuesFunc(func(g *Group) {
				for _, f := range b.reqType.fields {
					g.Id(f.paramName())
				}
			})
		})
		g.If(Err().Op("!=").Nil()).BlockFunc(func(g *Group) {
			if rpc.Response != nil {
				if rpc.Response.IsPtr {
					g.Return(Nil(), Err())
				} else {
					g.Return(b.namedType(b.f, rpc.Response).Values(), Err())
				}
			} else {
				g.Return(Err())
			}
		})
		if rpc.Response != nil {
			g.Return(Id("resp"), Nil())
		} else {
			g.Return(Nil())
		}
	})
}

// rawPath creates a raw path representation, replacing path parameters
// with their indices to ensure all httprouter paths use consistent path param names,
// since otherwise httprouter reports path conflicts.
func rawPath(path *paths.Path) string {
	var b strings.Builder
	nParam := 0
	for i, s := range path.Segments {
		if i != 0 || path.Type.LeadingSlash() {
			b.WriteByte('/')
		}

		switch s.Type {
		case paths.Literal:
			b.WriteString(s.Value)
			continue

		case paths.Param:
			b.WriteByte(':')
		case paths.Wildcard:
			b.WriteByte('*')
		}
		b.WriteString(strconv.Itoa(nParam))
		nParam++
	}
	return b.String()
}

// pathParamNames yields a []string literal containing the names
// of the path parameters, in order.
func pathParamNames(path *paths.Path) Code {
	n := 0
	expr := Index().String().ValuesFunc(func(g *Group) {
		for _, s := range path.Segments {
			if s.Type != paths.Literal {
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
