package codegen

import (
	"fmt"
	"net/http"
	"path"
	"sort"

	. "github.com/dave/jennifer/jen"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser"
	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
	"encr.dev/pkg/errlist"
)

const JsonPkg = "github.com/json-iterator/go"

type Builder struct {
	res *parser.Result

	marshaller *gocodegen.MarshallingCodeGenerator
	errors     *errlist.List

	// Cache of request/response encodings.
	// Access via b.requestEncoding(rpc) and b.responseEncoding(rpc).
	reqEncodingCache  map[*est.RPC][]*encoding.RequestEncoding
	respEncodingCache map[*est.RPC]*encoding.ResponseEncoding
}

func NewBuilder(res *parser.Result) *Builder {
	marshallerPkgPath := path.Join(res.Meta.ModulePath, "__encore", "etype")
	marshaller := gocodegen.NewMarshallingCodeGenerator(marshallerPkgPath, "Marshaller", false)

	return &Builder{
		res:        res,
		errors:     errlist.New(res.FileSet),
		marshaller: marshaller,

		reqEncodingCache:  make(map[*est.RPC][]*encoding.RequestEncoding),
		respEncodingCache: make(map[*est.RPC]*encoding.ResponseEncoding),
	}
}

func (b *Builder) Main(compilerVersion string, mainPkgPath, mainFuncName string) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	if mainPkgPath != "" {
		f = NewFilePathName(mainPkgPath, "main")
	} else {
		f = NewFile("main")
	}

	b.registerImports(f, mainPkgPath)
	b.importServices(f, mainPkgPath)

	mwNames, mwCode := b.RenderMiddlewares(mainPkgPath)

	svcNames := make([]string, 0, len(b.res.App.Services))
	for _, svc := range b.res.App.Services {
		svcNames = append(svcNames, svc.Name)
	}
	sort.Strings(svcNames)

	corsAllowHeaders, corsExposeHeaders, err := b.computeCORSHeaders()
	if err != nil {
		b.error(eerror.Wrap(err, "codegen", "failed to compute CORS headers", nil))
	}

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       b.authDataType(),
			Id("EncoreCompiler"): Lit(compilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(b.res.Meta.AppRevision),
				Id("Uncommitted"): Lit(b.res.Meta.UncommittedChanges),
			}),
			Id("CORSAllowHeaders"):  corsAllowHeaders,
			Id("CORSExposeHeaders"): corsExposeHeaders,
			Id("PubsubTopics"):      b.computeStaticPubsubConfig(),
			Id("Testing"):           False(),
			Id("TestService"):       Lit(""),
			Id("BundledServices"):   b.computeBundledServices(),
		})
		g.Id("handlers").Op(":=").Add(b.computeHandlerRegistrationConfig(mwNames))
		g.Id("svcInit").Op(":=").Add(b.computeServiceInitConfig())

		authHandlerExpr := Nil()
		if ah := b.res.App.AuthHandler; ah != nil {
			authHandlerExpr = Qual(ah.Svc.Root.ImportPath, b.authHandlerName(ah))
		}

		g.Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
			Id("ServiceInit"): Id("svcInit"),
			Id("AuthHandler"): authHandlerExpr,
		}))
	})
	f.Line()

	if mainFuncName == "" {
		mainFuncName = "main"
	}

	f.Func().Id(mainFuncName).Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)

	for _, c := range mwCode {
		f.Line()
		f.Add(c)
	}

	return f, b.errors.Err()
}

func (b *Builder) computeStaticPubsubConfig() Code {
	pubsubTopicDict := Dict{}
	for _, topic := range b.res.App.PubSubTopics {
		subscriptions := Dict{}

		for _, sub := range topic.Subscribers {
			traceID := int(b.res.Nodes[sub.DeclFile.Pkg][sub.IdentAST].Id)

			subscriptions[Lit(sub.Name)] = Values(Dict{
				Id("Service"):  Lit(sub.DeclFile.Pkg.Service.Name),
				Id("TraceIdx"): Lit(traceID),
			})
		}

		pubsubTopicDict[Lit(topic.Name)] = Values(Dict{
			Id("Subscriptions"): Map(String()).Op("*").Qual("encore.dev/appruntime/config", "StaticPubsubSubscription").Values(subscriptions),
		})
	}
	return Map(String()).Op("*").Qual("encore.dev/appruntime/config", "StaticPubsubTopic").Values(pubsubTopicDict)
}

func (b *Builder) computeCORSHeaders() (allowHeaders, exposeHeaders Code, err error) {
	// computeResponseHeaders computes the headers that are part of the request for a given RPC.
	computeRequestHeaders := func(rpc *est.RPC) []*encoding.ParameterEncoding {
		reqs := b.reqEncoding(rpc)
		var params []*encoding.ParameterEncoding
		for _, r := range reqs {
			params = append(params, r.HeaderParameters...)
		}
		return params
	}

	// computeResponseHeaders computes the headers that are part of the response for a given RPC.
	computeResponseHeaders := func(rpc *est.RPC) []*encoding.ParameterEncoding {
		if resp := b.respEncoding(rpc); resp != nil {
			return resp.HeaderParameters
		}
		return nil
	}

	type result struct {
		computeHeaders func(rpc *est.RPC) []*encoding.ParameterEncoding
		seenHeader     map[string]bool
		headers        []string
		out            Code
	}

	var (
		allow  = &result{computeHeaders: computeRequestHeaders}
		expose = &result{computeHeaders: computeResponseHeaders}
	)

	for _, res := range []*result{allow, expose} {
		res.seenHeader = make(map[string]bool)

		for _, svc := range b.res.App.Services {
			for _, rpc := range svc.RPCs {
				for _, param := range res.computeHeaders(rpc) {
					name := http.CanonicalHeaderKey(param.Name)
					if !res.seenHeader[name] {
						res.seenHeader[name] = true
						res.headers = append(res.headers, name)
					}
				}
			}
		}
		// Sort the headers so that the generated code is deterministic.
		sort.Strings(res.headers)

		// Construct the code snippet ([]string{"Authorization", "X-Bar", "X-Foo", ...})
		if len(res.headers) == 0 {
			res.out = Nil()
		} else {
			usedHeadersCode := make([]Code, 0, len(res.headers))
			for _, header := range res.headers {
				usedHeadersCode = append(usedHeadersCode, Lit(header))
			}
			res.out = Index().String().Values(usedHeadersCode...)
		}
	}

	return allow.out, expose.out, nil
}

func (b *Builder) computeHandlerRegistrationConfig(mwNames map[*est.Middleware]*Statement) *Statement {
	return Index().Qual("encore.dev/appruntime/api", "HandlerRegistration").CustomFunc(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	}, func(g *Group) {
		var globalMW []*est.Middleware
		for _, mw := range b.res.App.Middleware {
			if mw.Global {
				globalMW = append(globalMW, mw)
			}
		}

		for _, svc := range b.res.App.Services {
			for _, rpc := range svc.RPCs {
				// Compute middleware for this service.
				rpcMW := b.res.App.MatchingMiddleware(rpc)
				mwExpr := Nil()
				if len(rpcMW) > 0 {
					mwExpr = Index().Op("*").Qual("encore.dev/appruntime/api", "Middleware").ValuesFunc(func(g *Group) {
						for _, mw := range rpcMW {
							g.Add(mwNames[mw])
						}
					})
				}

				g.Values(Dict{
					Id("Handler"):    Qual(svc.Root.ImportPath, b.rpcHandlerName(rpc)),
					Id("Middleware"): mwExpr,
				})
			}
		}
	})
}

func (b *Builder) computeServiceInitConfig() *Statement {
	return Index().Qual("encore.dev/appruntime/service", "Initializer").CustomFunc(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	}, func(g *Group) {
		for _, svc := range b.res.App.Services {
			if svc.Struct != nil {
				g.Qual(svc.Root.ImportPath, b.serviceStructName(svc.Struct))
			}
		}
	})
}

func (b *Builder) importServices(f *File, mainPkgPath string) {
	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialised.
	for _, svc := range b.res.App.Services {
		if svc.Root.ImportPath != mainPkgPath {
			f.Anon(svc.Root.ImportPath)
		}
	}
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
		typName := b.schemaTypeToGoType(derefPointer(ah.AuthData.Type))
		if ah.AuthData.IsPointer() {
			// reflect.TypeOf((*T)(nil))
			return Qual("reflect", "TypeOf").Call(Parens(Op("*").Add(typName)).Call(Nil()))
		} else {
			// reflect.TypeOf(T{})
			return Qual("reflect", "TypeOf").Call(typName.Values())
		}
	}
	return Nil()
}

func (b *Builder) computeBundledServices() Code {
	sortedNames := make([]string, 0, len(b.res.App.Services))
	for _, svc := range b.res.App.Services {
		sortedNames = append(sortedNames, svc.Name)
	}
	sort.Strings(sortedNames)

	return Index().String().ValuesFunc(func(g *Group) {
		for _, name := range sortedNames {
			g.Lit(name)
		}
	})
}

func (b *Builder) getSvcNum(svc *est.Service) int {
	sortedNames := make([]string, 0, len(b.res.App.Services))
	for _, svc := range b.res.App.Services {
		sortedNames = append(sortedNames, svc.Name)
	}
	sort.Strings(sortedNames)

	for i, name := range sortedNames {
		if name == svc.Name {
			return i + 1
		}
	}
	b.errorf("cannot find service %s", svc.Name)
	return 0
}

func (b *Builder) error(err error) {
	panic(bailout{err})
}

func (b *Builder) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

type bailout struct {
	err error
}

func (b bailout) String() string {
	return fmt.Sprintf("bailout(%s)", b.err)
}

// reqEncoding returns the request encoding for the given RPC.
// If the RPC has no request schema, it returns nil.
// If the parsing fails it logs an error and returns nil.
func (b *Builder) reqEncoding(rpc *est.RPC) []*encoding.RequestEncoding {
	if cached, ok := b.reqEncodingCache[rpc]; ok {
		return cached
	}

	var result []*encoding.RequestEncoding
	if rpc.Request != nil {
		var err error
		result, err = encoding.DescribeRequest(b.res.Meta, rpc.Request.Type, nil, rpc.HTTPMethods...)
		if err != nil {
			b.errors.Addf(rpc.Func.Pos(), "failed to describe request: %v", err)
			result = nil
		}
	}

	// Cache the result regardless of success/failure.
	b.reqEncodingCache[rpc] = result
	return result
}

// respEncoding returns the response encoding for the given RPC.
// If the RPC has no response schema, it returns nil.
// If the parsing fails it logs an error and returns nil.
func (b *Builder) respEncoding(rpc *est.RPC) *encoding.ResponseEncoding {
	if cached, ok := b.respEncodingCache[rpc]; ok {
		return cached
	}

	var result *encoding.ResponseEncoding
	if rpc.Response != nil {
		var err error
		result, err = encoding.DescribeResponse(b.res.Meta, rpc.Response.Type, nil)
		if err != nil {
			b.errors.Addf(rpc.Func.Pos(), "failed to describe response: %v", err)
			result = nil
		}
	}

	// Cache the result regardless of success/failure.
	b.respEncodingCache[rpc] = result
	return result
}
