package codegen

import (
	"fmt"
	gotoken "go/token"
	"path"
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

	"encore.dev/appruntime/api":         "api",
	"encore.dev/appruntime/app":         "app",
	"encore.dev/appruntime/app/appinit": "appinit",
	"encore.dev/appruntime/config":      "config",
	"encore.dev/appruntime/serde":       "serde",
	"encore.dev/beta/errs":              "errs",
	"encore.dev/storage/sqldb":          "sqldb",
	"encore.dev/types/uuid":             "uuid",
}

func (b *Builder) registerImports(f *File) {
	f.ImportNames(importNames)
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
	bb := &rpcBuilder{Builder: b, f: f, svc: svc, rpc: rpc}
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

	reqType  structDesc
	respType structDesc
}

func (b *rpcBuilder) Write(f *File) error {
	decodeReq := b.renderDecodeReq()
	encodeResp := b.renderEncodeResp()
	reqType := b.renderStructDesc(b.ReqTypeName(), &b.reqType, true)
	respType := b.renderStructDesc(b.RespTypeName(), &b.respType, false)

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
		Op("*").Id(b.RespTypeName()),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Endpoint").Op(":").Lit(rpc.Name),
		Id("Methods").Op(":").Add(methods),
		Id("Path").Op(":").Lit(rpc.Path.String()),
		Id("DefLoc").Op(":").Lit(defLoc),
		Id("Access").Op(":").Add(access),
		Id("DecodeReq").Op(":").Add(decodeReq),
		Id("AppHandler").Op(":").Add(b.AppHandlerFunc()),
		Id("EncodeResp").Op(":").Add(encodeResp),
	)

	for _, c := range reqType {
		f.Add(c)
	}
	for _, c := range respType {
		f.Add(c)
	}
	f.Add(handler)

	if !rpc.Raw {
		caller := b.renderCaller()
		f.Add(caller)
	}

	return nil
}

type structDesc struct {
	fields []structField
	names  namealloc.Allocator
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
	// If this is a raw endpoint, we already know the fields to use.
	if b.rpc.Raw {
		b.reqType.AddField(other, "W", Qual("net/http", "ResponseWriter"), schema.Builtin_ANY)
		b.reqType.AddField(other, "Req", Op("*").Qual("net/http", "Request"), schema.Builtin_ANY)
	}

	return Func().Params(
		Id("req").Op("*").Qual("net/http", "Request"),
		Id("ps").Qual("github.com/julienschmidt/httprouter", "Params"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
	).Params(Id("reqData").Op("*").Id(b.ReqTypeName()), Err().Error()).BlockFunc(func(g *Group) {
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
			g.Return(Id("reqData"), Nil())
			return
		}

		if seenWildcard {
			g.Comment("Trim the leading slash from wildcard parameter, as Encore's semantics excludes it,")
			g.Comment("while the httprouter implementation includes it.")
			g.Id("ps").Index(Lit(wildcardIdx)).Dot("Value").Op("=").Qual("strings", "TrimPrefix").Call(Id("ps").Index(Lit(wildcardIdx)).Dot("Value"), Lit("/"))
			g.Line()
		}

		dec := b.marshaller.NewPossibleInstance("dec")
		g.Add(dec.WithFunc(func(g *Group) {
			// Decode path params
			for i, seg := range segs {
				pathSegmentValue := Id("ps").Index(Lit(i)).Dot("Value")

				// If the segment type is a string, then we want to unescape it
				switch seg.ValueType {
				case schema.Builtin_STRING, schema.Builtin_UUID:
					g.If(
						List(Id("value"), Err()).Op(":=").Qual("net/url", "PathUnescape").Call(pathSegmentValue),
						Err().Op("==").Nil().
							Block(
								Id("ps").Index(Lit(i)).Dot("Value").Op("=").Id("value"),
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
				if b.rpc.Request.IsPtr {
					field := b.reqType.AddField(payload, "Params", b.typeName(b.rpc.Request, false), schema.Builtin_ANY)
					g.Id("params").Op(":=").Op("&").Add(b.typeName(b.rpc.Request, true)).Values()
					g.Id("reqData").Dot(field).Op("=").Id("params")
				} else {
					g.Var().Id("params").Add(b.typeName(b.rpc.Request, true))
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
			g.Return(Nil(), dec.LastError())
		})...)
		g.Return(Id("reqData"), Nil())
	})
}

func (b *rpcBuilder) AppHandlerFunc() *Statement {
	rpc := b.rpc
	return Func().Params(
		Id("ctx").Qual("context", "Context"),
		Id("req").Op("*").Id(b.ReqTypeName()),
	).Params(Op("*").Id(b.RespTypeName()), Error()).BlockFunc(func(g *Group) {
		if rpc.Raw {
			g.Qual(b.rpc.Svc.Root.ImportPath, b.rpc.Name).CallFunc(func(g *Group) {
				for _, f := range b.reqType.fields {
					g.Id("req").Dot(f.fieldName)
				}
			})
			g.Return(Op("&").Id(b.RespTypeName()).Values(), Nil())
			return
		}

		// TODO handle raw endpoints, no return vals, etc
		g.Do(func(s *Statement) {
			if rpc.Response != nil {
				s.List(Id("resp"), Err())
			} else {
				s.Err()
			}
		}).Op(":=").Qual(b.rpc.Svc.Root.ImportPath, b.rpc.Name).CallFunc(func(g *Group) {
			g.Id("ctx")
			for _, f := range b.reqType.fields {
				g.Id("req").Dot(f.fieldName)
			}
		})
		g.If(Err().Op("!=").Nil()).Block(Return(Nil(), Err()))

		g.Return(
			Op("&").Id(b.RespTypeName()).ValuesFunc(func(g *Group) {
				if rpc.Response != nil {
					g.Id("resp")
				}
			}),
			Nil(),
		)
	})
}

func (b *rpcBuilder) renderEncodeResp() *Statement {
	return Func().Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("json").Qual("github.com/json-iterator/go", "API"),
		Id("out").Op("*").Id(b.RespTypeName()),
	).Params(Error()).BlockFunc(func(g *Group) {
		if b.rpc.Response == nil {
			g.Return(Nil())
			return
		}
		b.respType.AddField(payload, "Data", b.namedType(b.f, b.rpc.Response), schema.Builtin_ANY)

		g.Var().Err().Error()

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
							g.Add(Id("ser").Dot("WriteField").Call(Lit(f.Name), Id("out").Dot("Data").Dot(f.SrcName), Lit(f.OmitEmpty)))
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
							headerSlice, err := headerEncoder.ToStringSlice(f.Type, Id("out").Dot("Data").Dot(f.SrcName))
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
		if b.rpc.Response.IsPtr {
			g.If(Id("out").Dot("Data").Op("!=").Nil()).Block(responseEncoder)
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

func (b *rpcBuilder) renderStructDesc(typName string, desc *structDesc, forRequest bool) []Code {
	if len(desc.fields) == 0 && !forRequest {
		return []Code{Type().Id(typName).Op("=").Qual("encore.dev/appruntime/api", "Void")}
	}

	recv := desc.names.Get("p")

	codes := []Code{
		Type().Id(typName).StructFunc(func(g *Group) {
			for _, f := range desc.fields {
				g.Id(f.fieldName).Add(f.goType.Clone())
			}
		}),
		Func().Params(Id(recv).Op("*").Id(typName)).Id("Serialize").Params(Id("json").Qual("github.com/json-iterator/go", "API")).Params(Index().Index().Byte(), Error()).BlockFunc(func(g *Group) {
			if len(desc.fields) == 0 {
				g.Return(Nil(), Nil())
				return
			}

			g.Id("data").Op(":=").Make(Index().Index().Byte(), Lit(len(desc.fields)))

			g.For(List(Id("i"), Id("val")).Op(":=").Range().Index(Op("...")).Any().ValuesFunc(func(g *Group) {
				for _, f := range desc.fields {
					g.Id(recv).Dot(f.fieldName)
				}
			})).Block(
				List(Id("v"), Err()).Op(":=").Id("json").Dot("Marshal").Call(Id("val")),
				If(Err().Op("!=").Nil()).Block(Return(Nil(), Err())),
				Id("data").Index(Id("i")).Op("=").Id("v"),
			)

			g.Return(Id("data"), Nil())
		}),
		Func().Params(Id(recv).Op("*").Id(typName)).Id("Clone").Params().Params(Op("*").Id(typName), Error()).BlockFunc(func(g *Group) {
			// We could optimize the clone operation if there are no reference types (pointers, maps, slices)
			// in the struct. For now, simply serialize it as JSON and back.
			g.Var().Id("clone").Id(typName)
			g.List(Id("bytes"), Id("err")).Op(":=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Marshal").Call(Id(recv))
			g.If(Err().Op("==").Nil()).Block(
				Err().Op("=").Qual("github.com/json-iterator/go", "ConfigDefault").Dot("Unmarshal").Call(Id("bytes"), Op("&").Id("clone")),
			)
			g.Return(Op("&").Id("clone"), Err())
		}),
	}

	if forRequest {
		pathParamsFn := Func().Params(Id(recv).Op("*").Id(typName)).Id("Path").Params().Params(
			String(),
			Qual("encore.dev/appruntime/api", "PathParams"),
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

				g.Id("params").Op(":=").Qual("encore.dev/appruntime/api", "PathParams").ValuesFunc(func(g *Group) {
					for _, f := range pathParamFields {
						typ := &schema.Type{Typ: &schema.Type_Builtin{Builtin: f.builtin}}
						code, err := enc.ToString(typ, Id(recv).Dot(f.fieldName))
						if err != nil {
							b.errorf("api endpoint %s.%s: unable to convert path parameter %s to string: %v",
								b.svc.Name, b.rpc.Name, f.originalName, err)
							break
						}

						g.Values(Dict{
							Id("Key"):   Lit(f.originalName),
							Id("Value"): code,
						})
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
						g.Id("params").Index(Lit(idx)).Dot("Value")
						idx++
					}
				}
			})
			g.Return(pathExpr, Id("params"), Nil())
		})
		codes = append(codes, pathParamsFn)
	}

	return codes
}

func (b *rpcBuilder) decodeHeaders(g *Group, pos gotoken.Pos, requestDecoder *gocodegen.MarshallingCodeWrapper, params []*encoding.ParameterEncoding) {
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

func (b *rpcBuilder) decodeQueryString(g *Group, pos gotoken.Pos, requestDecoder *gocodegen.MarshallingCodeWrapper, params []*encoding.ParameterEncoding) {
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
			// TODO implement
			g.Qual("encore.dev/appruntime/api", "CallContext").Values()
			g.Op("&").Id(b.ReqTypeName()).ValuesFunc(func(g *Group) {
				for _, f := range b.reqType.fields {
					g.Id(f.paramName())
				}
			})
		})
		if rpc.Response != nil {
			g.Return(Id("resp").Dot("Data"), Err())
		} else {
			g.Return(Err())
		}
	})
}
