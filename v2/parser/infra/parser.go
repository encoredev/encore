package infra

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/resource/cache"
	"encr.dev/v2/parser/infra/resource/config"
	"encr.dev/v2/parser/infra/resource/cron"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/pubsub"
	"encr.dev/v2/parser/infra/resource/secrets"
	"encr.dev/v2/parser/infra/resource/sqldb"
)

func NewParser(c *parsectx.Context, schema *schema.Parser) *Parser {
	return &Parser{
		c:      c,
		schema: schema,
	}
}

type Parser struct {
	c      *parsectx.Context
	schema *schema.Parser
}

// ParseResult describes the results of parsing a given package.
type ParseResult struct {
	Resources []resource.Resource
}

// allParsers are all the resource parsers we support.
var allParsers = []*resource.Parser{
	cache.ClusterParser,
	cache.KeyspaceParser,
	config.ConfigParser,
	cron.JobParser,
	metrics.MetricParser,
	pubsub.TopicParser,
	pubsub.SubscriptionParser,
	secrets.SecretsParser,
	sqldb.DatabaseParser,
}

func (p *Parser) Parse(pkg *pkginfo.Package) ParseResult {
	var res ParseResult

	// TODO respect dependency order
	for _, parser := range allParsers {
		resources := parser.Run(&resource.Pass{
			Context:      p.c,
			SchemaParser: p.schema,
			Pkg:          pkg,
		})
		res.Resources = append(res.Resources, resources...)
	}

	// TODO validation

	return res
}
