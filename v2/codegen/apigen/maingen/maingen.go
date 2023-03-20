package maingen

import (
	"net/http"
	"sort"

	. "github.com/dave/jennifer/jen"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/infra/pubsub"
)

type GenParams struct {
	Gen        *codegen.Generator
	Desc       *app.Desc
	MainModule *pkginfo.Module

	// CompilerVersion is the version of the compiler to embed in the generated code.
	CompilerVersion string
	// AppRevision is the revision of the app to embed in the generated code.
	AppRevision string
	// AppUncommitted tracks whether there were uncommitted changes in the app
	// at the time of build.
	AppUncommitted bool

	APIHandlers    map[*api.Endpoint]*codegen.VarDecl
	AuthHandler    option.Option[*codegen.VarDecl]
	Middleware     map[*middleware.Middleware]*codegen.VarDecl
	ServiceStructs map[*app.Service]*codegen.VarDecl
}

func Gen(p GenParams) {
	gen, appDesc := p.Gen, p.Desc

	mainPkgDir := p.MainModule.RootDir.Join("__encore", "main")
	mainPkgPath := paths.Pkg(p.MainModule.Path).JoinSlash("__encore", "main")

	file := gen.InjectFile(mainPkgPath, "main", mainPkgDir, "main.go", "main")

	f := file.Jen

	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialized.
	for _, svc := range appDesc.Services {
		svc.Framework.ForAll(func(svcDesc *apiframework.ServiceDesc) {
			rootPkg := svcDesc.RootPkg
			if rootPkg.ImportPath != mainPkgPath {
				f.Anon(rootPkg.ImportPath.String())
			}
		})
	}

	authHandler := option.
		Map(p.AuthHandler, func(ah *codegen.VarDecl) *Statement { return ah.Qual() }).
		GetOrElse(Nil())

	allowHeaders, exposeHeaders := computeCORSHeaders(appDesc)

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       authDataType(gen.Util, appDesc),
			Id("EncoreCompiler"): Lit(p.CompilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(p.AppRevision),
				Id("Uncommitted"): Lit(p.AppUncommitted),
			}),
			Id("CORSAllowHeaders"):  allowHeaders,
			Id("CORSExposeHeaders"): exposeHeaders,
			Id("PubsubTopics"):      pubsubTopics(appDesc),
			Id("Testing"):           False(),
			Id("TestService"):       Lit(""),
			Id("BundledServices"):   bundledServices(appDesc),
		})

		g.Id("handlers").Op(":=").Add(computeHandlerRegistrationConfig(appDesc, p.APIHandlers, p.Middleware))

		g.Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
			Id("ServiceInit"): serviceInitConfig(p.ServiceStructs),
			Id("AuthHandler"): authHandler,
		}))
	})

	f.Func().Id("main").Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)
}

func pubsubTopics(appDesc *app.Desc) *Statement {
	return Map(String()).Op("*").Qual("encore.dev/appruntime/config", "StaticPubsubTopic").Values(DictFunc(func(d Dict) {
		// Get all the topics and subscriptions
		var (
			topics      []*pubsub.Topic
			subsByTopic = make(map[pkginfo.QualifiedName][]*pubsub.Subscription)
		)
		for _, r := range appDesc.Parse.Resources() {
			switch r := r.(type) {
			case *pubsub.Topic:
				topics = append(topics, r)
			case *pubsub.Subscription:
				subsByTopic[r.Topic] = append(subsByTopic[r.Topic], r)
			}
		}

		for _, topic := range topics {
			subs := DictFunc(func(d Dict) {
				for _, b := range appDesc.Parse.PkgDeclBinds(topic) {
					qn := b.QualifiedName()
					for _, sub := range subsByTopic[qn] {
						// TODO we should have a better way of knowing which service a subscription belongs to
						if svc, ok := appDesc.ServiceForPath(sub.File.Pkg.FSPath); ok {
							d[Lit(sub.Name)] = Values(Dict{
								Id("Service"):  Lit(svc.Name),
								Id("TraceIdx"): Lit(0), // TODO node id
							})
						}
					}
				}
			})

			d[Lit(topic.Name)] = Values(Dict{
				Id("Subscriptions"): Map(String()).Op("*").Qual(
					"encore.dev/appruntime/config", "StaticPubsubSubscription").Values(subs),
			})
		}
	}))
}

func bundledServices(appDesc *app.Desc) *Statement {
	// Sort the names by service name so the output is deterministic.
	names := fns.Map(appDesc.Services, func(svc *app.Service) string {
		return svc.Name
	})
	sort.Strings(names)

	return Index().String().ValuesFunc(func(g *Group) {
		for _, name := range names {
			g.Lit(name)
		}
	})
}

func computeCORSHeaders(appDesc *app.Desc) (allowHeaders, exposeHeaders *Statement) {
	// computeRequestHeaders computes the headers that are part of the request for a given RPC.
	computeRequestHeaders := func(ep *api.Endpoint) []*apienc.ParameterEncoding {
		var params []*apienc.ParameterEncoding
		for _, r := range ep.RequestEncoding() {
			params = append(params, r.HeaderParameters...)
		}
		return params
	}

	// computeResponseHeaders computes the headers that are part of the response for a given RPC.
	computeResponseHeaders := func(ep *api.Endpoint) []*apienc.ParameterEncoding {
		return ep.ResponseEncoding().HeaderParameters
	}

	type result struct {
		computeHeaders func(ep *api.Endpoint) []*apienc.ParameterEncoding
		seenHeader     map[string]bool
		headers        []string
		out            *Statement
	}

	var (
		allow  = &result{computeHeaders: computeRequestHeaders}
		expose = &result{computeHeaders: computeResponseHeaders}
	)

	for _, res := range []*result{allow, expose} {
		res.seenHeader = make(map[string]bool)

		for _, svc := range appDesc.Services {
			svc.Framework.ForAll(func(fw *apiframework.ServiceDesc) {
				for _, ep := range fw.Endpoints {
					for _, param := range res.computeHeaders(ep) {
						name := http.CanonicalHeaderKey(param.WireName)
						if !res.seenHeader[name] {
							res.seenHeader[name] = true
							res.headers = append(res.headers, name)
						}
					}
				}
			})
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

	return allow.out, expose.out
}

func computeHandlerRegistrationConfig(appDesc *app.Desc, epMap map[*api.Endpoint]*codegen.VarDecl, mwMap map[*middleware.Middleware]*codegen.VarDecl) *Statement {
	return Index().Qual("encore.dev/appruntime/api", "HandlerRegistration").CustomFunc(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	}, func(g *Group) {
		for _, svc := range appDesc.Services {
			svc.Framework.ForAll(func(fw *apiframework.ServiceDesc) {
				for _, ep := range fw.Endpoints {
					// Compute middleware for this service.
					rpcMW := appDesc.MatchingMiddleware(ep)
					mwExpr := Nil()
					if len(rpcMW) > 0 {
						mwExpr = Index().Op("*").Qual("encore.dev/appruntime/api", "Middleware").ValuesFunc(func(g *Group) {
							for _, mw := range rpcMW {
								g.Add(mwMap[mw].Qual())
							}
						})
					}

					g.Values(Dict{
						Id("Handler"):    epMap[ep].Qual(),
						Id("Middleware"): mwExpr,
					})
				}
			})
		}
	})
}

func authDataType(gu *genutil.Helper, desc *app.Desc) *Statement {
	authHandler := option.FlatMap(desc.Framework, func(fw *apiframework.AppDesc) option.Option[*authhandler.AuthHandler] { return fw.AuthHandler })
	authData := option.FlatMap(authHandler, func(ah *authhandler.AuthHandler) option.Option[*schema.TypeDeclRef] { return ah.AuthData })

	return option.Map(authData, func(ref *schema.TypeDeclRef) *Statement {
		return Qual("reflect", "TypeOf").Call(gu.Zero(ref.ToType()))
	}).GetOrElse(Nil())
}

func serviceInitConfig(svcStructs map[*app.Service]*codegen.VarDecl) *Statement {
	// Get the map keys and sort them for deterministic output.
	svcs := maps.Keys(svcStructs)
	slices.SortFunc(svcs, func(a, b *app.Service) bool {
		return a.Name < b.Name
	})

	return Index().Qual("encore.dev/appruntime/service", "Initializer").CustomFunc(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	}, func(g *Group) {
		for _, svc := range svcs {
			g.Add(svcStructs[svc].Qual())
		}
	})
}
