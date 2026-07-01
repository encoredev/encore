// Package eclmeta bridges Encore application metadata
// (encore.parser.meta.v2) to ECL evaluation: it turns the resources an app
// declares into ECL resources and evaluates a policy against them.
package eclmeta

import (
	"sort"

	"encr.dev/pkg/ecl"
	metav2 "encr.dev/proto/encore/parser/meta/v2"
)

// Enrich adds per-resource selector attributes (e.g. team, tags) and config
// that aren't present in the metadata. It is called for each resource produced
// from the metadata, with the resource's kind and name and a pointer to the
// resource to mutate.
type Enrich func(kind, name string, r *ecl.Resource)

// Resources builds the ECL resources to evaluate from an app's metadata: one
// resource per app-discovered resource (service, sql_database, bucket,
// pubsub_topic, cache, secret, cron_job), carrying its name and the declared
// config available in the metadata.
//
// ECL-managed kinds (service_instance, sql_cluster) are not produced here — ECL
// instantiates those itself from named/dynamic blocks. If enrich is non-nil it
// is called for each resource.
func Resources(md *metav2.Resources, enrich Enrich) []*ecl.Resource {
	if md == nil {
		return nil
	}
	var out []*ecl.Resource
	add := func(kind, name string, config map[string]ecl.Value) {
		r := &ecl.Resource{Kind: kind, Name: name, Config: config}
		if enrich != nil {
			enrich(kind, name, r)
		}
		out = append(out, r)
	}

	for _, n := range sortedKeys(md.Services) {
		add("service", md.Services[n].GetName(), nil)
	}
	for _, n := range sortedKeys(md.SqlDatabases) {
		add("sql_database", md.SqlDatabases[n].GetName(), nil)
	}
	for _, n := range sortedKeys(md.Buckets) {
		b := md.Buckets[n]
		add("bucket", b.GetName(), map[string]ecl.Value{
			"public_access": ecl.Bool(b.GetPublic()),
			"versioning":    ecl.Bool(b.GetVersioned()),
		})
	}
	for _, n := range sortedKeys(md.PubsubTopics) {
		add("pubsub_topic", md.PubsubTopics[n].GetName(), nil)
	}
	for _, n := range sortedKeys(md.CacheClusters) {
		add("cache", md.CacheClusters[n].GetName(), nil)
	}
	for _, n := range sortedKeys(md.CronJobs) {
		add("cron_job", md.CronJobs[n].GetName(), nil)
	}
	for _, n := range sortedKeys(md.Secrets) {
		add("secret", secretName(md.Secrets[n]), nil)
	}
	return out
}

// Evaluate evaluates the rule set against an app's metadata for an environment
// with the given selector attributes. It is shorthand for
// rs.EvaluateEnv(envAttrs, Resources(md, enrich)).
func Evaluate(rs *ecl.RuleSet, envAttrs map[string]ecl.Value, md *metav2.Resources, enrich Enrich) (*ecl.EnvResult, error) {
	return rs.EvaluateEnv(envAttrs, Resources(md, enrich))
}

func secretName(s *metav2.Secret) string {
	if g := s.GetGlobal(); g != nil {
		return g.GetName()
	}
	if l := s.GetLocal(); l != nil {
		return l.GetName()
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
