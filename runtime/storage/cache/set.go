package cache

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
)

// NewSetKeyspace creates a keyspace that stores unordered sets in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
//
// The type parameter V specifies the value type, which is the type
// of the elements in each set. It must be a basic type (string, int, int64, or float64).
func NewSetKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *SetKeyspace[K, V] {
	fromRedis := basicFromRedisFactory[V]()
	toRedis := basicToRedisFactory[V]()

	return &SetKeyspace[K, V]{
		newClient[K, V](cluster, cfg, fromRedis, toRedis),
	}
}

// SetKeyspace represents a set of cache keys,
// each containing an unordered set of values of type V.
type SetKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//		myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *SetKeyspace[K, V]) With(opts ...WriteOption) *SetKeyspace[K, V] {
	return &SetKeyspace[K, V]{k.client.with(opts)}
}

// Add adds one or more values to the set stored at key.
// If the key does not already exist, it is first created as an empty set.
//
// It reports the number of values that were added to the set,
// not including values already present beforehand.
//
// See https://redis.io/commands/sadd/ for more information.
func (s *SetKeyspace[K, V]) Add(ctx context.Context, key K, values ...V) (added int, err error) {
	const op = "set add"
	k, err := s.key(key, op)
	if err != nil {
		return 0, err
	}

	vals := fnMap(values, func(v V) any { return v })
	res, err := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SAdd(ctx, k, vals...)
	}).Result()

	err = toErr(err, op, k)
	return int(res), err
}

// Remove removes one or more values from the set stored at key.
//
// If a value is not present in the set is it ignored.
//
// Remove reports the number of values that were removed from the set.
// If the key does not already exist, it is a no-op and reports 0, nil.
//
// See https://redis.io/commands/srem/ for more information.
func (s *SetKeyspace[K, V]) Remove(ctx context.Context, key K, values ...V) (removed int, err error) {
	const op = "set remove"
	k, err := s.key(key, op)
	if err != nil {
		return 0, err
	}

	vals := fnMap(values, func(v V) any { return v })
	res, err := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SRem(ctx, k, vals...)
	}).Result()

	err = toErr(err, op, k)
	return int(res), err
}

// PopOne removes a random element from the set stored at key and returns it.
//
// If the set is empty it reports an error matching Miss.
//
// See https://redis.io/commands/spop/ for more information.
func (s *SetKeyspace[K, V]) PopOne(ctx context.Context, key K) (val V, err error) {
	const op = "set pop one"
	k, err := s.key(key, op)
	if err != nil {
		return val, err
	}

	res, err := do(s.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.SPop(ctx, k)
	}).Result()

	if err == nil {
		val, err = s.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

// Pop removes up to 'count' random elements (bounded by the set's size)
// from the set stored at key and returns them.
//
// If the set is empty it returns an empty slice and no error.
//
// See https://redis.io/commands/spop/ for more information.
func (s *SetKeyspace[K, V]) Pop(ctx context.Context, key K, count int) ([]V, error) {
	const op = "set pop"
	k, err := s.key(key, op)
	if err != nil {
		return nil, err
	}

	res, err := do(s.client, ctx, k, func(c cmdable) *redis.StringSliceCmd {
		return c.SPopN(ctx, k, int64(count))
	}).Result()

	var vals []V
	if err == nil {
		vals, err = s.fromRedisMulti(res)
	}
	err = toErr(err, op, k)
	return vals, err
}

// Contains reports whether the set stored at key contains the given value.
//
// If the key does not exist it reports false, nil.
//
// See https://redis.io/commands/sismember/ for more information.
func (s *SetKeyspace[K, V]) Contains(ctx context.Context, key K, val V) (bool, error) {
	const op = "set contains"
	k, err := s.key(key, op)
	if err != nil {
		return false, err
	}

	res, err := s.redis.SIsMember(ctx, k, val).Result()
	err = toErr(err, op, k)
	return res, err
}

// Len reports the number of elements in the set stored at key.
//
// If the key does not exist it reports 0, nil.
//
// See https://redis.io/commands/slen/ for more information.
func (s *SetKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	const op = "set len"
	k, err := s.key(key, op)
	if err != nil {
		return 0, err
	}

	res, err := s.redis.SCard(ctx, k).Result()
	err = toErr(err, op, k)
	return res, err
}

// Items returns the elements in the set stored at key.
//
// If the key does not exist it returns an empty slice and no error.
//
// See https://redis.io/commands/smembers/ for more information.
func (s *SetKeyspace[K, V]) Items(ctx context.Context, key K) ([]V, error) {
	const op = "set items"
	k, err := s.key(key, op)
	if err != nil {
		return nil, err
	}
	res, err := s.items(ctx, op, k)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, k)
}

// ItemsMap is identical to Items except it returns the values as a map.
//
// If the key does not exist it returns an empty (but non-nil) map and no error.
//
// See https://redis.io/commands/smembers/ for more information.
func (s *SetKeyspace[K, V]) ItemsMap(ctx context.Context, key K) (map[V]struct{}, error) {
	const op = "set items"
	k, err := s.key(key, op)
	if err != nil {
		return nil, err
	}

	res, err := s.items(ctx, op, k)
	if err != nil {
		return nil, err
	}
	return s.toMap(res, op, k)
}

func (s *SetKeyspace[K, V]) items(ctx context.Context, op, key string) ([]string, error) {
	res, err := s.redis.SMembers(ctx, key).Result()
	err = toErr(err, op, key)
	return res, err
}

// Diff computes the set difference, between the first set and all the consecutive sets.
//
// Set difference means the values present in the first set that are not present
// in any of the other sets.
//
// Keys that do not exist are considered as empty sets.
//
// At least one key must be provided: if no keys are provided an error is reported.
//
// See https://redis.io/commands/sdiff/ for more information.
func (s *SetKeyspace[K, V]) Diff(ctx context.Context, keys ...K) ([]V, error) {
	const op = "set diff"
	res, firstKey, err := s.diff(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

// DiffMap is identical to Diff except it returns the values as a map.
//
// See https://redis.io/commands/sdiff/ for more information.
func (s *SetKeyspace[K, V]) DiffMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	const op = "set diff"
	res, firstKey, err := s.diff(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res, op, firstKey)
}

func (s *SetKeyspace[K, V]) diff(ctx context.Context, op string, keys []K) (vals []string, firstKey string, err error) {
	ks, err := s.keys(keys, op)
	if err != nil {
		return nil, "", err
	}
	if len(ks) > 0 {
		firstKey = ks[0]
	}

	vals, err = s.redis.SDiff(ctx, ks...).Result()
	return vals, firstKey, toErr(err, op, firstKey)
}

// DiffStore computes the set difference between keys (like Diff) and stores the result in destination.
//
// It reports the size of the resulting set.
//
// See https://redis.io/commands/sdiffstore/ for more information.
func (s *SetKeyspace[K, V]) DiffStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	const op = "store set diff"
	dst, err := s.key(destination, op)
	if err != nil {
		return 0, err
	}

	ks, err := s.keys(keys, op)
	if err != nil {
		return 0, err
	}

	res, err := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SDiffStore(ctx, dst, ks...)
	}).Result()

	err = toErr(err, op, dst)
	return res, err
}

// Intersect computes the set intersection between the sets stored at the given keys.
//
// Set intersection means the values common to all the provided sets.
//
// Keys that do not exist are considered to be empty sets.
// As a result, if any key is missing the final result is the empty set.
//
// At least one key must be provided: if no keys are provided an error is reported.
//
// See https://redis.io/commands/sinter/ for more information.
func (s *SetKeyspace[K, V]) Intersect(ctx context.Context, keys ...K) ([]V, error) {
	const op = "intersect"
	res, firstKey, err := s.intersect(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

// IntersectMap is identical to Intersect except it returns the values as a map.
//
// See https://redis.io/commands/sinter/ for more information.
func (s *SetKeyspace[K, V]) IntersectMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	const op = "intersect"
	res, firstKey, err := s.intersect(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res, op, firstKey)
}

func (s *SetKeyspace[K, V]) intersect(ctx context.Context, op string, keys []K) (vals []string, firstKey string, err error) {
	ks, err := s.keys(keys, op)
	if err != nil {
		return nil, "", err
	}
	if len(ks) > 0 {
		firstKey = ks[0]
	}

	res, err := s.redis.SInter(ctx, ks...).Result()
	err = toErr(err, op, firstKey)
	return res, firstKey, err
}

// IntersectStore computes the set intersection between keys (like Intersect) and stores the result in destination.
//
// It reports the size of the resulting set.
//
// See https://redis.io/commands/sinterstore/ for more information.
func (s *SetKeyspace[K, V]) IntersectStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	const op = "store set intersect"
	dst, err := s.key(destination, op)
	if err != nil {
		return 0, err
	}
	ks, err := s.keys(keys, op)
	if err != nil {
		return 0, err
	}

	res, err := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SInterStore(ctx, dst, ks...)
	}).Result()
	err = toErr(err, op, dst)
	return res, err
}

// Union computes the set union between the sets stored at the given keys.
//
// Set union means the values present in at least one of the provided sets.
//
// Keys that do not exist are considered to be empty sets.
//
// At least one key must be provided: if no keys are provided an error is reported.
//
// See https://redis.io/commands/sunion/ for more information.
func (s *SetKeyspace[K, V]) Union(ctx context.Context, keys ...K) ([]V, error) {
	const op = "union"
	res, firstKey, err := s.union(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

// UnionMap is identical to Union except it returns the values as a map.
//
// See https://redis.io/commands/sunion/ for more information.
func (s *SetKeyspace[K, V]) UnionMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	const op = "union"
	res, firstKey, err := s.union(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res, op, firstKey)
}

func (s *SetKeyspace[K, V]) union(ctx context.Context, op string, keys []K) (vals []string, firstKey string, err error) {
	ks, err := s.keys(keys, op)
	if err != nil {
		return nil, "", err
	}

	if len(ks) > 0 {
		firstKey = ks[0]
	}

	res, err := s.redis.SUnion(ctx, ks...).Result()
	err = toErr(err, op, firstKey)
	return res, firstKey, err
}

// UnionStore computes the set union between sets (like Union) and stores the result in destination.
//
// It reports the size of the resulting set.
//
// See https://redis.io/commands/sunionstore/ for more information.
func (s *SetKeyspace[K, V]) UnionStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	const op = "store set union"
	dst, err := s.key(destination, op)
	if err != nil {
		return 0, err
	}
	ks, err := s.keys(keys, op)
	if err != nil {
		return 0, err
	}
	res, err := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SUnionStore(ctx, dst, ks...)
	}).Result()
	err = toErr(err, op, dst)
	return res, err
}

// SampleOne returns a random member from the set stored at key.
//
// If the key does not exist it reports an error matching Miss.
//
// See https://redis.io/commands/srandmember/ for more information.
func (s *SetKeyspace[K, V]) SampleOne(ctx context.Context, key K) (val V, err error) {
	const op = "set sample one"
	k, err := s.key(key, op)
	if err != nil {
		return val, err
	}

	res, err := s.redis.SRandMember(ctx, k).Result()
	if err == nil {
		val, err = s.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

// Sample returns up to 'count' distinct random elements from the set stored at key.
// The same element is never returned multiple times.
//
// If the key does not exist it returns an empty slice and no error.
//
// See https://redis.io/commands/srandmember/ for more information.
func (s *SetKeyspace[K, V]) Sample(ctx context.Context, key K, count int) ([]V, error) {
	const op = "set sample one"
	k, err := s.key(key, op)
	if err != nil {
		return nil, err
	}

	if count < 0 {
		err = toErr(errors.New("negative count"), op, k)
		return nil, err
	} else if count == 0 {
		return nil, nil
	}

	var vals []V
	res, err := s.redis.SRandMemberN(ctx, k, int64(count)).Result()
	if err == nil {
		vals, err = s.fromRedisMulti(res)
	}
	err = toErr(err, op, k)
	return vals, err
}

// SampleWithReplacement returns count random elements from the set stored at key.
// The same element may be returned multiple times.
//
// If the key does not exist it returns an empty slice and no error.
//
// See https://redis.io/commands/srandmember/ for more information.
func (s *SetKeyspace[K, V]) SampleWithReplacement(ctx context.Context, key K, count int) ([]V, error) {
	const op = "set sample with replacement"
	k, err := s.key(key, op)
	if err != nil {
		return nil, err
	}

	if count < 0 {
		err = toErr(errors.New("negative count"), op, k)
		return nil, err
	} else if count == 0 {
		return nil, nil
	}

	var vals []V
	res, err := s.redis.SRandMemberN(ctx, k, -int64(count)).Result()
	if err == nil {
		vals, err = s.fromRedisMulti(res)
	}
	err = toErr(err, op, k)
	return vals, err
}

// Move atomically moves the given value from the set stored at src
// to the set stored at dst.
//
// If the element does not exist in src it reports false, nil.
//
// If the element already exists in dst it is still removed from src
// and Move still reports true, nil.
func (s *SetKeyspace[K, V]) Move(ctx context.Context, src, dst K, val V) (moved bool, err error) {
	const op = "move"
	srcKey, err := s.key(src, op)
	if err != nil {
		return false, err
	}
	dstKey, err := s.key(dst, op)
	if err != nil {
		return false, err
	}

	res, err := do2(s.client, ctx, srcKey, dstKey, func(c cmdable) *redis.BoolCmd {
		return c.SMove(ctx, srcKey, dstKey, val)
	}).Result()
	return res, toErr(err, op, srcKey)
}

func (s *SetKeyspace[K, V]) toSlice(res []string, op, key string) ([]V, error) {
	ret := make([]V, len(res))
	for i, r := range res {
		val, err := s.fromRedis(r)
		if err != nil {
			return nil, toErr(err, op, key)
		}
		ret[i] = val
	}
	return ret, nil
}

func (s *SetKeyspace[K, V]) toMap(res []string, op, key string) (map[V]struct{}, error) {
	ret := make(map[V]struct{}, len(res))
	for _, r := range res {
		val, err := s.fromRedis(r)
		if err != nil {
			return nil, toErr(err, op, key)
		}
		ret[val] = struct{}{}
	}
	return ret, nil
}
