package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// TODO:
// Multi-get, multi-set

func NewStringKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *StringKeyspace[K] {
	return &StringKeyspace[K]{
		&basicKeyspace[K, string]{
			newClient[K, string](cluster, cfg),
		},
	}
}

type StringKeyspace[K any] struct {
	*basicKeyspace[K, string]
}

func (s *StringKeyspace[K]) Append(ctx context.Context, key K, val string) (newLen int64, err error) {
	return toErr2(s.client.redis.Append(ctx, s.key(key), val).Result())
}

func (s *StringKeyspace[K]) GetRange(ctx context.Context, key K, from, to int64) (string, error) {
	return toErr2(s.client.redis.GetRange(ctx, s.key(key), from, to).Result())
}

func (s *StringKeyspace[K]) SetRange(ctx context.Context, key K, offset int64, val string) (newLen int64, err error) {
	return toErr2(s.client.redis.SetRange(ctx, s.key(key), offset, val).Result())
}

func (s *StringKeyspace[K]) Len(ctx context.Context, key K) (int64, error) {
	return toErr2(s.client.redis.StrLen(ctx, s.key(key)).Result())
}

func NewIntKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *IntKeyspace[K] {
	return &IntKeyspace[K]{
		&basicKeyspace[K, int64]{
			newClient[K, int64](cluster, cfg),
		},
	}
}

type IntKeyspace[K any] struct {
	*basicKeyspace[K, int64]
}

func (s *IntKeyspace[K]) Incr(ctx context.Context, key K, delta int64) (int64, error) {
	return toErr2(s.client.redis.IncrBy(ctx, s.key(key), delta).Result())
}

func (s *IntKeyspace[K]) Decr(ctx context.Context, key K, delta int64) (int64, error) {
	return toErr2(s.client.redis.DecrBy(ctx, s.key(key), delta).Result())
}

func NewFloatKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *FloatKeyspace[K] {
	return &FloatKeyspace[K]{
		&basicKeyspace[K, float64]{
			newClient[K, float64](cluster, cfg),
		},
	}
}

type FloatKeyspace[K any] struct {
	*basicKeyspace[K, float64]
}

func (s *FloatKeyspace[K]) Incr(ctx context.Context, key K, delta float64) (float64, error) {
	return toErr2(s.client.redis.IncrByFloat(ctx, s.key(key), delta).Result())
}

func (s *FloatKeyspace[K]) Decr(ctx context.Context, key K, delta float64) (float64, error) {
	return toErr2(s.client.redis.IncrByFloat(ctx, s.key(key), -delta).Result())
}

type basicKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

func (s *basicKeyspace[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	res, err := toErr2(s.redis.Get(ctx, s.key(key)).Result())
	if err != nil {
		return val, err
	}
	return s.val(res)
}

func (s *basicKeyspace[K, V]) Add(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	_, err := toErr2(s.redis.SetNX(ctx, s.key(key), val, exp).Result())
	return err
}

// TODO explore Set options:
// - Expiry duration
// - Expiry deadline
// - Keep TTL

func (s *basicKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	return toErr(s.redis.Set(ctx, s.key(key), val, exp).Err())
}

func (s *basicKeyspace[K, V]) Update(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	return toErr(s.redis.SetXX(ctx, s.key(key), val, exp).Err())
}

func (s *basicKeyspace[K, V]) GetAndSet(ctx context.Context, key K, val V) (prev *V, err error) {
	res, err := s.redis.GetSet(ctx, s.key(key), val).Result()
	return s.valOrNil(res, err)
}

func (s *basicKeyspace[K, V]) GetAndDelete(ctx context.Context, key K) (val *V, err error) {
	res, err := toErr2(s.redis.GetDel(ctx, s.key(key)).Result())
	return s.valOrNil(res, err)
}

func (s *basicKeyspace[K, V]) Delete(ctx context.Context, key K) error {
	return toErr(s.redis.Del(ctx, s.key(key)).Err())
}

func toErr(err error) error {
	if err == redis.Nil {
		err = Nil
	}
	return err
}

func toErr2[T any](val T, err error) (T, error) {
	if err == redis.Nil {
		err = Nil
	}
	return val, err
}
