package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// TODO:
// Multi-get, multi-set

// NewStringKeyspace creates a keyspace that stores string values in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
func NewStringKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *StringKeyspace[K] {
	return &StringKeyspace[K]{
		&basicKeyspace[K, string]{
			newClient[K, string](cluster, cfg),
		},
	}
}

// StringKeyspace represents a set of cache keys that hold string values.
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

// TODO explore Set options:
// - Expiry duration
// - Expiry deadline
// - Keep TTL

func (s *basicKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	exp := s.expiry()
	return toErr(s.redis.Set(ctx, s.key(key), val, exp).Err())
}

func (s *basicKeyspace[K, V]) SetIfNotExist(ctx context.Context, key K, val V) error {
	exp := s.expiry()
	_, err := toErr2(s.redis.SetNX(ctx, s.key(key), val, exp).Result())
	return err
}

func (s *basicKeyspace[K, V]) Update(ctx context.Context, key K, val V) error {
	exp := s.expiry()
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

func (s *basicKeyspace[K, V]) expiry() time.Duration {
	now := time.Now()
	expiry := s.defaultExpiry(now)

	// The redis library treats the zero duration as "never expire",
	// whereas we have a dedicated sentinel value for that.
	// Translate this to durations.
	if expiry == NeverExpire {
		return 0
	}
	dur := expiry.Sub(now)
	if dur == 0 {
		dur = time.Millisecond // expire immediately
	}
	return dur
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

// Key Expiry Rules:
// 1. If a DefaultExpiry is used, all mutations cause the TTL to be updated
// 2. For operations that support it this happens with a single command,
// otherwise it performs a pipelined mutate + expire.
// 3. To avoid this use NeverExpire or KeepTTL
