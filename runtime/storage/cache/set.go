package cache

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
)

func NewSetKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *SetKeyspace[K, V] {
	fromRedis := basicFromRedisFactory[V]()
	toRedis := basicToRedisFactory[V]()

	return &SetKeyspace[K, V]{
		newClient[K, V](cluster, cfg, fromRedis, toRedis),
	}
}

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

func (s *SetKeyspace[K, V]) Diff(ctx context.Context, keys ...K) ([]V, error) {
	const op = "set diff"
	res, firstKey, err := s.diff(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

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

func (s *SetKeyspace[K, V]) Intersect(ctx context.Context, keys ...K) ([]V, error) {
	const op = "intersect"
	res, firstKey, err := s.intersect(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

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

func (s *SetKeyspace[K, V]) Union(ctx context.Context, keys ...K) ([]V, error) {
	const op = "union"
	res, firstKey, err := s.union(ctx, op, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res, op, firstKey)
}

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
