package caches

import (
	"testing"

	"encr.dev/v2/parser/resource/resourcetest"
)

func TestParseCluster(t *testing.T) {
	tests := []resourcetest.Case[*Cluster]{
		{
			Name: "basic",
			Code: `
// Cluster docs
var x = cache.NewCluster("name", cache.ClusterConfig{})
`,
			Want: &Cluster{
				Name:           "name",
				Doc:            "Cluster docs\n",
				EvictionPolicy: "allkeys-lru",
			},
		},
		{
			Name: "with_eviction_policy",
			Code: `
// Cluster docs
var x = cache.NewCluster("name", cache.ClusterConfig{EvictionPolicy: cache.VolatileLFU})
`,
			Want: &Cluster{
				Name:           "name",
				Doc:            "Cluster docs\n",
				EvictionPolicy: "volatile-lfu",
			},
		},
		{
			Name: "with_bad_eviction_policy",
			Code: `
// Cluster docs
var x = cache.NewCluster("name", cache.ClusterConfig{EvictionPolicy: "x"})
`,
			WantErrs: []string{`.*Invalid Cache Eviction Policy.*`},
		},
		{
			Name: "with_invalid_eviction_policy",
			Code: `
// Cluster docs
var x = cache.NewCluster("name", cache.ClusterConfig{EvictionPolicy: cache.NonExisting})
`,
			WantErrs: []string{
				".*Field `EvictionPolicy` must be a constant literal.*",
				"Must be one of the constants defined in the cache package",
			},
		},
	}

	resourcetest.Run(t, ClusterParser, tests)
}
