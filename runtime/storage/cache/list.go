package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
)

func NewListKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *ListKeyspace[K, V] {
	fromRedis := basicFromRedisFactory[V]()
	toRedis := basicToRedisFactory[V]()

	return &ListKeyspace[K, V]{
		client: newClient[K, V](cluster, cfg, fromRedis, toRedis),
	}
}

type BasicType interface {
	string | int | int64 | float64
}

type ListKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

func (k *ListKeyspace[K, V]) With(opts ...WriteOption) *ListKeyspace[K, V] {
	return &ListKeyspace[K, V]{k.client.with(opts)}
}

func (l *ListKeyspace[K, V]) PushLeft(ctx context.Context, key K, values ...V) (newLen int64, err error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	// Convert []V to []any since that's what the Redis library expects.
	vals := fnMap(values, func(v V) any { return v })
	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LPush(ctx, k, vals...)
	})
	return toErr2(res.Result())
}

func (l *ListKeyspace[K, V]) PushRight(ctx context.Context, key K, values ...V) (newLen int64, err error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	// Convert []V to []any since that's what the Redis library expects.
	vals := fnMap(values, func(v V) any { return v })
	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.RPush(ctx, k, vals...)
	})
	return toErr2(res.Result())
}

func (l *ListKeyspace[K, V]) PopLeft(ctx context.Context, key K) (val V, err error) {
	k, err := l.key(key)
	if err != nil {
		return val, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.LPop(ctx, k)
	})
	s, err := toErr2(res.Result())
	if err != nil {
		return val, err
	}
	return l.fromRedis(s)
}

func (l *ListKeyspace[K, V]) PopRight(ctx context.Context, key K) (val V, err error) {
	k, err := l.key(key)
	if err != nil {
		return val, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.RPop(ctx, k)
	})
	s, err := toErr2(res.Result())
	if err != nil {
		return val, err
	}
	return l.fromRedis(s)
}

func (l *ListKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}
	return toErr2(l.redis.LLen(ctx, k).Result())
}

func (l *ListKeyspace[K, V]) Trim(ctx context.Context, key K, start, stop int64) error {
	k, err := l.key(key)
	if err != nil {
		return err
	}
	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LTrim(ctx, k, start, stop)
	})
	return toErr(res.Err())
}

func (l *ListKeyspace[K, V]) Set(ctx context.Context, key K, idx int64, val V) error {
	k, err := l.key(key)
	if err != nil {
		return err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LSet(ctx, k, idx, val)
	})
	return toErr(res.Err())
}

func (l *ListKeyspace[K, V]) Get(ctx context.Context, key K, idx int64) (val V, err error) {
	k, err := l.key(key)
	if err != nil {
		return val, err
	}
	res, err := toErr2(l.redis.LIndex(ctx, k, idx).Result())
	if err != nil {
		return val, err
	}
	return l.fromRedis(res)
}

func (l *ListKeyspace[K, V]) Items(ctx context.Context, key K) ([]V, error) {
	return l.Slice(ctx, key, 0, -1)
}

func (l *ListKeyspace[K, V]) Slice(ctx context.Context, key K, from, to int64) ([]V, error) {
	k, err := l.key(key)
	if err != nil {
		return nil, err
	}
	res, err := toErr2(l.redis.LRange(ctx, k, from, to).Result())
	if err != nil {
		return nil, err
	}

	ret := make([]V, len(res))
	for i, s := range res {
		val, err := l.fromRedis(s)
		if err != nil {
			return nil, err
		}
		ret[i] = val
	}
	return ret, nil
}

func (l *ListKeyspace[K, V]) InsertBefore(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LInsertBefore(ctx, k, needle, newVal)
	})

	newLen, err = toErr2(res.Result())
	if newLen == -1 {
		return 0, Nil
	}
	return newLen, err
}

func (l *ListKeyspace[K, V]) InsertAfter(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LInsertAfter(ctx, k, needle, newVal)
	})

	newLen, err = toErr2(res.Result())
	if newLen == -1 {
		return 0, Nil
	}
	return newLen, err
}

func (l *ListKeyspace[K, V]) RemoveAll(ctx context.Context, key K, needle V) (removed int64, err error) {
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, 0, needle)
	})
	return toErr2(res.Result())
}

func (l *ListKeyspace[K, V]) RemoveFirst(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	if count < 0 {
		panic("RemoveFirst: negative count")
	} else if count == 0 {
		return 0, nil
	}
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, count, needle)
	})
	return toErr2(res.Result())
}

func (l *ListKeyspace[K, V]) RemoveLast(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	if count < 0 {
		panic("RemoveFirst: negative count")
	} else if count == 0 {
		return 0, nil
	}
	k, err := l.key(key)
	if err != nil {
		return 0, err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, -count, needle)
	})
	return toErr2(res.Result())
}

type ListPos string

const (
	Left  ListPos = "LEFT"
	Right ListPos = "RIGHT"
)

func (l *ListKeyspace[K, V]) Move(ctx context.Context, src, dst K, fromPos, toPos ListPos) (moved V, err error) {
	srcKey, err := l.key(src)
	if err != nil {
		return moved, err
	}
	dstKey, err := l.key(dst)
	if err != nil {
		return moved, err
	}

	res := do2(l.client, ctx, srcKey, dstKey, func(c cmdable) *redis.StringCmd {
		return c.LMove(ctx, srcKey, dstKey, string(fromPos), string(toPos))
	})

	s, err := toErr2(res.Result())
	if err != nil {
		return moved, err
	}
	return l.fromRedis(s)
}

func basicFromRedisFactory[V BasicType]() func(val string) (V, error) {
	var zero V
	typ := any(zero)

	var fn any
	switch typ.(type) {
	case string:
		fn = func(val string) (string, error) {
			return val, nil
		}
	case int:
		fn = func(val string) (int, error) {
			res, err := strconv.ParseInt(val, 10, 64)
			return int(res), err
		}
	case int64:
		fn = func(val string) (int64, error) {
			return strconv.ParseInt(val, 10, 64)
		}
	case float64:
		fn = func(val string) (float64, error) {
			return strconv.ParseFloat(val, 64)
		}
	default:
		panic(fmt.Sprintf("unsupported BasicType %T", typ))
	}

	return fn.(func(val string) (V, error))
}

func basicToRedisFactory[V BasicType]() func(val V) (any, error) {
	return func(val V) (any, error) { return val, nil }
}
