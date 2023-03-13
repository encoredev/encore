package infra

import (
	"encr.dev/v2/parser/infra/cache"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/cron"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource/resourceparser"
)

// allParsers are all the resource parsers we support.
var allParsers = []*resourceparser.Parser{
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
