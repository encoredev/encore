package cache

import (
	"context"

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

func (k *SetKeyspace[K, V]) With(opts ...WriteOption) *SetKeyspace[K, V] {
	return &SetKeyspace[K, V]{k.client.with(opts)}
}

func (s *SetKeyspace[K, V]) Add(ctx context.Context, key K, values ...V) (added int, err error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	vals := fnMap(values, func(v V) any { return v })
	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SAdd(ctx, k, vals...)
	})

	val, err := toErr2(res.Result())
	return int(val), err
}

func (s *SetKeyspace[K, V]) Remove(ctx context.Context, key K, values ...V) (removed int, err error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	vals := fnMap(values, func(v V) any { return v })
	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SRem(ctx, k, vals...)
	})

	val, err := toErr2(res.Result())
	return int(val), err
}

func (s *SetKeyspace[K, V]) PopOne(ctx context.Context, key K) (val V, err error) {
	k, err := s.key(key)
	if err != nil {
		return val, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.SPop(ctx, k)
	})

	rs, err := toErr2(res.Result())
	if err != nil {
		return val, err
	}
	return s.fromRedis(rs)
}

func (s *SetKeyspace[K, V]) Pop(ctx context.Context, key K, count int) ([]V, error) {
	k, err := s.key(key)
	if err != nil {
		return nil, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.StringSliceCmd {
		return c.SPopN(ctx, k, int64(count))
	})

	rs, err := toErr2(res.Result())
	if err != nil {
		return nil, err
	}
	return s.fromRedisMulti(rs)
}

func (s *SetKeyspace[K, V]) Contains(ctx context.Context, key K, val V) (bool, error) {
	k, err := s.key(key)
	if err != nil {
		return false, err
	}

	return toErr2(s.redis.SIsMember(ctx, k, val).Result())
}

func (s *SetKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	return toErr2(s.redis.SCard(ctx, k).Result())
}

func (s *SetKeyspace[K, V]) Items(ctx context.Context, key K) ([]V, error) {
	res, err := s.items(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res)
}

func (s *SetKeyspace[K, V]) ItemsMap(ctx context.Context, key K) (map[V]struct{}, error) {
	res, err := s.items(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.toMap(res)
}

func (s *SetKeyspace[K, V]) items(ctx context.Context, key K) ([]string, error) {
	k, err := s.key(key)
	if err != nil {
		return nil, err
	}

	return toErr2(s.redis.SMembers(ctx, k).Result())
}

func (s *SetKeyspace[K, V]) Diff(ctx context.Context, keys ...K) ([]V, error) {
	res, err := s.diff(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res)
}

func (s *SetKeyspace[K, V]) DiffMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	res, err := s.diff(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res)
}

func (s *SetKeyspace[K, V]) diff(ctx context.Context, keys []K) ([]string, error) {
	ks, err := s.keys(keys)
	if err != nil {
		return nil, err
	}

	return toErr2(s.redis.SDiff(ctx, ks...).Result())
}

func (s *SetKeyspace[K, V]) DiffStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	dst, err := s.key(destination)
	if err != nil {
		return 0, err
	}

	ks, err := s.keys(keys)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SDiffStore(ctx, dst, ks...)
	})
	return toErr2(res.Result())
}

func (s *SetKeyspace[K, V]) Intersect(ctx context.Context, keys ...K) ([]V, error) {
	res, err := s.intersect(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res)
}

func (s *SetKeyspace[K, V]) IntersectMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	res, err := s.intersect(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res)
}

func (s *SetKeyspace[K, V]) intersect(ctx context.Context, keys []K) ([]string, error) {
	ks, err := s.keys(keys)
	if err != nil {
		return nil, err
	}

	return toErr2(s.redis.SInter(ctx, ks...).Result())
}

func (s *SetKeyspace[K, V]) IntersectStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	dst, err := s.key(destination)
	if err != nil {
		return 0, err
	}
	ks, err := s.keys(keys)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SInterStore(ctx, dst, ks...)
	})
	return toErr2(res.Result())
}

func (s *SetKeyspace[K, V]) Union(ctx context.Context, keys ...K) ([]V, error) {
	res, err := s.union(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toSlice(res)
}

func (s *SetKeyspace[K, V]) UnionMap(ctx context.Context, keys ...K) (map[V]struct{}, error) {
	res, err := s.union(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.toMap(res)
}

func (s *SetKeyspace[K, V]) union(ctx context.Context, keys []K) ([]string, error) {
	ks, err := s.keys(keys)
	if err != nil {
		return nil, err
	}

	return toErr2(s.redis.SUnion(ctx, ks...).Result())
}

func (s *SetKeyspace[K, V]) UnionStore(ctx context.Context, destination K, keys ...K) (size int64, err error) {
	dst, err := s.key(destination)
	if err != nil {
		return 0, err
	}
	ks, err := s.keys(keys)
	if err != nil {
		return 0, err
	}
	res := do(s.client, ctx, dst, func(c cmdable) *redis.IntCmd {
		return c.SUnionStore(ctx, dst, ks...)
	})
	return toErr2(res.Result())
}

func (s *SetKeyspace[K, V]) SampleOne(ctx context.Context, key K) (val V, err error) {
	k, err := s.key(key)
	if err != nil {
		return val, err
	}
	res, err := toErr2(s.redis.SRandMember(ctx, k).Result())
	if err != nil {
		return val, err
	}
	return s.fromRedis(res)
}

func (s *SetKeyspace[K, V]) Sample(ctx context.Context, key K, count int) ([]V, error) {
	if count < 0 {
		panic("Sample: negative count")
	} else if count == 0 {
		return nil, nil
	}

	k, err := s.key(key)
	if err != nil {
		return nil, err
	}

	res, err := toErr2(s.redis.SRandMemberN(ctx, k, int64(count)).Result())
	if err != nil {
		return nil, err
	}
	return s.fromRedisMulti(res)
}

func (s *SetKeyspace[K, V]) SampleWithReplacement(ctx context.Context, key K, count int) ([]V, error) {
	if count < 0 {
		panic("SampleWithReplacement: negative count")
	} else if count == 0 {
		return nil, nil
	}

	k, err := s.key(key)
	if err != nil {
		return nil, err
	}
	res, err := toErr2(s.redis.SRandMemberN(ctx, k, -int64(count)).Result())
	if err != nil {
		return nil, err
	}
	return s.fromRedisMulti(res)
}

func (s *SetKeyspace[K, V]) Move(ctx context.Context, src, dst K, val V) (moved bool, err error) {
	srcKey, err := s.key(src)
	if err != nil {
		return false, err
	}
	dstKey, err := s.key(dst)
	if err != nil {
		return false, err
	}

	res := do2(s.client, ctx, srcKey, dstKey, func(c cmdable) *redis.BoolCmd {
		return c.SMove(ctx, srcKey, dstKey, val)
	})
	return toErr2(res.Result())
}

func (s *SetKeyspace[K, V]) toSlice(res []string) ([]V, error) {
	ret := make([]V, len(res))
	for i, r := range res {
		val, err := s.fromRedis(r)
		if err != nil {
			return nil, err
		}
		ret[i] = val
	}
	return ret, nil
}

func (s *SetKeyspace[K, V]) toMap(res []string) (map[V]struct{}, error) {
	ret := make(map[V]struct{}, len(res))
	for _, r := range res {
		val, err := s.fromRedis(r)
		if err != nil {
			return nil, err
		}
		ret[val] = struct{}{}
	}
	return ret, nil
}
