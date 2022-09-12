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

func (k *StringKeyspace[K]) With(opts ...WriteOption) *StringKeyspace[K] {
	return &StringKeyspace[K]{k.basicKeyspace.with(opts)}
}

func (s *StringKeyspace[K]) Append(ctx context.Context, key K, val string) (newLen int64, err error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.Append(ctx, k, val)
	})
	return toErr2(res.Result())
}

func (s *StringKeyspace[K]) GetRange(ctx context.Context, key K, from, to int64) (string, error) {
	k, err := s.key(key)
	if err != nil {
		return "", err
	}

	return toErr2(s.client.redis.GetRange(ctx, k, from, to).Result())
}

func (s *StringKeyspace[K]) SetRange(ctx context.Context, key K, offset int64, val string) (newLen int64, err error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SetRange(ctx, k, offset, val)
	})
	return toErr2(res.Result())
}

func (s *StringKeyspace[K]) Len(ctx context.Context, key K) (int64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	return toErr2(s.client.redis.StrLen(ctx, k).Result())
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

func (k *IntKeyspace[K]) With(opts ...WriteOption) *IntKeyspace[K] {
	return &IntKeyspace[K]{k.basicKeyspace.with(opts)}
}

func (s *IntKeyspace[K]) Incr(ctx context.Context, key K, delta int64) (int64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.IncrBy(ctx, k, delta)
	})
	return toErr2(res.Result())
}

func (s *IntKeyspace[K]) Decr(ctx context.Context, key K, delta int64) (int64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.DecrBy(ctx, k, delta)
	})
	return toErr2(res.Result())
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

func (k *FloatKeyspace[K]) With(opts ...WriteOption) *FloatKeyspace[K] {
	return &FloatKeyspace[K]{k.basicKeyspace.with(opts)}
}

func (s *FloatKeyspace[K]) Incr(ctx context.Context, key K, delta float64) (float64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.FloatCmd {
		return c.IncrByFloat(ctx, k, delta)
	})
	return toErr2(res.Result())
}

func (s *FloatKeyspace[K]) Decr(ctx context.Context, key K, delta float64) (float64, error) {
	k, err := s.key(key)
	if err != nil {
		return 0, err
	}

	res := do(s.client, ctx, k, func(c cmdable) *redis.FloatCmd {
		return c.IncrByFloat(ctx, k, -delta)
	})
	return toErr2(res.Result())
}

type basicKeyspace[K, V any] struct {
	*client[K, V]
}

func (s *basicKeyspace[K, V]) with(opts []WriteOption) *basicKeyspace[K, V] {
	return &basicKeyspace[K, V]{s.client.with(opts)}
}

func (s *basicKeyspace[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	k, err := s.key(key)
	if err != nil {
		return val, err
	}

	res, err := toErr2(s.redis.Get(ctx, k).Result())
	if err != nil {
		return val, err
	}
	return s.val(res)
}

func (s *basicKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	_, err := s.set(ctx, key, val, 0)
	return err
}

func (s *basicKeyspace[K, V]) SetIfNotExists(ctx context.Context, key K, val V) error {
	_, err := s.set(ctx, key, val, setNX)
	return err
}

func (s *basicKeyspace[K, V]) Replace(ctx context.Context, key K, val V) error {
	_, err := s.set(ctx, key, val, setXX)
	return err
}

func (s *basicKeyspace[K, V]) GetAndSet(ctx context.Context, key K, val V) (prev *V, err error) {
	return s.valOrNil(s.set(ctx, key, val, setGet))
}

func (s *basicKeyspace[K, V]) GetAndDelete(ctx context.Context, key K) (val *V, err error) {
	k, err := s.key(key)
	if err != nil {
		return nil, err
	}

	// When deleting we don't need to deal with expiry
	res, err := toErr2(s.redis.GetDel(ctx, k).Result())
	return s.valOrNil(res, err)
}

func (s *basicKeyspace[K, V]) Delete(ctx context.Context, key K) error {
	k, err := s.key(key)
	if err != nil {
		return err
	}

	// When deleting we don't need to deal with expiry
	return toErr(s.redis.Del(ctx, k).Err())
}

type setFlag uint8

const (
	setGet setFlag = 1 << iota
	setNX
	setXX
)

func (s *basicKeyspace[K, V]) set(ctx context.Context, key K, val V, flag setFlag) (string, error) {
	k, err := s.key(key)
	if err != nil {
		return "", err
	}

	get := (flag & setGet) == setGet
	nx := (flag & setNX) == setNX
	xx := (flag & setXX) == setXX

	args := make([]any, 3, 7)
	args[0] = "set"
	args[1] = k
	args[2] = val
	if nx {
		args = append(args, "nx")
	} else if xx {
		args = append(args, "xx")
	}
	if get {
		args = append(args, "get")
	}

	now := time.Now()
	exp := s.expiry(now)
	switch exp {
	case neverExpire:
		// do nothing; default Redis behavior
	case keepTTL:
		args = append(args, "keepttl")
	default:
		dur := exp.Sub(now)
		if dur < 0 {
			// The expiry is in the past; use a very old unix timestamp to
			// delete the key immediately. Note that we can't use timestamp 0
			// or else [Mini]redis complains.
			args = append(args, "exat", 1)
		} else {
			if usePreciseDur(dur) {
				args = append(args, "px", int64(dur/time.Millisecond))
			} else {
				args = append(args, "ex", int64(dur/time.Second))
			}
		}
	}

	if get {
		cmd := redis.NewStringCmd(ctx, args...)
		_ = s.redis.Process(ctx, cmd)
		return toErr2(cmd.Result())
	}

	cmd := redis.NewStatusCmd(ctx, args...)
	_ = s.redis.Process(ctx, cmd)
	return "", toErr(cmd.Err())
}

func usePreciseDur(dur time.Duration) bool {
	return dur < time.Second || dur%time.Second != 0
}
