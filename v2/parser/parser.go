package parser

import (
	"cmp"
	"slices"
	"sync"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/scan"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/infra/caches"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/crons"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
	"encr.dev/v2/parser/resource/usage"
)

func NewParser(c *parsectx.Context) *Parser {
	loader := pkginfo.New(c)
	schemaParser := schema.NewParser(c, loader)
	return &Parser{
		c:             c,
		loader:        loader,
		schemaParser:  schemaParser,
		registry:      resourceparser.NewRegistry(allParsers),
		usageResolver: newUsageResolver(),
	}
}

type Parser struct {
	c             *parsectx.Context
	loader        *pkginfo.Loader
	schemaParser  *schema.Parser
	registry      *resourceparser.Registry
	usageResolver *usage.Resolver
}

func (p *Parser) MainModule() *pkginfo.Module {
	return p.loader.MainModule()
}

func (p *Parser) RuntimeModule() *pkginfo.Module {
	return p.loader.RuntimeModule()
}

// Parse parses the given application for uses of the Encore API Framework
// and the Encore infrastructure SDK.
func (p *Parser) Parse() *Result {
	var (
		mu        sync.Mutex
		pkgs      []*pkginfo.Package
		resources []resource.Resource
		binds     []resource.Bind
	)

	scan.ProcessModule(p.c.Errs, p.loader, p.c.MainModuleDir, func(pkg *pkginfo.Package) {
		if pkg.Name == "main" {
			// Ignore main packages that aren't the main package we're building, if any.
			if mainPkg, ok := p.c.Build.MainPkg.Get(); !ok || pkg.ImportPath != mainPkg {
				return
			}
		}

		pass := &resourceparser.Pass{
			Context:      p.c,
			SchemaParser: p.schemaParser,
			Pkg:          pkg,
		}

		interested := p.registry.InterestedParsers(pkg)
		for _, p := range interested {
			p.Run(pass)
		}

		mu.Lock()
		pkgs = append(pkgs, pkg)
		resources = append(resources, pass.Resources()...)
		binds = append(binds, pass.Binds()...)
		mu.Unlock()
	})

	// Normally every resource is independent, but in the case of implicit sqldb
	// databases that come up as a result of having a "migrations" folder
	// we can end up with duplicate resources if we also have a sqldb.NewDatabase call.
	// Deduplicate them for now.
	resources, binds = deduplicateSQLDBResources(resources, binds)

	// Sort resources by package and position so the array is stable between runs
	// as we process modules in parallel we can't rely on the order of the
	// resources being stable coming into this function.
	slices.SortFunc(resources, func(a, b resource.Resource) int {
		p1, p2 := p.c.FS.Position(a.Pos()), p.c.FS.Position(b.Pos())
		if n := cmp.Compare(p1.Filename, p2.Filename); n != 0 {
			return n
		} else if n := cmp.Compare(p1.Line, p2.Line); n != 0 {
			return n
		} else if n := cmp.Compare(p1.Column, p2.Column); n != 0 {
			return n
		}
		return cmp.Compare(a.Pos(), b.Pos())
	})

	// Then sort the binds
	slices.SortFunc(binds, func(a, b resource.Bind) int {
		if a.Package() != b.Package() {
			return cmp.Compare(a.Package().FSPath, b.Package().FSPath)
		}

		return cmp.Compare(a.Pos(), b.Pos())
	})

	// Finally, sort the packages
	slices.SortFunc(pkgs, func(a, b *pkginfo.Package) int {
		return cmp.Compare(a.FSPath, b.FSPath)
	})

	// Because we've ordered pkgs and binds, usageExprs will be stable
	usageExprs := usage.ParseExprs(p.schemaParser, pkgs, binds)

	// Add the implicit sqldb usages.
	usageExprs = append(usageExprs, sqldb.ComputeImplicitUsage(p.c.Errs, pkgs, binds)...)

	return computeResult(p.c.Errs, p.MainModule(), p.usageResolver, pkgs, resources, binds, usageExprs)
}

// allParsers are all the resource parsers we support.
var allParsers = []*resourceparser.Parser{
	apis.Parser,
	caches.ClusterParser,
	caches.KeyspaceParser,
	config.LoadParser,
	crons.JobParser,
	metrics.MetricParser,
	pubsub.TopicParser,
	pubsub.SubscriptionParser,
	secrets.SecretsParser,
	sqldb.DatabaseParser,
	sqldb.MigrationParser,
	sqldb.NamedParser,
}

func newUsageResolver() *usage.Resolver {
	r := usage.NewResolver()
	// Infrastructure SDK
	usage.RegisterUsageResolver[*caches.Keyspace](r, caches.ResolveKeyspaceUsage)
	usage.RegisterUsageResolver[*config.Load](r, config.ResolveConfigUsage)
	usage.RegisterUsageResolver[*pubsub.Topic](r, pubsub.ResolveTopicUsage)
	usage.RegisterUsageResolver[*sqldb.Database](r, sqldb.ResolveDatabaseUsage)

	// API Framework
	usage.RegisterUsageResolver[*api.Endpoint](r, api.ResolveEndpointUsage)
	usage.RegisterUsageResolver[*authhandler.AuthHandler](r, authhandler.ResolveAuthHandlerUsage)
	usage.RegisterUsageResolver[*servicestruct.ServiceStruct](r, servicestruct.ResolveServiceStructUsage)
	return r
}

// deduplicateSQLDBResources deduplicates SQL Database resources and their associated binds
// in the case where we have multiple SQL Database resources with the same name,
// as a result of having an explicit bind (via sqldb.NewDatabase) and an implicit bind (via a "migrations" folder).
func deduplicateSQLDBResources(resources []resource.Resource, binds []resource.Bind) ([]resource.Resource, []resource.Bind) {
	bindsPerDB := make(map[string][]resource.Bind)
	bindsPerMigrationDir := make(map[paths.MainModuleRelSlash][]resource.Bind)
	for _, b := range binds {
		// All the binds we're interested in contain a resource and not a path.
		r := b.ResourceRef().Resource
		if db, ok := r.(*sqldb.Database); ok {
			bindsPerDB[db.Name] = append(bindsPerDB[db.Name], b)
			bindsPerMigrationDir[db.MigrationDir] = append(bindsPerMigrationDir[db.MigrationDir], b)
		}
	}

	resourcesToRemove := make(map[resource.Resource]bool)
	bindsToRemove := make(map[resource.Bind]bool)
	for _, binds := range bindsPerDB {
		implicitIdx := slices.IndexFunc(binds, func(b resource.Bind) bool {
			_, ok := b.(*resource.ImplicitBind)
			return ok
		})
		explicitIdx := slices.IndexFunc(binds, func(b resource.Bind) bool {
			_, ok := b.(*resource.ImplicitBind)
			return !ok
		})

		if implicitIdx >= 0 && explicitIdx >= 0 {
			// We have both types of binds. Delete the implicit one.
			implicit := binds[implicitIdx]
			bindsToRemove[implicit] = true

			// If they refer to different underlying resources, delete the implicit resource.
			explicit := binds[explicitIdx]
			if res := implicit.ResourceRef().Resource; res != nil && res != explicit.ResourceRef().Resource {
				resourcesToRemove[res] = true
			}
		}
	}

	// Now do the same based on migration dir
	for _, binds := range bindsPerMigrationDir {
		implicitIdx := slices.IndexFunc(binds, func(b resource.Bind) bool {
			_, ok := b.(*resource.ImplicitBind)
			return ok
		})
		explicitIdx := slices.IndexFunc(binds, func(b resource.Bind) bool {
			_, ok := b.(*resource.ImplicitBind)
			return !ok
		})

		if implicitIdx >= 0 && explicitIdx >= 0 {
			// We have both types of binds. Delete the implicit one.
			implicit := binds[implicitIdx]
			bindsToRemove[implicit] = true

			// If they refer to different underlying resources, delete the implicit resource.
			explicit := binds[explicitIdx]
			if res := implicit.ResourceRef().Resource; res != nil && res != explicit.ResourceRef().Resource {
				resourcesToRemove[res] = true
			}
		}
	}

	if len(resourcesToRemove) == 0 && len(bindsToRemove) == 0 {
		// Nothing to do.
		return resources, binds
	}

	updatedResources := make([]resource.Resource, 0, len(resources))
	updatedBinds := make([]resource.Bind, 0, len(binds))

	for _, r := range resources {
		if !resourcesToRemove[r] {
			updatedResources = append(updatedResources, r)
		}
	}
	for _, b := range binds {
		if !bindsToRemove[b] {
			updatedBinds = append(updatedBinds, b)
		}
	}

	return updatedResources, updatedBinds
}
