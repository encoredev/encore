package apigen

import (
	"encr.dev/pkg/fns"
	"encr.dev/v2/codegen/infragen/cachegen"
	"encr.dev/v2/codegen/infragen/configgen"
	"encr.dev/v2/codegen/infragen/metricsgen"
	secretsgen "encr.dev/v2/codegen/infragen/secrets"
	"encr.dev/v2/codegen/internal/gen"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/resource/cache"
	"encr.dev/v2/parser/infra/resource/config"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/secrets"
)

func Process(gg *gen.Generator, parseResults []*apis.ParseResult) {
	for _, res := range parseResults {
		pkg := resources[0].DeclaredIn().Pkg
		switch key.kind {
		case resource.CacheKeyspace:
			cachegen.GenKeyspace(gg, pkg, fns.Map(resources, func(r resource.Resource) *cache.Keyspace {
				return r.(*cache.Keyspace)
			}))
		case resource.Metric:
			metricsgen.Gen(gg, pkg, fns.Map(resources, func(r resource.Resource) *metrics.Metric {
				return r.(*metrics.Metric)
			}))
		case resource.Secrets:
			secretsgen.Gen(gg, pkg, fns.Map(resources, func(r resource.Resource) *secrets.Secrets {
				return r.(*secrets.Secrets)
			}))
		case resource.ConfigLoad:
			configgen.Gen(gg, pkg, fns.Map(resources, func(r resource.Resource) *config.Load {
				return r.(*config.Load)
			}))
		}
	}
}
