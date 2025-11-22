package cache

import "context"

// NewStructKeyspace creates a keyspace that stores structs in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
//
// The value parameter V specifies the named struct type that should be stored.
func NewStructKeyspace[K, V any](cluster *Cluster, cfg KeyspaceConfig) *StructKeyspace[K, V] {
	json := cluster.mgr.json
	fromRedis := func(val string) (V, error) {
		var v V
		err := json.UnmarshalFromString(val, &v)
		return v, err
	}
	toRedis := func(val V) (any, error) {
		return json.MarshalToString(val)
	}

	return &StructKeyspace[K, V]{
		&basicKeyspace[K, V]{
			newClient[K, V](cluster, cfg, fromRedis, toRedis),
		},
	}
}

// StructKeyspace represents a set of cache keys that hold struct values.
type StructKeyspace[K, V any] struct {
	*basicKeyspace[K, V]
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//
//	myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *StructKeyspace[K, V]) With(opts ...WriteOption) *StructKeyspace[K, V] {
	return &StructKeyspace[K, V]{k.basicKeyspace.with(opts)}
}

// Get gets the value stored at key.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/get/ for more information.
func (s *StructKeyspace[K, V]) Get(ctx context.Context, key K) (V, error) {
	return s.basicKeyspace.Get(ctx, key)
}

// MultiGet gets the values stored at multiple keys.
// For each key, the result contains an Err field indicating success or failure.
// If Err is nil, Value contains the cached value.
// If Err matches Miss, the key was not found.
//
// See https://redis.io/commands/mget/ for more information.
func (s *StructKeyspace[K, V]) MultiGet(ctx context.Context, keys ...K) ([]Result[V], error) {
	return s.basicKeyspace.MultiGet(ctx, keys...)
}

// Set updates the value stored at key to val.
//
// See https://redis.io/commands/set/ for more information.
func (s *StructKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	return s.basicKeyspace.Set(ctx, key, val)
}

// SetIfNotExists sets the value stored at key to val, but only if the key does not exist beforehand.
// If the key already exists, it reports an error matching KeyExists.
//
// See https://redis.io/commands/setnx/ for more information.
func (s *StructKeyspace[K, V]) SetIfNotExists(ctx context.Context, key K, val V) error {
	return s.basicKeyspace.SetIfNotExists(ctx, key, val)
}

// Replace replaces the existing value stored at key to val.
// If the key does not already exist, it reports an error matching Miss.
//
// See https://redis.io/commands/set/ for more information.
func (s *StructKeyspace[K, V]) Replace(ctx context.Context, key K, val V) error {
	return s.basicKeyspace.Replace(ctx, key, val)
}

// GetAndSet updates the value of key to val and returns the previously stored value.
// If the key does not already exist, it sets it and returns 0, nil.
//
// See https://redis.io/commands/getset/ for more information.
func (s *StructKeyspace[K, V]) GetAndSet(ctx context.Context, key K, val V) (oldVal V, err error) {
	return s.basicKeyspace.GetAndSet(ctx, key, val)
}

// GetAndDelete deletes the key and returns the previously stored value.
// If the key does not already exist, it does nothing and returns the zero value of V and nil.
//
// See https://redis.io/commands/getdel/ for more information.
func (s *StructKeyspace[K, V]) GetAndDelete(ctx context.Context, key K) (oldVal V, err error) {
	return s.basicKeyspace.GetAndDelete(ctx, key)
}

// Delete deletes the specified keys.
//
// If a key does not exist it is ignored.
//
// It reports the number of keys that were deleted.
//
// See https://redis.io/commands/del/ for more information.
func (s *StructKeyspace[K, V]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	return s.client.Delete(ctx, keys...)
}
