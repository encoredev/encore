package cache

import (
	"testing"

	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schematest"
	"encr.dev/v2/parser/infra/resource/resourcetest"
)

func TestParseKeyspace(t *testing.T) {
	// TODO(andre) Add cache cluster references
	tests := []resourcetest.Case[*Keyspace]{
		{
			Name: "basic",
			Code: `
// Keyspace docs
var x = cache.NewStringKeyspace[string](cluster, cache.KeyspaceConfig{
	KeyPattern: ":key",
})
`,
			Want: &Keyspace{
				Doc:       "Keyspace docs\n",
				KeyType:   schematest.String(),
				ValueType: schematest.String(),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Param, Value: "key"},
					},
				},
			},
		},
		{
			Name: "int",
			Code: `
var x = cache.NewIntKeyspace[int64](cluster, cache.KeyspaceConfig{
		KeyPattern: "int",
	})
`,
			Want: &Keyspace{
				KeyType:   schematest.Builtin(schema.Int64),
				ValueType: schematest.Builtin(schema.Int64),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Literal, Value: "int"},
					},
				},
			},
		}, {
			Name: "float",
			Code: `
var x = cache.NewFloatKeyspace[string](cluster, cache.KeyspaceConfig{
	KeyPattern: "float",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Builtin(schema.Float64),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Literal, Value: "float"},
					},
				},
			},
		},
		{
			Name: "list",
			Code: `
var x = cache.NewListKeyspace[string, bool](cluster, cache.KeyspaceConfig{
	KeyPattern: "list",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Bool(),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Literal, Value: "list"},
					},
				},
			},
		},
		{
			Name: "set",
			Code: `
var x = cache.NewSetKeyspace[string, bool](cluster, cache.KeyspaceConfig{
	KeyPattern: "set",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Bool(),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Literal, Value: "set"},
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
var x = cache.NewStructKeyspace[string, Foo](cluster, cache.KeyspaceConfig{
	KeyPattern: "struct",
})
`,
			Want: &Keyspace{
				KeyType:   schematest.String(),
				ValueType: schematest.Named(schematest.TypeInfo("Foo")),
				Path: &KeyspacePath{
					Segments: []Segment{
						{Type: Literal, Value: "struct"},
					},
				},
			},
		},
	}

	resourcetest.Run(t, KeyspaceParser, tests)
}
