package infragen

import (
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/infragen/cachegen"
	"encr.dev/v2/codegen/infragen/configgen"
	"encr.dev/v2/codegen/infragen/metricsgen"
	"encr.dev/v2/codegen/infragen/pubsubgen"
	"encr.dev/v2/codegen/infragen/secretsgen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/infra/caches"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
	"encr.dev/v2/parser/resource"
)

func Process(gg *codegen.Generator, appDesc *app.Desc) {
	type groupKey struct {
		pkg  paths.Pkg
		kind resource.Kind
	}

	groups := make(map[groupKey][]resource.Resource)
	pkgMap := make(map[paths.Pkg]*pkginfo.Package)
	for _, r := range appDesc.Parse.Resources() {
		// Group by package.
		var pkg *pkginfo.Package
		switch r := r.(type) {
		case *caches.Keyspace:
			pkg = r.File.Pkg
		case *metrics.Metric:
			pkg = r.File.Pkg
		case *secrets.Secrets:
			pkg = r.File.Pkg
		case *pubsub.Subscription:
			pkg = r.File.Pkg
		case *config.Load:
			pkg = r.File.Pkg
		default:
			continue
		}

		key := groupKey{pkg: pkg.ImportPath, kind: r.Kind()}
		groups[key] = append(groups[key], r)
		pkgMap[pkg.ImportPath] = pkg
	}

	for key, resources := range groups {
		pkg := pkgMap[key.pkg]
		switch key.kind {
		case resource.CacheKeyspace:
			cachegen.GenKeyspace(gg, pkg, fns.Map(resources, func(r resource.Resource) *caches.Keyspace {
				return r.(*caches.Keyspace)
			}))
		case resource.Metric:
			metricsgen.Gen(gg, pkg, fns.Map(resources, func(r resource.Resource) *metrics.Metric {
				return r.(*metrics.Metric)
			}))
		case resource.PubSubSubscription:
			pubsubgen.Gen(gg, pkg, appDesc, fns.Map(resources, func(r resource.Resource) *pubsub.Subscription {
				return r.(*pubsub.Subscription)
			}))
		case resource.Secrets:
			svc, _ := appDesc.ServiceForPath(pkg.FSPath)
			secretsgen.Gen(gg, option.AsOptional(svc), pkg, fns.Map(resources, func(r resource.Resource) *secrets.Secrets {
				return r.(*secrets.Secrets)
			}))
		case resource.ConfigLoad:
			svc, ok := appDesc.ServiceForPath(pkg.FSPath)
			if !ok {
				gg.Errs.Addf(resources[0].(*config.Load).AST.Pos(), "config loads must be declared in a service package")
				continue
			}

			configgen.Gen(gg, svc, pkg, fns.Map(resources, func(r resource.Resource) *config.Load {
				return r.(*config.Load)
			}))
		}
	}
}
