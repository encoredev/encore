package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/parser/paths"
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

type decodeKey struct {
	builtin schema.Builtin
	slice   bool
}

type Builder struct {
	res *parser.Result

	builtins     []decoderDescriptor
	seenBuiltins map[decodeKey]decoderDescriptor
}

func NewBuilder(res *parser.Result) *Builder {
	return &Builder{
		res:          res,
		seenBuiltins: make(map[decodeKey]decoderDescriptor),
	}
}

func (b *Builder) Main() (f *File, err error) {
	defer func() {
		if err := recover(); err != nil {
			if b, ok := err.(bailout); ok {
				err = b.err
			} else {
				panic(err)
			}
		}
	}()

	f = NewFile("main")
	f.ImportNames(importNames)
	f.ImportAlias("encoding/json", "stdjson")

	for _, pkg := range b.res.App.Packages {
		f.ImportName(pkg.ImportPath, pkg.Name)
	}

	f.Var().Id("json").Op("=").Qual("github.com/json-iterator/go", "Config").Values(Dict{
		Id("EscapeHTML"):             False(),
		Id("SortMapKeys"):            True(),
		Id("ValidateJsonRawMessage"): True(),
	}).Dot("Froze").Call()

	f.Line()

	for _, svc := range b.res.App.Services {
		for _, rpc := range svc.RPCs {
			f.Add(b.buildRPC(f, svc, rpc))
			f.Line()
		}
	}

	getEnv := func(name string) Code {
		return Qual("os", "Getenv").Call(Lit(name))
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
								panic(fmt.Errorf("unhandled access type %v", rpc.Access))
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
			Id("Services"):    Id("services"),
			Id("AuthData"):    b.authDataType(),
			Id("Testing"):     False(),
			Id("TestService"): Lit(""),
		}),
		Return(Op("&").Qual("encore.dev/runtime/config", "Config").Values(Dict{
			Id("Static"):  Id("static"),
			Id("Runtime"): Qual("encore.dev/runtime/config", "ParseRuntime").Call(getEnv("ENCORE_RUNTIME_CONFIG")),
			Id("Secrets"): Qual("encore.dev/runtime/config", "ParseSecrets").Call(getEnv("ENCORE_APP_SECRETS")),
		}), Nil()),
	)
	f.Line()

	f.Func().Id("main").Params().Block(
		If(Id("err").Op(":=").Qual("encore.dev/runtime", "ListenAndServe").Call(), Err().Op("!=").Nil()).Block(
			Qual("encore.dev/runtime", "Logger").Call().Dot("Fatal").Call().
				Dot("Err").Call(Id("err")).
				Dot("Msg").Call(Lit("could not listen and serve")),
		),
	)
	f.Line()

	f.Type().Id("validationDetails").Struct(
		Id("Field").String().Tag(map[string]string{"json": "field"}),
		Id("Err").String().Tag(map[string]string{"json": "err"}),
	)
	f.Func().Params(Id("validationDetails")).Id("ErrDetails").Params().Block()

	b.writeAuthFuncs(f)
	b.writeDecoder(f)

	return f, nil
}

func (b *Builder) buildRPC(f *File, svc *est.Service, rpc *est.RPC) *Statement {
	return Func().Id("__encore_"+svc.Name+"_"+rpc.Name).Params(
		Id("w").Qual("net/http", "ResponseWriter"),
		Id("req").Op("*").Qual("net/http", "Request"),
		Id("ps").Qual("github.com/julienschmidt/httprouter", "Params"),
	).BlockFunc(func(g *Group) {
		g.Id("ctx").Op(":=").Id("req").Dot("Context").Call()
		g.Qual("encore.dev/runtime", "BeginOperation").Call()
		g.Defer().Qual("encore.dev/runtime", "FinishOperation").Call()
		g.Line()

		hasParams, pathSegs := b.decodeRequest(g, svc, rpc)

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
		if rpc.Raw {
			g.Err().Op(":=").Qual("encore.dev/runtime", "BeginRequest").Call(Id("ctx"), Qual("encore.dev/runtime", "RequestData").Values(DictFunc(func(d Dict) {
				d[Id("Type")] = Qual("encore.dev/runtime", "RPCCall")
				d[Id("Service")] = Lit(svc.Name)
				d[Id("Endpoint")] = Lit(rpc.Name)
				d[Id("CallExprIdx")] = Lit(0)
				d[Id("EndpointExprIdx")] = Lit(traceID)
				d[Id("Inputs")] = Nil()
				if b.res.App.AuthHandler != nil {
					d[Id("UID")] = Id("uid")
					d[Id("AuthData")] = Id("authData")
				}
			})))
			g.If(Err().Op("!=").Nil()).Block(
				Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), buildErr("Internal", "internal error")),
				Return(),
			)
			g.Line()

			g.Id("m").Op(":=").Qual("github.com/felixge/httpsnoop", "CaptureMetrics").Call(
				Qual("net/http", "HandlerFunc").Call(Qual(svc.Root.ImportPath, rpc.Name)), Id("w"), Id("req"),
			)
			g.If(Id("m").Dot("Code").Op(">=").Lit(400)).Block(
				Err().Op("=").Qual("fmt", "Errorf").Call(Lit("response status code %d"), Id("m").Dot("Code")),
			)
			g.Qual("encore.dev/runtime", "FinishHTTPRequest").Call(Nil(), Err(), Id("m").Dot("Code"))
			return
		}

		g.Err().Op(":=").Qual("encore.dev/runtime", "BeginRequest").Call(Id("ctx"), Qual("encore.dev/runtime", "RequestData").Values(DictFunc(func(d Dict) {
			d[Id("Type")] = Qual("encore.dev/runtime", "RPCCall")
			d[Id("Service")] = Lit(svc.Name)
			d[Id("Endpoint")] = Lit(rpc.Name)
			d[Id("CallExprIdx")] = Lit(0)
			d[Id("EndpointExprIdx")] = Lit(traceID)
			if rpc.Request != nil || len(pathSegs) > 0 {
				d[Id("Inputs")] = Id("inputs")
			} else {
				d[Id("Inputs")] = Nil()
			}
			if b.res.App.AuthHandler != nil {
				d[Id("UID")] = Id("uid")
				d[Id("AuthData")] = Id("authData")
			}
		})))
		g.If(Err().Op("!=").Nil()).Block(
			Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), buildErr("Internal", "internal error")),
			Return(),
		).Do(func(s *Statement) {
			if hasParams {
				s.Else().If(Err().Op(":=").Id("dec").Dot("Err").Call(), Err().Op("!=").Nil()).Block(
					Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Err()),
					Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Err()),
					Return(),
				)
			}
		})
		g.Line()

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
			g.Comment("Serialize the response")
			g.Var().Id("respData").Index().Byte()
			g.List(Id("respData"), Id("marshalErr")).Op(":=").Id("json").Dot("MarshalIndent").Call(Id("resp"), Lit(""), Lit("  "))
			g.If(Id("marshalErr").Op("!=").Nil()).Block(
				Id("marshalErr").Op("=").Add(wrapErrCode(Id("marshalErr"), "Internal", "failed to marshal response")),
				Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Id("marshalErr")),
				Qual("encore.dev/beta/errs", "HTTPError").Call(Id("w"), Id("marshalErr")),
				Return(),
			)
			g.Id("respData").Op("=").Append(Id("respData"), LitRune('\n'))
			g.Id("output").Op(":=").Index().Index().Byte().Values(Id("respData"))
			g.Qual("encore.dev/runtime", "FinishRequest").Call(Id("output"), Nil())
			g.Id("w").Dot("Header").Call().Dot("Set").Call(Lit("Content-Type"), Lit("application/json"))
			g.Id("w").Dot("WriteHeader").Call(Lit(200))
			g.Id("w").Dot("Write").Call(Id("respData"))
		} else {
			g.Qual("encore.dev/runtime", "FinishRequest").Call(Nil(), Nil())
			g.Id("w").Dot("Header").Call().Dot("Set").Call(Lit("Content-Type"), Lit("application/json"))
			g.Id("w").Dot("WriteHeader").Call(Lit(200))
		}
	})
}

func (b *Builder) decodeRequest(g *Group, svc *est.Service, rpc *est.RPC) (hasParams bool, pathSegs []paths.Segment) {
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
	g.Var().Id("dec").Id("typeDecoder")

	// Decode path params
	for i, seg := range segs {
		name := b.builtinDecoder(seg.ValueType, false, fmt.Sprintf("rpc %s.%s: path parameter #%d", svc.Name, rpc.Name, i+1))
		g.Do(func(s *Statement) {
			// If it's a raw endpoint the params are not used, but validate them regardless.
			if rpc.Raw {
				s.Id("_").Op("=")
			} else {
				s.Id("p" + strconv.Itoa(i)).Op(":=")
			}
		}).Id("dec").Dot(name).Call(Lit(seg.Value), Id("ps").Index(Lit(i)).Dot("Value"), True())
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
		var qsMethods []string
		bodyMethods := false
	MethodLoop:
		for _, m := range rpc.HTTPMethods {
			switch m {
			case "GET", "HEAD", "DELETE":
				qsMethods = append(qsMethods, m)
			case "*":
				bodyMethods = true
				qsMethods = []string{"GET", "HEAD", "DELETE"}
				break MethodLoop
			default:
				bodyMethods = true
			}
		}

		g.Line()
		if rpc.Request.IsPtr {
			g.Id("params").Op(":=").Op("&").Add(b.typeName(rpc.Request, true)).Values()
		} else {
			g.Var().Id("params").Add(b.typeName(rpc.Request, true))
		}

		var qsStmts, bodyStmts []Code
		if len(qsMethods) > 0 {
			qsStmts = append(qsStmts,
				Id("qs").Op(":=").Id("req").Dot("URL").Dot("Query").Call(),
			)

			if named := rpc.Request.Type.GetNamed(); named != nil {
				decl := b.res.App.Decls[named.Id]
				if st := decl.Type.GetStruct(); st != nil {
					for _, f := range st.Fields {
						qsName := f.QueryStringName
						if qsName == "-" {
							continue
						}

						name := b.queryStringDecoder(f.Typ, fmt.Sprintf("api %s.%s: field %s", svc.Name, rpc.Name, f.Name))
						qsStmts = append(qsStmts, Id("params").Dot(f.Name).Op("=").Id("dec").Dot(name).Call(
							Lit(qsName),
							Id("qs").Do(func(s *Statement) {
								if f.Typ.GetList() != nil {
									s.Index(Lit(qsName))
								} else {
									s.Dot("Get").Call(Lit(qsName))
								}
							}),
							False(),
						))
					}
				}
			}
			qsStmts = append(qsStmts,
				Id("inputs").Op("=").Append(Id("inputs"), Index().Byte().Call(Lit("?").Op("+").Id("req").Dot("URL").Dot("RawQuery"))),
			)
		}

		if bodyMethods {
			bodyStmts = append(bodyStmts,
				Id("payload").Op(":=").Id("dec").Dot("Body").Call(Id("req").Dot("Body"), Op("&").Id("params")),
				Id("inputs").Op("=").Append(Id("inputs"), Id("payload")),
			)
		}

		// Use switch if we have both body and query string cases
		if len(qsMethods) > 0 && bodyMethods {
			g.Switch(Id("req").Dot("Method")).Block(
				CaseFunc(func(g *Group) {
					for _, m := range qsMethods {
						g.Lit(m)
					}
				}).Block(qsStmts...),
				Default().Block(bodyStmts...),
			)
		} else if len(qsMethods) > 0 {
			for _, s := range qsStmts {
				g.Add(s)
			}
		} else if bodyMethods {
			for _, s := range bodyStmts {
				g.Add(s)
			}
		}
	}

	g.Line()
	return true, segs
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
				Id("CallExprIdx"):     Lit(0),
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
	Block  []Code
}

func (b *Builder) queryStringDecoder(t *schema.Type, src string) string {
	switch t := t.Typ.(type) {
	case *schema.Type_List:
		if bt, ok := t.List.Elem.Typ.(*schema.Type_Builtin); ok {
			return b.builtinDecoder(bt.Builtin, true, src)
		}
		panic(fmt.Sprintf("unsupported query string type: list of %T", t.List.Elem))
	case *schema.Type_Builtin:
		return b.builtinDecoder(t.Builtin, false, src)
	default:
		panic(fmt.Sprintf("unsupported query string type: %T", t))
	}
}

func (b *Builder) builtinDecoder(t schema.Builtin, slice bool, src string) string {
	key := decodeKey{builtin: t, slice: slice}
	if n, ok := b.seenBuiltins[key]; ok {
		return n.Method
	} else if slice {
		k2 := decodeKey{builtin: t}
		b.builtinDecoder(t, false, src)
		desc := b.seenBuiltins[k2]
		name := desc.Method + "List"
		fn := decoderDescriptor{name, Index().String(), Index().Add(desc.Result), []Code{
			For(List(Id("_"), Id("x")).Op(":=").Range().Id("s")).Block(
				Id("v").Op("=").Append(Id("v"), Id("d").Dot(desc.Method).Call(Id("x"))),
			),
			Return(Id("v")),
		}}
		b.seenBuiltins[key] = fn
		b.builtins = append(b.builtins, fn)
	}

	var fn decoderDescriptor
	switch t {
	case schema.Builtin_STRING:
		fn = decoderDescriptor{"String", String(), String(), []Code{Return(Id("s"))}}
	case schema.Builtin_BYTES:
		fn = decoderDescriptor{"Bytes", String(), Index().Byte(), []Code{
			List(Id("v"), Err()).Op(":=").Qual("encoding/base64", "URLEncoding").Dot("DecodeString").Call(Id("s")),
			Id("d").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_BOOL:
		fn = decoderDescriptor{"Bool", String(), Bool(), []Code{
			List(Id("v"), Err()).Op(":=").Qual("strconv", "ParseBool").Call(Id("s")),
			Id("d").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_UUID:
		fn = decoderDescriptor{"UUID", String(), Qual("encore.dev/types/uuid", "UUID"), []Code{
			List(Id("v"), Err()).Op(":=").Qual("encore.dev/types/uuid", "FromString").Call(Id("s")),
			Id("d").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_TIME:
		fn = decoderDescriptor{"Time", String(), Qual("time", "Time"), []Code{
			List(Id("v"), Err()).Op(":=").Qual("time", "Parse").Call(Qual("time", "RFC3339"), Id("s")),
			Id("d").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			Return(Id("v")),
		}}
	case schema.Builtin_USER_ID:
		fn = decoderDescriptor{"UserID", String(), Qual("encore.dev/beta/auth", "UID"), []Code{
			Return(Qual("encore.dev/beta/auth", "UID").Call(Id("s"))),
		}}
	case schema.Builtin_JSON:
		fn = decoderDescriptor{"JSON", String(), Qual("encoding/json", "RawMessage"), []Code{
			Return(Qual("encoding/json", "RawMessage").Call(Id("s"))),
		}}
	default:
		type kind int
		const (
			unsigned kind = iota + 1
			signed
			float
		)
		numTypes := map[schema.Builtin]struct {
			typ  string
			kind kind
			bits int
		}{
			schema.Builtin_INT8:    {"int8", signed, 8},
			schema.Builtin_INT16:   {"int16", signed, 16},
			schema.Builtin_INT32:   {"int32", signed, 32},
			schema.Builtin_INT64:   {"int64", signed, 64},
			schema.Builtin_INT:     {"int", signed, 64},
			schema.Builtin_UINT8:   {"uint8", unsigned, 8},
			schema.Builtin_UINT16:  {"uint16", unsigned, 16},
			schema.Builtin_UINT32:  {"uint32", unsigned, 32},
			schema.Builtin_UINT64:  {"uint64", unsigned, 64},
			schema.Builtin_UINT:    {"uint", unsigned, 64},
			schema.Builtin_FLOAT64: {"float64", float, 64},
			schema.Builtin_FLOAT32: {"float32", float, 32},
		}

		def, ok := numTypes[t]
		if !ok {
			b.errorf("generating code for %s: unsupported type: %s", src, t)
		}

		cast := def.typ != "int64" && def.typ != "uint64" && def.typ != "float64"
		fn = decoderDescriptor{strings.Title(def.typ), String(), Id(def.typ), []Code{
			List(Id("x"), Err()).Op(":=").Do(func(s *Statement) {
				switch def.kind {
				case unsigned:
					s.Qual("strconv", "ParseUint").Call(Id("s"), Lit(10), Lit(def.bits))
				case signed:
					s.Qual("strconv", "ParseInt").Call(Id("s"), Lit(10), Lit(def.bits))
				case float:
					s.Qual("strconv", "ParseFloat").Call(Id("s"), Lit(def.bits))
				default:
					b.errorf("generating code for %s: unknown kind %v", src, def.kind)
				}
			}),
			Id("d").Dot("setErr").Call(Lit("invalid parameter"), Id("field"), Err()),
			ReturnFunc(func(g *Group) {
				if cast {
					g.Id(def.typ).Call(Id("x"))
				} else {
					g.Id("x")
				}
			}),
		}}
	}

	b.seenBuiltins[key] = fn
	b.builtins = append(b.builtins, fn)
	return fn.Method
}

func (b *Builder) writeDecoder(f *File) {
	f.Comment("typeDecoder decodes types from incoming requests")
	f.Type().Id("typeDecoder").Struct(Err().Error())
	for _, desc := range b.builtins {
		f.Func().Params(
			Id("d").Op("*").Id("typeDecoder"),
		).Id(desc.Method).Params(Id("field"), Id("s").Add(desc.Input), Id("required").Bool()).Params(Id("v").Add(desc.Result)).BlockFunc(func(g *Group) {
			g.If(Op("!").Id("required").Op("&&").Id("s").Op("==").Lit("")).Block(Return())
			for _, s := range desc.Block {
				g.Add(s)
			}
		})
		f.Line()
	}

	f.Func().Params(Id("d").Op("*").Id("typeDecoder")).Id("Body").Params(Id("body").Qual("io", "Reader"), Id("dst").Interface()).Params(Id("payload").Index().Byte()).Block(
		List(Id("payload"), Err()).Op(":=").Qual("io/ioutil", "ReadAll").Call(Id("body")),
		If(Err().Op("==").Nil().Op("&&").Len(Id("payload")).Op("==").Lit(0)).Block(
			Id("d").Dot("setErr").Call(Lit("missing request body"), Lit("request_body"), Qual("fmt", "Errorf").Call(Lit("missing request body"))),
		).Else().If(Err().Op("!=").Nil()).Block(
			Id("d").Dot("setErr").Call(Lit("could not parse request body"), Lit("request_body"), Err()),
		).Else().If(Err().Op(":=").Id("json").Dot("Unmarshal").Call(Id("payload"), Id("dst")), Err().Op("!=").Nil()).Block(
			Id("d").Dot("setErr").Call(Lit("could not parse request body"), Lit("request_body"), Err()),
		),
		Return(Id("payload")),
	)
	f.Line()

	f.Func().Params(Id("d").Op("*").Id("typeDecoder")).Id("Err").Params().Params(Error()).Block(
		Return(Id("d").Dot("err")),
	)
	f.Line()

	f.Func().Params(Id("d").Op("*").Id("typeDecoder")).Id("setErr").Params(List(Id("msg"), Id("field")).String(), Err().Error()).Block(
		If(Err().Op("!=").Nil().Op("&&").Id("d").Dot("err").Op("==").Nil()).Block(
			Id("d").Dot("err").Op("=").Add(buildErrDetails("InvalidArgument", Id("msg"), Id("field"), Err().Dot("Error").Call())),
		),
	)
	f.Line()
}

func (b *Builder) typeName(param *est.Param, skipPtr bool) *Statement {
	typName := b.schemaTypeToGoType(param.Type)

	if param.IsPtr && !skipPtr {
		typName = Op("*").Add(typName)
	}

	return typName
}

func (b *Builder) authDataType() Code {
	s := ""

	if ah := b.res.App.AuthHandler; ah != nil && ah.AuthData != nil {
		typ := b.typeName(ah.AuthData, false)                           // Get the type
		statement := Var().Id("a").Add(typ)                             // place within a var statement to force `typ` to render as a type, rather than a statement
		s = strings.TrimPrefix(fmt.Sprintf("%#v", statement), "var a ") // render the statement, trimming the prefix
	}

	return Lit(s) // return a string literal
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
