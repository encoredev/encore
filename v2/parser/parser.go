package parser

import (
	"sync"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/scan"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/infra/cache"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/cron"
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

	usageExprs := usage.ParseExprs(p.c.Errs, pkgs, binds)
	return computeResult(p.c.Errs, p.usageResolver, pkgs, resources, binds, usageExprs)
}

// allParsers are all the resource parsers we support.
var allParsers = []*resourceparser.Parser{
	apis.Parser,
	cache.ClusterParser,
	cache.KeyspaceParser,
	config.LoadParser,
	cron.JobParser,
	metrics.MetricParser,
	pubsub.TopicParser,
	pubsub.SubscriptionParser,
	secrets.SecretsParser,
	sqldb.DatabaseParser,
	sqldb.NamedParser,
}

func newUsageResolver() *usage.Resolver {
	r := usage.NewResolver()
	usage.RegisterUsageResolver[*pubsub.Topic](r, pubsub.ResolveTopicUsage)
	return r
}
