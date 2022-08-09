package cache

import (
	"context"
)

// TODO:
// Multi-get, multi-set

func NewStringKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *StringKeyspace[K] {
	return nil
}

type StringKeyspace[K any] struct {
	impl *basicKeyspace[K, string]
}

func (s *StringKeyspace[K]) Get(ctx context.Context, key K) (string, error) {
	return s.impl.Get(ctx, key)
}

func (s *StringKeyspace[K]) Set(ctx context.Context, key K, val string) error {
	return s.impl.Set(ctx, key, val)
}

// Add adds a key if it does not already exist.
func (s *StringKeyspace[K]) Add(ctx context.Context, key K, val string) error {
	return s.impl.Add(ctx, key, val)
}

func (s *StringKeyspace[K]) Update(ctx context.Context, key K, val string) error {
	return s.impl.Update(ctx, key, val)
}

func (s *StringKeyspace[K]) Delete(ctx context.Context, key K) error {
	return s.impl.Delete(ctx, key)
}

func (s *StringKeyspace[K]) GetAndSet(ctx context.Context, key K, val string) (*string, error) {
	return s.impl.GetAndSet(ctx, key, val)
}

func (s *StringKeyspace[K]) GetAndDelete(ctx context.Context, key K) (*string, error) {
	return s.impl.GetAndDelete(ctx, key)
}

func (s *StringKeyspace[K]) Append(ctx context.Context, key K, val string) (newLen int64, err error) {
	return s.impl.redis.Append(ctx, s.impl.key(key), val).Result()
}

func (s *StringKeyspace[K]) GetRange(ctx context.Context, key K, from, to int64) (string, error) {
	return s.impl.redis.GetRange(ctx, s.impl.key(key), from, to).Result()
}

func (s *StringKeyspace[K]) SetRange(ctx context.Context, key K, offset int64, val string) (newLen int64, err error) {
	return s.impl.redis.SetRange(ctx, s.impl.key(key), offset, val).Result()
}

func (s *StringKeyspace[K]) Len(ctx context.Context, key K) (int64, error) {
	return s.impl.redis.StrLen(ctx, s.impl.key(key)).Result()
}

func NewIntKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *IntKeyspace[K] {
	return nil
}

type IntKeyspace[K any] struct {
	impl *basicKeyspace[K, int64]
}

func (s *IntKeyspace[K]) Get(ctx context.Context, key K) (int64, error) {
	return s.impl.Get(ctx, key)
}

func (s *IntKeyspace[K]) Set(ctx context.Context, key K, val int64) error {
	return s.impl.Set(ctx, key, val)
}

// Add adds a key if it does not already exist.
func (s *IntKeyspace[K]) Add(ctx context.Context, key K, val int64) error {
	return s.impl.Add(ctx, key, val)
}

func (s *IntKeyspace[K]) Update(ctx context.Context, key K, val int64) error {
	return s.impl.Update(ctx, key, val)
}

func (s *IntKeyspace[K]) Delete(ctx context.Context, key K) error {
	return s.impl.Delete(ctx, key)
}

func (s *IntKeyspace[K]) GetAndSet(ctx context.Context, key K, val int64) (*int64, error) {
	return s.impl.GetAndSet(ctx, key, val)
}

func (s *IntKeyspace[K]) GetAndDelete(ctx context.Context, key K) (*int64, error) {
	return s.impl.GetAndDelete(ctx, key)
}

func (s *IntKeyspace[K]) Incr(ctx context.Context, key K, delta int64) (int64, error) {
	return s.impl.redis.IncrBy(ctx, s.impl.key(key), delta).Result()
}

func (s *IntKeyspace[K]) Decr(ctx context.Context, key K, delta int64) (int64, error) {
	return s.impl.redis.DecrBy(ctx, s.impl.key(key), delta).Result()
}

func NewFloatKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *FloatKeyspace[K] {
	return nil
}

type FloatKeyspace[K any] struct {
	impl *basicKeyspace[K, float64]
}

func (s *FloatKeyspace[K]) Get(ctx context.Context, key K) (float64, error) {
	return s.impl.Get(ctx, key)
}

func (s *FloatKeyspace[K]) Set(ctx context.Context, key K, val float64) error {
	return s.impl.Set(ctx, key, val)
}

// Add adds a key if it does not already exist.
func (s *FloatKeyspace[K]) Add(ctx context.Context, key K, val float64) error {
	return s.impl.Add(ctx, key, val)
}

func (s *FloatKeyspace[K]) Update(ctx context.Context, key K, val float64) error {
	return s.impl.Update(ctx, key, val)
}

func (s *FloatKeyspace[K]) Delete(ctx context.Context, key K) error {
	return s.impl.Delete(ctx, key)
}

func (s *FloatKeyspace[K]) GetAndSet(ctx context.Context, key K, val float64) (*float64, error) {
	return s.impl.GetAndSet(ctx, key, val)
}

func (s *FloatKeyspace[K]) GetAndDelete(ctx context.Context, key K) (*float64, error) {
	return s.impl.GetAndDelete(ctx, key)
}

func (s *FloatKeyspace[K]) Incr(ctx context.Context, key K, delta float64) (float64, error) {
	return s.impl.redis.IncrByFloat(ctx, s.impl.key(key), delta).Result()
}

func (s *FloatKeyspace[K]) Decr(ctx context.Context, key K, delta float64) (float64, error) {
	return s.impl.redis.IncrByFloat(ctx, s.impl.key(key), -delta).Result()
}

type basicKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

func (s *basicKeyspace[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	res, err := s.redis.Get(ctx, s.key(key)).Result()
	if err != nil {
		return val, err
	}
	return s.val(res)
}

func (s *basicKeyspace[K, V]) Add(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	_, err := s.redis.SetNX(ctx, s.key(key), val, exp).Result()
	return err
}

// TODO explore Set options:
// - Expiry duration
// - Expiry deadline
// - Keep TTL

func (s *basicKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	return s.redis.Set(ctx, s.key(key), val, exp).Err()
}

func (s *basicKeyspace[K, V]) Update(ctx context.Context, key K, val V) error {
	exp := s.cfg.DefaultExpiry
	return s.redis.SetXX(ctx, s.key(key), val, exp).Err()
}

func (s *basicKeyspace[K, V]) GetAndSet(ctx context.Context, key K, val V) (prev *V, err error) {
	res, err := s.redis.GetSet(ctx, s.key(key), val).Result()
	return s.valOrNil(res, err)
}

func (s *basicKeyspace[K, V]) GetAndDelete(ctx context.Context, key K) (val *V, err error) {
	res, err := s.redis.GetDel(ctx, s.key(key)).Result()
	return s.valOrNil(res, err)
}

func (s *basicKeyspace[K, V]) Delete(ctx context.Context, key K) error {
	return s.redis.Del(ctx, s.key(key)).Err()
}
