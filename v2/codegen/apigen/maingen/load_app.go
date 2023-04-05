package maingen

import (
	"net/http"
	"sort"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/infra/pubsub"
)

type testParams struct {
	EnvsToEmbed []string
}

func genLoadApp(p GenParams, test option.Option[testParams]) {
	var (
		rtconfDir  = p.RuntimeModule.RootDir.Join("appruntime", "shared", "appconf")
		rtconfPkg  = paths.Pkg("encore.dev/appruntime/shared/appconf")
		rtconfName = "appconf"
	)

	file := p.Gen.InjectFile(rtconfPkg, rtconfName, rtconfDir, "staticcfg.go", "staticcfg")
	f := file.Jen

	allowHeaders, exposeHeaders := computeCORSHeaders(p.Desc)

	var envsToEmbed []string
	if test, ok := test.Get(); ok {
		envsToEmbed = test.EnvsToEmbed
	}

	f.Func().Id("init").Params().BlockFunc(func(g *Group) {
		staticCfg := Dict{
			//Id("AuthData"):       authDataType(p.Gen.Util, p.Desc),
			Id("EncoreCompiler"): Lit(p.CompilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/exported/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(p.AppRevision),
				Id("Uncommitted"): Lit(p.AppUncommitted),
			}),
			Id("CORSAllowHeaders"):  allowHeaders,
			Id("CORSExposeHeaders"): exposeHeaders,
			Id("PubsubTopics"):      pubsubTopics(p.Gen, p.Desc),
			Id("Testing"):           Lit(test.Present()),
			Id("TestServiceMap"):    testServiceMap(p.Desc),
			Id("BundledServices"):   bundledServices(p.Desc),
		}

		if len(envsToEmbed) > 0 {
			staticCfg[Id("TestAsExternalBinary")] = True()
			for _, env := range envsToEmbed {
				key, value, _ := strings.Cut(env, "=")
				g.Qual("encore.dev/appruntime/shared/encoreenv", "Set").Call(Lit(key), Lit(value))
			}
		}

		g.Id("Static").Op("=").Op("&").Qual("encore.dev/appruntime/exported/config", "Static").Values(staticCfg)

		//g.Id("handlers").Op(":=").Add(computeHandlerRegistrationConfig(p.Desc, p.APIHandlers, p.Middleware))
		//
		//g.Return(Op("&").Qual("encore.dev/appruntime/apisdk/app/appinit", "LoadData").Values(Dict{
		//	Id("StaticCfg"):   Id("static"),
		//	Id("APIHandlers"): Id("handlers"),
		//	Id("ServiceInit"): serviceInitConfig(p.ServiceStructs),
		//	Id("AuthHandler"): authHandler,
		//}))
	})
}

func pubsubTopics(gen *codegen.Generator, appDesc *app.Desc) *Statement {
	return Map(String()).Op("*").Qual("encore.dev/appruntime/exported/config", "StaticPubsubTopic").Values(DictFunc(func(d Dict) {
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
						// HACK: we should have a better way of knowing which service a subscription belongs to
						if svc, ok := appDesc.ServiceForPath(sub.File.Pkg.FSPath); ok {

							d[Lit(sub.Name)] = Values(Dict{
								Id("Service"):  Lit(svc.Name),
								Id("TraceIdx"): Lit(gen.TraceNodes.Sub(sub)),
							})
						}
					}
				}
			})

			d[Lit(topic.Name)] = Values(Dict{
				Id("Subscriptions"): Map(String()).Op("*").Qual(
					"encore.dev/appruntime/exported/config", "StaticPubsubSubscription").Values(subs),
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

func testServiceMap(appDesc *app.Desc) *Statement {
	return Map(String()).String().Values(DictFunc(func(d Dict) {
		for _, svc := range appDesc.Services {
			d[Lit(svc.Name)] = Lit(svc.FSRoot.ToIO())
		}
	}))
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
	return Index().Qual("encore.dev/appruntime/apisdk/api", "HandlerRegistration").CustomFunc(Options{
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
						mwExpr = Index().Op("*").Qual("encore.dev/appruntime/apisdk/api", "Middleware").ValuesFunc(func(g *Group) {
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
