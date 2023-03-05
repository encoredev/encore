package infra

import (
	"golang.org/x/exp/slices"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/resource/cache"
	"encr.dev/v2/parser/infra/resource/config"
	"encr.dev/v2/parser/infra/resource/cron"
	"encr.dev/v2/parser/infra/resource/metrics"
	"encr.dev/v2/parser/infra/resource/pubsub"
	"encr.dev/v2/parser/infra/resource/secrets"
	"encr.dev/v2/parser/infra/resource/sqldb"
)

func newParserRegistry(parsers []*resource.Parser) *parserRegistry {
	forImports, always := parsersForImports(parsers)
	return &parserRegistry{
		parsers:              parsers,
		alwaysInterested:     always,
		interestedForImports: forImports,
	}
}

type parserRegistry struct {
	parsers []*resource.Parser

	// alwaysInterested are the parses that should be run against all packages.
	alwaysInterested []*resource.Parser

	// interestedForImports maps from import paths to the list of parsers
	// interested in packages that import that package.
	interestedForImports map[paths.Pkg][]*resource.Parser
}

// InterestedParsers returns the parsers interested in processing a given package.
func (r *parserRegistry) InterestedParsers(pkg *pkginfo.Package) []*resource.Parser {
	parsers := slices.Clone(r.alwaysInterested)

	for imp := range pkg.Imports {
		// Add the parsers that are interested in this package as long
		// as they're not already in the list.
		// Note: this is O(n^2) but n is small, so this should be faster than maintaining a set.
		for _, p := range r.interestedForImports[imp] {
			if !slices.Contains(parsers, p) {
				parsers = append(parsers, p)
			}
		}
	}

	return parsers
}

// parsersForImports returns a map from package paths to the list of parsers
// interested in that package.
func parsersForImports(parsers []*resource.Parser) (forImports map[paths.Pkg][]*resource.Parser, always []*resource.Parser) {
	forImports = make(map[paths.Pkg][]*resource.Parser)

ParserLoop:
	for _, parser := range parsers {
		for _, imp := range parser.RequiredImports {
			if imp == "*" {
				always = append(always, parser)
				continue ParserLoop
			}
			forImports[imp] = append(forImports[imp], parser)
		}
	}
	return forImports, always
}

// allParsers are all the resource parsers we support.
var allParsers = []*resource.Parser{
	cache.ClusterParser,
	cache.KeyspaceParser,
	config.LoadParser,
	cron.JobParser,
	metrics.MetricParser,
	pubsub.TopicParser,
	pubsub.SubscriptionParser,
	secrets.SecretsParser,
	sqldb.DatabaseParser,
}
