package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser"
	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	"encr.dev/pkg/errlist"
	schema "encr.dev/proto/encore/parser/schema/v1"

	. "github.com/dave/jennifer/jen"
)

var importNames = map[string]string{
	"github.com/felixge/httpsnoop":        "httpsnoop",
	"github.com/json-iterator/go":         "jsoniter",
	"github.com/julienschmidt/httprouter": "httprouter",

	"encore.dev/beta/errs":      "errs",
	"encore.dev/runtime":        "runtime",
	"encore.dev/runtime/config": "config",
	"encore.dev/storage/sqldb":  "sqldb",
	"encore.dev/types/uuid":     "uuid",
}

const JsonPkg = "github.com/json-iterator/go"

type decodeKey struct {
	builtin schema.Builtin
	slice   bool
}

type Builder struct {
	res             *parser.Result
	compilerVersion string

	marshaller *gocodegen.MarshallingCodeGenerator
	errors     *errlist.List
}

func NewBuilder(res *parser.Result, compilerVersion string) *Builder {
	return &Builder{
		res:             res,
		compilerVersion: compilerVersion,
		errors:          errlist.New(res.FileSet),
		marshaller:      gocodegen.NewMarshallingCodeGenerator("marshaller", false),
	}
}

func (b *Builder) Main() (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFile("main")
	f.ImportNames(importNames)
	f.ImportAlias("encoding/json", "stdjson")

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it it will be imported under its proper name.
	f.Anon("encore.dev/runtime")

	for _, pkg := range b.res.App.Packages {
		f.ImportName(pkg.ImportPath, pkg.Name)
	}

	f.Var().Id("json").Op("=").Qual(JsonPkg, "Config").Values(Dict{
		Id("EscapeHTML"):             False(),
		Id("SortMapKeys"):            True(),
		Id("ValidateJsonRawMessage"): True(),
	}).Dot("Froze").Call()

	f.Line()

	for _, svc := range b.res.App.Services {
		for _, rpc := range svc.RPCs {
			f.Add(b.buildRPC(svc, rpc))
			f.Line()
		}
	}

	getAndClearEnv := func(name string) Code {
		return Id("getAndClearEnv").Call(Lit(name))
	}

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadConfig registers the Encore services.")
	f.Comment("//go:linkname loadConfig encore.dev/runtime/config.loadConfig")
	f.Func().Id("loadConfig").Params().Params(Op("*").Qual("encore.dev/runtime/config", "Config"), Error()).Block(
		Id("services").Op(":=").Index().Op("*").Qual("encore.dev/runtime/config", "Service").ValuesFunc(func(g *Group) {
			for _, svc := range b.res.App.Services {
				g.Values(Dict{
					Id("Name"):    Lit(svc.Name),
					Id("RelPath"): Lit(svc.Root.RelPath),
					Id("Endpoints"): Index().Op("*").Qual("encore.dev/runtime/config", "Endpoint").ValuesFunc(func(g *Group) {
						for _, rpc := range svc.RPCs {
							var access *Statement
							switch rpc.Access {
							case est.Public:
								access = Qual("encore.dev/runtime/config", "Public")
							case est.Auth:
								access = Qual("encore.dev/runtime/config", "Auth")
							case est.Private:
								access = Qual("encore.dev/runtime/config", "Private")
							default:
								b.errors.Addf(rpc.Func.Pos(), "unhandled access type %v", rpc.Access)
							}
							g.Values(Dict{
								Id("Name"):    Lit(rpc.Name),
								Id("Raw"):     Lit(rpc.Raw),
								Id("Path"):    Lit(rpc.Path.String()),
								Id("Handler"): Id("__encore_" + svc.Name + "_" + rpc.Name),
								Id("Access"):  access,
								Id("Methods"): Index().String().ValuesFunc(func(g *Group) {
									for _, m := range rpc.HTTPMethods {
										g.Lit(m)
									}
								}),
							})
						}
					}),
				})
			}
		}),
		Id("static").Op(":=").Op("&").Qual("encore.dev/runtime/config", "Static").Values(Dict{
			Id("Services"):       Id("services"),
			Id("AuthData"):       b.authDataType(),
			Id("EncoreCompiler"): Lit(b.compilerVersion),
			Id("AppCommit"): Qual("encore.dev/runtime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(b.res.Meta.AppRevision),
				Id("Uncommitted"): Lit(b.res.Meta.UncommittedChanges),
			}),
			Id("Testing"):     False(),
			Id("TestService"): Lit(""),
		}),
		Return(Op("&").Qual("encore.dev/runtime/config", "Config").Values(Dict{
			Id("Static"):  Id("static"),
			Id("Runtime"): Qual("encore.dev/runtime/config", "ParseRuntime").Call(getAndClearEnv("ENCORE_RUNTIME_CONFIG")),
			Id("Secrets"): Qual("encore.dev/runtime/config", "ParseSecrets").Call(getAndClearEnv("ENCORE_APP_SECRETS")),
		}), Nil()),
	)
	f.Line()

	f.Func().Id("main").Params().Block(
		If(Err().Op(":=").Qual("encore.dev/runtime", "ListenAndServe").Call(), Err().Op("!=").Nil()).Block(
			Qual("encore.dev/runtime", "Logger").Call().Dot("Fatal").Call().
				Dot("Err").Call(Err()).
				Dot("Msg").Call(Lit("could not listen and serve")),
		),
	)
	f.Line()

	f.Comment("getAndClearEnv gets an env variable and unsets it.")
	f.Func().Id("getAndClearEnv").Params(Id("env").String()).Params(String()).Block(
		Id("val").Op(":=").Qual("os", "Getenv").Call(Id("env")),
		Qual("os", "Unsetenv").Call(Id("env")),
		Return(Id("val")),
	)
	f.Line()

	f.Type().Id("validationDetails").Struct(
		Id("Field").String().Tag(map[string]string{"json": "field"}),
		Id("Err").String().Tag(map[string]string{"json": "err"}),
	)
	f.Func().Params(Id("validationDetails")).Id("ErrDetails").Params().Block()

	b.writeAuthFuncs(f)
	b.marshaller.WriteToFile(f)

	return f, b.errors.Err()
}

func (b *Builder) buildRPC(svc *est.Service, rpc *est.RPC) *Statement {
	return Func().Id("__encore_"+svc.Name+"_"+rpc.Name).Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("req").Op("*").Qual("net/http", "Request"),
		Id("ps").Qual("github.com/julienschmidt/httprouter", "Params"),
	).BlockFunc(func(g *Group) {
		g.Id("ctx").Op(":=").Id("req").Dot("Context").Call()
		g.Qual("encore.dev/runtime", "BeginOperation").Call()
		g.Defer().Qual("encore.dev/runtime", "FinishOperation").Call()
		g.Line()
		g.Var().Err().Error()
		requestDecoder := b.marshaller.NewPossibleInstance("dec")
		var hasPathParams bool
		var pathSegs []paths.Segment
		requestDecoder.Add(CustomFunc(Options{Separator: "\n"}, func(g *Group) {
			hasPathParams, pathSegs = b.decodeRequest(requestDecoder, g, rpc)

			if b.res.App.AuthHandler != nil {
				g.List(Id("uid"), Id("authData"), Id("proceed")).Op(":=").Id("__encore_authenticate").Call(
					Id("w"), Id("req"), Lit(rpc.Access == est.Auth), Lit(svc.Name), Lit(rpc.Name),
				)
				g.If(Op("!").Id("proceed")).Block(
					Return(),
				)
				g.Line()
			}

			traceID := int(b.res.Nodes[rpc.Svc.Root][rpc.Func].Id)
			g.Err().Op("=").Qual("encore.dev/runtime", "BeginRequest").Call(Id("ctx"), Qual("encore.dev/runtime", "RequestData").Values(DictFunc(func(d Dict) {
				d[Id("Type")] = Qual("encore.dev/runtime", "RPCCall")
				d[Id("Service")] = Lit(svc.Name)
				d[Id("Endpoint")] = Lit(rpc.Name)
				d[Id("Path")] = Id("req").Dot("URL").Dot("Path")
				d[Id("EndpointExprIdx")] = Lit(traceID)
				if !rpc.Raw && (rpc.Request != nil || len(pathSegs) > 0) {
					d[Id("Inputs")] = Id("inputs")
				} else {
					d[Id("Inputs")] = Nil()
				}
				if hasPathParams {
					d[Id("PathSegments")] = Id("ps")
				}

				if b.res.App.AuthHandler != nil {
					d[Id("UID")] = Id("uid")
					d[Id("AuthData")] = Id("authData")
				}
			})))
			g.If(Err().Op("!=").Nil()).Block(
				Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), buildErr("Internal", "internal error")),
				Return(),
			)
		}))
		g.Add(requestDecoder.Finalize(
			Err().Op(":=").Id("dec").Dot("LastError"),
			Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Err()),
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Err()),
			Return(),
		)...)
		g.Line()

		if rpc.Raw {
			g.Id("m").Op(":=").Qual("github.com/felixge/httpsnoop", "CaptureMetrics").Call(
				Qual("net/http", "HandlerFunc").Call(Qual(svc.Root.ImportPath, rpc.Name)), Id("w"), Id("req"),
			)
			g.If(Id("m").Dot("Code").Op(">=").Lit(400)).Block(
				Err().Op("=").Qual("fmt", "Errorf").Call(Lit("response status code %d"), Id("m").Dot("Code")),
			)
			g.Qual("encore.dev/runtime", "FinishHTTPRequest").Call(Nil(), Err(), Id("m").Dot("Code"))
			return
		}

		g.Comment("Call the endpoint")
		g.Defer().Func().Params().Block(
			Comment("Catch handler panic"),
			If(
				Id("e").Op(":=").Recover(),
				Id("e").Op("!=").Nil(),
			).Block(
				Err().Op(":=").Add(buildErrf("Internal", "panic handling request: %v", Id("e"))),
				Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Err()),
				Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Err()),
			),
		).Call()

		g.ListFunc(func(g *Group) {
			if rpc.Response != nil {
				g.Id("resp")
			}
			g.Id("respErr")
		}).Op(":=").Qual(svc.Root.ImportPath, rpc.Name).CallFunc(func(g *Group) {
			g.Id("req").Dot("Context").Call()
			for i := range pathSegs {
				g.Id("p" + strconv.Itoa(i))
			}
			if rpc.Request != nil {
				g.Id("params")
			}
		})
		g.If(Id("respErr").Op("!=").Nil()).Block(
			Id("respErr").Op("=").Qual("encore.dev/beta/errs", "Convert").Call(Id("respErr")),
			Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Id("respErr")),
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Id("respErr")),
			Return(),
		)
		g.Line()

		if rpc.Response != nil {
			b.encodeResponse(g, rpc)
		} else {
			g.Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Nil())
			g.Id("w").Dot("Header").Call().Dot("Set").Call(Lit("Content-Type"), Lit("application/json"))
			g.Id("w").Dot("WriteHeader").Call(Lit(200))
		}
	})
}

func (b *Builder) encodeResponse(g *Group, rpc *est.RPC) {
	g.Comment("Serialize the response")
	g.Var().Id("respData").Index().Byte()
	resp, err := encoding.DescribeResponse(b.res.Meta, rpc.Response.Type)
	if err != nil {
		b.errors.Addf(rpc.Func.Pos(), "failed to describe response: %v", err.Error())
	}
	var bodyStmts []Code
	headerValues := Dict{}
	headerEncoder := b.marshaller.NewPossibleInstance("headerEncoder")
	for _, f := range resp.Fields {
		switch f.Location {
		case encoding.Header:
			headerSlice, err := headerEncoder.ToStringSlice(f.Field.Typ, Id("resp").Dot(f.Field.Name))
			if err != nil {
				b.errors.Addf(rpc.Func.Pos(), "failed to generate haader serializers: %v", err.Error())
			}
			headerValues[Lit(f.Name)] = headerSlice
		case encoding.Body:
			bodyStmts = append(bodyStmts, Id("ser").Dot("WriteField").Call(Lit(f.Name), Id("resp").Dot(f.Field.Name), Lit(f.OmitEmpty)))
		default:
			b.errors.Addf(rpc.Func.Pos(), "unsupported response location: %d", f.Location)
		}
	}
	if len(bodyStmts) > 0 {
		g.Line().Comment("Encode JSON body")
		g.List(Id("respData"), Err()).Op("=").Qual("encore.dev/runtime/serde", "SerializeJSONFunc").Call(Id("json"), Func().Params(Id("ser").Op("*").Qual("encore.dev/runtime/serde", "JSONSerializer")).Block(
			bodyStmts...))
		g.If(Err().Op("!=").Nil()).Block(
			Id("marshalErr").Op(":=").Add(wrapErrCode(Err(), "Internal", "failed to marshal response")),
			Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Id("marshalErr")),
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Id("marshalErr")),
			Return(),
		)
	}

	if len(headerValues) > 0 {
		g.Line().Comment("Encode headers")
		headerEncoder.Add(Id("headers").Op(":=").Map(String()).Index().String().Values(headerValues))
		g.Add(headerEncoder.Finalize(
			Id("headerErr").Op(":=").Add(wrapErrCode(Id("headerEncoder").Dot("LastError"), "Internal", "failed to marshal headers")),
			Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Id("headerErr")),
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Id("headerErr")),
			Return(),
		)...)
	}

	g.Line().Comment("Record tracing data")
	g.Id("respData").Op("=").Append(Id("respData"), LitRune('\n'))
	g.Id("output").Op(":=").Index().Index().Byte().Values(Id("respData"))
	g.Qual("encore.dev/runtime", "FinishRequest").Call(Id("output"), Nil())

	g.Line().Comment("Write response")
	if len(headerValues) > 0 {
		g.For(List(Id("k"), Id("vs")).Op(":=").Range().Id("headers")).Block(
			For(List(Id("_"), Id("v")).Op(":=").Range().Id("vs")).Block(
				Id("w").Dot("Header").Call().Dot("Add").Call(Id("k"), Id("v")),
			),
		)
	}
	g.Id("w").Dot("Header").Call().Dot("Set").Call(Lit("Content-Type"), Lit("application/json"))
	g.Id("w").Dot("WriteHeader").Call(Lit(200))
	g.Id("w").Dot("Write").Call(Id("respData"))
}

func (b *Builder) decodeRequest(requestDecoder *gocodegen.MarshallingCodeWrapper, g *Group, rpc *est.RPC) (hasPathParams bool, pathSegs []paths.Segment) {
	segs := make([]paths.Segment, 0, len(rpc.Path.Segments))
	seenWildcard := false
	wildcardIdx := 0
	for _, s := range rpc.Path.Segments {
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

	if len(segs) == 0 && rpc.Request == nil {
		return false, segs
	}

	if seenWildcard {
		g.Line()
		g.Comment("Trim the leading slash from wildcard parameter, as Encore's semantics excludes it,")
		g.Comment("while the httprouter implementation includes it.")
		g.Id("ps").Index(Lit(wildcardIdx)).Dot("Value").Op("=").Qual("strings", "TrimPrefix").Call(Id("ps").Index(Lit(wildcardIdx)).Dot("Value"), Lit("/"))
		g.Line()
	}

	g.Comment("Decode request")
	// Decode path params
	for i, seg := range segs {
		decodeCall, err := requestDecoder.FromStringToBuiltin(seg.ValueType, seg.Value, Id("ps").Index(Lit(i)).Dot("Value"), true)
		if err != nil {
			b.errors.Addf(rpc.Func.Pos(), "could not create decoder for path param, %v", err)
		}
		g.Do(func(s *Statement) {
			// If it's a raw endpoint the params are not used, but validate them regardless.
			if rpc.Raw {
				s.Id("_").Op("=")
			} else {
				s.Id("p" + strconv.Itoa(i)).Op(":=")
			}
		}).Add(decodeCall)
	}

	if !rpc.Raw {
		if len(segs) > 0 {
			g.List(Id("inputs"), Id("_")).Op(":=").Qual("encore.dev/runtime", "SerializeInputs").CallFunc(func(g *Group) {
				for i := range segs {
					g.Id("p" + strconv.Itoa(i))
				}
			})
		} else {
			g.Var().Id("inputs").Index().Index().Byte()
		}
	}

	if rpc.Request != nil {
		// Parsing requests for HTTP methods without a body (GET, HEAD, DELETE) are handled by parsing the query string,
		// while other methods are parsed by reading the body and unmarshalling it as JSON.
		// If the same endpoint supports both, handle it with a switch.
		reqs, err := encoding.DescribeRequest(b.res.Meta, rpc.Request.Type, rpc.HTTPMethods...)
		if err != nil {
			b.errors.Addf(rpc.Func.Pos(), "failed to describe request: %v", err.Error())
		}
		g.Line()
		if rpc.Request.IsPtr {
			g.Id("params").Op(":=").Op("&").Add(b.typeName(rpc.Request, true)).Values()
		} else {
			g.Var().Id("params").Add(b.typeName(rpc.Request, true))
		}

		g.Add(Switch(Id("m").Op(":=").Id("req").Dot("Method"), Id("m")).BlockFunc(
			func(g *Group) {
				for _, r := range reqs {
					g.CaseFunc(func(g *Group) {
						for _, m := range r.HTTPMethods {
							g.Lit(m)
						}
					}).Line().Add(b.decodeRequestParameters(rpc, requestDecoder, r.Fields)...)
				}
				g.Default().Add(Id("panic").Call(Lit("HTTP method is not supported")))
			},
		))
		g.Comment("Add trace info")
		g.List(Id("jsonParams"), Err()).Op(":=").Id("json").Dot(
			"Marshal").Call(Id("params"))
		g.If(Err().Op("!=").Nil()).Block(
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), buildErr("Internal", "internal error")),
			Return(),
		)
		g.Id("inputs").Op("=").Append(Id("inputs"), Id("jsonParams"))
	}
	g.Line()
	return true, segs
}

func (b *Builder) decodeRequestParameters(rpc *est.RPC, requestDecoder *gocodegen.MarshallingCodeWrapper, fields []*encoding.ParameterEncoding) []Code {
	var qsStmts, bodyStmts, headerStmts, bodyCases []Code
	for _, f := range fields {
		switch f.Location {
		case encoding.Body:
			valDecoder, err := requestDecoder.FromJSON(f.Field.Typ, f.Name, "iter", Id("params").Dot(f.Field.Name))
			if err != nil {
				b.errorf("could not create parser for json type: %T", f.Field.Typ.Typ)
			}
			bodyCases = append(bodyCases,
				Case(Lit(strings.ToLower(f.Name))).Line().Add(valDecoder),
			)
		case encoding.Header:
			if len(headerStmts) == 0 {
				headerStmts = append(headerStmts,
					Comment("Decode Headers").Line(),
					Id("h").Op(":=").Id("req").Dot("Header").Line())
			}
			decoder, err := requestDecoder.FromString(f.Field.Typ, f.Name, Id("h").Dot("Get").Call(Lit(f.Name)), Id("h").Dot("Values").Call(Lit(f.Name)), false)
			if err != nil {
				b.errors.Addf(rpc.Func.Pos(), "could not create decoder for header: %v", err.Error())
			}
			headerStmts = append(headerStmts, Id("params").Dot(f.Field.Name).Op("=").Add(decoder).Line())
		case encoding.Query:
			if len(qsStmts) == 0 {
				qsStmts = append(qsStmts,
					Line().Comment("Decode Query String").Line(),
					Id("qs").Op(":=").Id("req").Dot("URL").Dot("Query").Call().Line())
			}
			decoder, err := requestDecoder.FromString(f.Field.Typ, f.Name, Id("qs").Dot("Get").Call(Lit(f.Name)), Id("qs").Index(Lit(f.Name)), false)
			if err != nil {
				b.errors.Addf(rpc.Func.Pos(), "could not create decoder for query: %v", err.Error())
			}
			qsStmts = append(qsStmts, Id("params").Dot(f.Field.Name).Op("=").Add(decoder).Line())
		}
	}

	if len(bodyCases) > 0 {
		bodyCases = append(bodyCases, Default().Line().Id("_").Op("=").Id("iter").Dot("SkipAndReturnBytes").Call())
		bodyStmts = append(bodyStmts,
			Line().Comment("Decode JSON Body").Line(),
			Id("payload").Op(":=").Add(requestDecoder.Body(Id("req").Dot("Body"))).Line(),
			Id("iter").Op(":=").Qual(JsonPkg, "ParseBytes").Call(Id("json"), Id("payload")),
			Line(),
			For(Id("iter").Dot("ReadObjectCB").Call(
				Func().Params(Id("_").Op("*").Qual(JsonPkg, "Iterator"), Id("key").String()).Bool().Block(
					Switch(Qual("strings", "ToLower").Call(Id("key"))).Block(bodyCases...),
					Return(True()),
				)).Block(),
			).Line(),
		)
	}
	rtnStmts := append(headerStmts, qsStmts...)
	rtnStmts = append(rtnStmts, bodyStmts...)
	return rtnStmts
}

func (b *Builder) writeAuthFuncs(f *File) {
	if b.res.App.AuthHandler == nil {
		return
	}
	f.Comment("__encore_authenticate authenticates a request.")
	f.Comment("It reports the user id, user data, and whether or not to proceed with the request.")
	f.Comment(`If requireAuth is false, it reports ("", nil, true) on authentication failure.`)
	f.Func().Id("__encore_authenticate").Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("req").Op("*").Qual("net/http", "Request"),
		Id("requireAuth").Bool(),
		List(Id("svcName"), Id("rpcName")).String(),
	).Params(
		Id("uid").Qual("encore.dev/beta/auth", "UID"),
		Id("authData").Interface(),
		Id("proceed").Bool(),
	).Block(
		Var().Id("token").String(),
		If(
			Id("auth").Op(":=").Id("req").Dot("Header").Dot("Get").Call(Lit("Authorization")),
			Id("auth").Op("!=").Lit(""),
		).Block(
			Id("TokenLoop").Op(":"),
			For(
				List(Id("_"), Id("prefix")).Op(":=").Range().Index(Op("...")).String().Values(Lit("Bearer "), Lit("Token ")),
			).Block(
				If(Qual("strings", "HasPrefix").Call(Id("auth"), Id("prefix"))).Block(
					If(
						Id("t").Op(":=").Id("auth").Index(Id("len").Call(Id("prefix")).Op(":")),
						Id("t").Op("!=").Lit(""),
					).Block(
						Id("token").Op("=").Id("t"),
						Break().Id("TokenLoop"),
					),
				),
			),
		),
		Line(),

		If(Id("token").Op("!=").Lit("")).Block(
			Var().Err().Error(),
			List(Id("uid"), Id("authData"), Err()).Op("=").Id("__encore_validateToken").Call(Id("req").Dot("Context").Call(), Id("token")),
			If(
				Qual("encore.dev/beta/errs", "Code").Call(Err()).Op("==").Qual("encore.dev/beta/errs", "Unauthenticated").Op("&&").Op("!").Id("requireAuth"),
			).Block(
				Return(Lit(""), Nil(), True()),
			).Else().If(Err().Op("!=").Nil()).Block(
				Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Err()),
				Return(Lit(""), Nil(), False()),
			).Else().Block(
				Return(Id("uid"), Id("authData"), True()),
			),
		),
		Line(),

		If(Id("requireAuth")).Block(
			Qual("encore.dev/runtime", "Logger").Call().Dot("Info").Call().
				Dot("Str").Call(Lit("service"), Id("svcName")).
				Dot("Str").Call(Lit("endpoint"), Id("rpcName")).
				Dot("Msg").Call(Lit("rejecting request due to missing auth token")),
			Qual("encore.dev/beta/errs", "HTTPError").Call(
				Id("w"), buildErr("Unauthenticated", "missing auth token"),
			),
			Return(Lit(""), Nil(), False()),
		).Else().Block(
			Return(Lit(""), Nil(), True()),
		),
	)

	authHandler := b.res.App.AuthHandler
	traceID := int(b.res.Nodes[authHandler.Svc.Root][authHandler.Func].Id)
	f.Comment("__encore_validateToken validates an auth token.")
	f.Func().Id("__encore_validateToken").Params(
		Id("ctx").Qual("context", "Context"),
		Id("token").String(),
	).Params(
		Id("uid").Qual("encore.dev/beta/auth", "UID"),
		Id("authData").Interface(),
		Id("authErr").Error(),
	).Block(
		If(Id("token").Op("==").Lit("")).Block(
			Return(Lit(""), Nil(), Nil()),
		),
		Id("done").Op(":=").Make(Chan().Struct()),
		List(Id("call"), Err()).Op(":=").Qual("encore.dev/runtime", "BeginAuth").Call(Lit(traceID), Id("token")),
		If(Err().Op("!=").Nil()).Block(
			Return(Lit(""), Nil(), Err()),
		),
		Line(),

		Go().Func().Params().BlockFunc(func(g *Group) {
			g.Defer().Id("close").Call(Id("done"))
			g.Id("authErr").Op("=").Id("call").Dot("BeginReq").Call(Id("ctx"), Qual("encore.dev/runtime", "RequestData").Values(Dict{
				Id("Type"):            Qual("encore.dev/runtime", "AuthHandler"),
				Id("Service"):         Lit(authHandler.Svc.Name),
				Id("Endpoint"):        Lit(authHandler.Name),
				Id("EndpointExprIdx"): Lit(traceID),
				Id("Inputs"):          Index().Index().Byte().Values(Index().Byte().Parens(Qual("strconv", "Quote").Call(Id("token")))),
			}))
			g.If(Id("authErr").Op("!=").Nil()).Block(
				Return(),
			)
			g.Defer().Func().Params().Block(
				If(
					Id("err2").Op(":=").Recover(),
					Id("err2").Op("!=").Nil(),
				).Block(
					Id("authErr").Op("=").Add(buildErrf("Internal", "auth handler panicked: %v", Id("err2"))),
					Id("call").Dot("FinishReq").Call(Nil(), Id("authErr")),
				),
			).Call()

			if authHandler.AuthData != nil {
				g.List(Id("uid"), Id("authData"), Id("authErr")).Op("=").Qual(authHandler.Svc.Root.ImportPath, authHandler.Name).Call(Id("ctx"), Id("token"))
				g.List(Id("serialized"), Id("_")).Op(":=").Qual("encore.dev/runtime", "SerializeInputs").Call(Id("uid"), Id("authData"))
			} else {
				g.List(Id("uid"), Id("authErr")).Op("=").Qual(authHandler.Svc.Root.ImportPath, authHandler.Name).Call(Id("ctx"), Id("token"))
				g.List(Id("serialized"), Id("_")).Op(":=").Qual("encore.dev/runtime", "SerializeInputs").Call(Id("uid"))
			}
			g.If(Id("authErr").Op("!=").Nil()).Block(
				Id("call").Dot("FinishReq").Call(Nil(), Id("authErr")),
			).Else().Block(
				Id("call").Dot("FinishReq").Call(Id("serialized"), Nil()),
			)
		}).Call(),
		Op("<-").Id("done"),
		Id("call").Dot("Finish").Call(Id("uid"), Id("authErr")),
		Return(Id("uid"), Id("authData"), Id("authErr")),
	)
}

type decoderDescriptor struct {
	Method string
	Input  Code
	Result Code
	IsList bool
	Block  []Code
}

func (b *Builder) typeName(param *est.Param, skipPtr bool) *Statement {
	typName := b.schemaTypeToGoType(param.Type)

	if param.IsPtr && !skipPtr {
		typName = Op("*").Add(typName)
	}

	return typName
}

func (b *Builder) authDataType() Code {
	if ah := b.res.App.AuthHandler; ah != nil && ah.AuthData != nil {
		typName := b.schemaTypeToGoType(ah.AuthData.Type)
		if ah.AuthData.IsPtr {
			// reflect.TypeOf((*T)(nil))
			return Qual("reflect", "TypeOf").Call(Parens(Op("*").Add(typName)).Call(Nil()))
		} else {
			// reflect.TypeOf(T{})
			return Qual("reflect", "TypeOf").Call(typName.Values())
		}
	}
	return Nil()
}

func (b *Builder) error(err error) {
	panic(bailout{err})
}

func (b *Builder) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

func buildErr(code, msg string) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msg").Call(Lit(msg)).Dot("Err").Call()
}

func buildErrDetails(code string, msg, field, err Code) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "B").Call().
		Dot("Code").Call(Qual(p, code)).
		Dot("Msg").Call(msg).
		Dot("Details").Call(
		Id("validationDetails").Values(Dict{
			Id("Field"): field,
			Id("Err"):   err,
		})).Dot("Err").Call()
}

func buildErrf(code, format string, args ...Code) *Statement {
	p := "encore.dev/beta/errs"
	args = append([]Code{Lit(format)}, args...)
	return Qual(p, "B").Call().Dot("Code").Call(Qual(p, code)).Dot("Msgf").Call(args...).Dot("Err").Call()
}

func wrapErrCode(err Code, code, msg string) *Statement {
	p := "encore.dev/beta/errs"
	return Qual(p, "WrapCode").Call(err, Qual(p, code), Lit(msg))
}

type bailout struct {
	err error
}
