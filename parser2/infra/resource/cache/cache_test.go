package cache

import (
	"testing"

	"encr.dev/parser2/infra/resource/resourcetest"
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
			WantErrs: []string{`.*invalid "EvictionPolicy" value: "x"`},
		},
		{
			Name: "with_invalid_eviction_policy",
			Code: `
// Cluster docs
var x = cache.NewCluster("name", cache.ClusterConfig{EvictionPolicy: cache.NonExisting})
`,
			WantErrs: []string{`.*field EvictionPolicy must be a constant literal`},
		},
	}

	resourcetest.Run(t, ClusterParser, tests)
}
