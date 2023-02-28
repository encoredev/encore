package infragen

import (
	"encr.dev/pkg/fns"
	"encr.dev/v2/codegen/infragen/cachegen"
	"encr.dev/v2/codegen/infragen/metricsgen"
	secretsgen "encr.dev/v2/codegen/infragen/secrets"
	"encr.dev/v2/codegen/internal/gen"
	"encr.dev/v2/internal/overlay"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/resource/cache"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/secrets"
)

func New(pc *parsectx.Context) *Generator {
	return &Generator{pc: pc}
}

type Generator struct {
	pc *parsectx.Context
}

func (g *Generator) Generate(resources []resource.Resource) []overlay.File {
	type groupKey struct {
		pkg  paths.Pkg
		kind resource.Kind
	}
	groups := make(map[groupKey][]resource.Resource)
	for _, r := range resources {
		key := groupKey{r.DeclaredIn().Pkg.ImportPath, r.Kind()}
		groups[key] = append(groups[key], r)
	}

	gg := gen.New(g.pc)

	for key, resources := range groups {
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
		}
	}

	return gg.Overlays()
}
