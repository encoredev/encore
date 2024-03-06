package maingen

import (
	"cmp"
	"maps"
	"net/http"
	"slices"
	"sort"

	. "github.com/dave/jennifer/jen"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/v2/app"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/infra/pubsub"
)

type testParams struct {
	ExternalTestBinary bool
	EnvsToEmbed        map[string]string
}

func GenAppConfig(p GenParams, test option.Option[testParams]) *config.Static {
	allowHeaders, exposeHeaders := computeCORSHeaders(p.Desc)

	rootDir := p.Desc.MainModule.RootDir.ToDisplay()
	if GenerateForInternalPackageTests {
		rootDir = "testing_path:main"
	}

	cfg := &config.Static{
		EncoreCompiler: p.CompilerVersion,
		AppCommit: config.CommitInfo{
			Revision:    p.AppRevision,
			Uncommitted: p.AppUncommitted,
		},
		CORSAllowHeaders:   allowHeaders,
		CORSExposeHeaders:  exposeHeaders,
		PubsubTopics:       pubsubTopics(p.Gen, p.Desc),
		Testing:            test.Present(),
		TestServiceMap:     testServiceMap(p.Desc),
		TestAppRootPath:    rootDir,
		BundledServices:    bundledServices(p.Desc),
		EnabledExperiments: p.Gen.Build.Experiments.StringList(),
		EmbeddedEnvs:       make(map[string]string),
	}

	if test, ok := test.Get(); ok {
		cfg.PrettyPrintLogs = test.ExternalTestBinary
		maps.Copy(cfg.EmbeddedEnvs, test.EnvsToEmbed)
	}

	return cfg
}

func pubsubTopics(gen *codegen.Generator, appDesc *app.Desc) map[string]*config.StaticPubsubTopic {
	result := make(map[string]*config.StaticPubsubTopic)
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
		subs := make(map[string]*config.StaticPubsubSubscription)
		for _, b := range appDesc.Parse.PkgDeclBinds(topic) {
			qn := b.QualifiedName()
			for _, sub := range subsByTopic[qn] {
				// HACK: we should have a better way of knowing which service a subscription belongs to
				if svc, ok := appDesc.ServiceForPath(sub.File.Pkg.FSPath); ok {

					subs[sub.Name] = &config.StaticPubsubSubscription{
						Service:  svc.Name,
						SvcNum:   uint16(svc.Num),
						TraceIdx: gen.TraceNodes.Sub(sub),
					}
				}
			}
		}

		result[topic.Name] = &config.StaticPubsubTopic{
			Subscriptions: subs,
		}
	}

	return result
}

func bundledServices(appDesc *app.Desc) []string {
	// Sort the names by service number since that's what we're indexing by.
	svcs := slices.Clone(appDesc.Services)
	slices.SortFunc(svcs, func(a, b *app.Service) int {
		return cmp.Compare(a.Num, b.Num)
	})

	return fns.Map(svcs, func(svc *app.Service) string {
		return svc.Name
	})
}

func testServiceMap(appDesc *app.Desc) map[string]string {
	result := make(map[string]string)
	for _, svc := range appDesc.Services {
		path := svc.FSRoot.ToIO()
		if GenerateForInternalPackageTests {
			path = "testing_path:" + svc.Name
		}
		result[svc.Name] = path
	}
	return result
}

func enabledExperiments(experiments *experiments.Set) *Statement {
	list := experiments.StringList()

	if len(list) == 0 {
		return Nil()
	}

	return Index().String().ValuesFunc(func(g *Group) {
		for _, e := range list {
			g.Lit(e)
		}
	})
}

func computeCORSHeaders(appDesc *app.Desc) (allowHeaders, exposeHeaders []string) {
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
	}
	return allow.headers, expose.headers
}

// GenerateForInternalPackageTests is set to true
// when we're running the maingen package tests, to generate code without
// temporary directories in the file paths (for reproducibility).
var GenerateForInternalPackageTests = false
