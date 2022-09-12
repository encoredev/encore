package cache

// NewStructKeyspace creates a keyspace that stores structs in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
//
// The value parameter V specifies the named struct type that should be stored.
func NewStructKeyspace[K, V any](cluster *Cluster, cfg KeyspaceConfig) *StructKeyspace[K, V] {
	return &StructKeyspace[K, V]{
		&basicKeyspace[K, V]{
			newClient[K, V](cluster, cfg),
		},
	}
}

// StructKeyspace represents a set of cache keys that hold struct values.
type StructKeyspace[K, V any] struct {
	*basicKeyspace[K, V]
}

func (k *StructKeyspace[K, V]) With(opts ...WriteOption) *StructKeyspace[K, V] {
	return &StructKeyspace[K, V]{k.basicKeyspace.with(opts)}
}
