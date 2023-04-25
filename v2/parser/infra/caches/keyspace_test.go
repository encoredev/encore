package caches

import (
	"testing"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schematest"
	"encr.dev/v2/parser/resource/resourcetest"
)

func TestParseKeyspace(t *testing.T) {
	tests := []resourcetest.Case[*Keyspace]{
		{
			Name: "basic",
			Code: `
var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

// Keyspace docs
var x = cache.NewStringKeyspace[string](cluster, cache.KeyspaceConfig{
	KeyPattern: ":key",
})
`,
			Want: &Keyspace{
				Doc:       "Keyspace docs\n",
				KeyType:   schematest.String(),
				ValueType: schematest.String(),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Param, Value: "key", ValueType: schema.String},
					},
				},
			},
		},
		{
			Name: "int",
			Code: `
var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

var x = cache.NewIntKeyspace[int64](cluster, cache.KeyspaceConfig{
		KeyPattern: "int",
	})
`,
			Want: &Keyspace{
				KeyType:   schematest.Builtin(schema.Int64),
				ValueType: schematest.Builtin(schema.Int64),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Literal, Value: "int", ValueType: schema.String},
					},
				},
			},
		}, {
			Name: "float",
			Code: `
var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

var x = cache.NewFloatKeyspace[string](cluster, cache.KeyspaceConfig{
	KeyPattern: "float",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Builtin(schema.Float64),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Literal, Value: "float", ValueType: schema.String},
					},
				},
			},
		},
		{
			Name: "list",
			Code: `
var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

var x = cache.NewListKeyspace[string, bool](cluster, cache.KeyspaceConfig{
	KeyPattern: "list",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Bool(),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Literal, Value: "list", ValueType: schema.String},
					},
				},
			},
		},
		{
			Name: "set",
			Code: `
var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

var x = cache.NewSetKeyspace[string, bool](cluster, cache.KeyspaceConfig{
	KeyPattern: "set",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Bool(),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Literal, Value: "set", ValueType: schema.String},
					},
				},
			},
		},
		{
			Name: "struct",
			Code: `
type Foo struct {
	Bar string
}

var cluster = cache.NewCluster("cluster", cache.ClusterConfig{})

var x = cache.NewStructKeyspace[string, Foo](cluster, cache.KeyspaceConfig{
	KeyPattern: "struct",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Named(schematest.TypeInfo("Foo")),
				Cluster:   pkginfo.Q("example.com", "cluster"),
				Path: &resourcepaths.Path{
					Segments: []resourcepaths.Segment{
						{Type: resourcepaths.Literal, Value: "struct", ValueType: schema.String},
					},
				},
			},
		},
	}

	resourcetest.Run(t, KeyspaceParser, tests)
}
