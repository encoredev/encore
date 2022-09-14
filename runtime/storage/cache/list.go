package cache

import (
	"context"
	"errors"
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
	const op = "push left"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	// Convert []V to []any since that's what the Redis library expects.
	vals := fnMap(values, func(v V) any { return v })
	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LPush(ctx, k, vals...)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

func (l *ListKeyspace[K, V]) PushRight(ctx context.Context, key K, values ...V) (newLen int64, err error) {
	const op = "push right"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	// Convert []V to []any since that's what the Redis library expects.
	vals := fnMap(values, func(v V) any { return v })
	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.RPush(ctx, k, vals...)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

func (l *ListKeyspace[K, V]) PopLeft(ctx context.Context, key K) (val V, err error) {
	const op = "pop left"
	k, err := l.key(key, op)
	if err != nil {
		return val, err
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.LPop(ctx, k)
	}).Result()

	if err == nil {
		val, err = l.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

func (l *ListKeyspace[K, V]) PopRight(ctx context.Context, key K) (val V, err error) {
	const op = "pop right"
	k, err := l.key(key, op)
	if err != nil {
		return val, err
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.StringCmd {
		return c.RPop(ctx, k)
	}).Result()
	if err == nil {
		val, err = l.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

func (l *ListKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	const op = "list len"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	res, err := l.redis.LLen(ctx, k).Result()
	err = toErr(err, op, k)
	return res, err
}

func (l *ListKeyspace[K, V]) Trim(ctx context.Context, key K, start, stop int64) error {
	const op = "list trim"
	k, err := l.key(key, op)
	if err != nil {
		return err
	}
	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LTrim(ctx, k, start, stop)
	})
	return toErr(res.Err(), op, k)
}

func (l *ListKeyspace[K, V]) Set(ctx context.Context, key K, idx int64, val V) error {
	const op = "list set"
	k, err := l.key(key, op)
	if err != nil {
		return err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LSet(ctx, k, idx, val)
	})
	return toErr(res.Err(), op, k)
}

func (l *ListKeyspace[K, V]) Get(ctx context.Context, key K, idx int64) (val V, err error) {
	const op = "list get"
	k, err := l.key(key, op)
	if err != nil {
		return val, err
	}
	res, err := l.redis.LIndex(ctx, k, idx).Result()

	if err == nil {
		val, err = l.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

func (l *ListKeyspace[K, V]) Items(ctx context.Context, key K) ([]V, error) {
	return l.slice(ctx, key, 0, -1, "items")
}

func (l *ListKeyspace[K, V]) Slice(ctx context.Context, key K, from, to int64) ([]V, error) {
	return l.slice(ctx, key, from, to, "slice")
}

func (l *ListKeyspace[K, V]) slice(ctx context.Context, key K, from, to int64, op string) ([]V, error) {
	k, err := l.key(key, op)
	if err != nil {
		return nil, err
	}
	res, err := l.redis.LRange(ctx, k, from, to).Result()
	if err != nil {
		return nil, toErr(err, op, k)
	}

	ret := make([]V, len(res))
	for i, s := range res {
		val, err := l.fromRedis(s)
		if err != nil {
			return nil, toErr(err, op, k)
		}
		ret[i] = val
	}
	return ret, nil
}

func (l *ListKeyspace[K, V]) InsertBefore(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	const op = "insert before"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	newLen, err = do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LInsertBefore(ctx, k, needle, newVal)
	}).Result()

	if err == nil && newLen == -1 {
		return 0, toErr(Miss, op, k)
	}
	return newLen, toErr(err, op, k)
}

func (l *ListKeyspace[K, V]) InsertAfter(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	const op = "insert after"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	newLen, err = do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LInsertAfter(ctx, k, needle, newVal)
	}).Result()

	if err == nil && newLen == -1 {
		return 0, toErr(Miss, op, k)
	}
	return newLen, toErr(err, op, k)
}

func (l *ListKeyspace[K, V]) RemoveAll(ctx context.Context, key K, needle V) (removed int64, err error) {
	const op = "remove all"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, 0, needle)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

func (l *ListKeyspace[K, V]) RemoveFirst(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	const op = "remove first"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	if count < 0 {
		err = toErr(errors.New("negative count"), op, k)
		return 0, err
	} else if count == 0 {
		return 0, nil
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, count, needle)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

func (l *ListKeyspace[K, V]) RemoveLast(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	const op = "remove last"
	k, err := l.key(key, op)
	if err != nil {
		return 0, err
	}

	if count < 0 {
		err = toErr(errors.New("negative count"), op, k)
		return 0, err
	} else if count == 0 {
		return 0, nil
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, -count, needle)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

type ListPos string

const (
	Left  ListPos = "LEFT"
	Right ListPos = "RIGHT"
)

func (l *ListKeyspace[K, V]) Move(ctx context.Context, src, dst K, fromPos, toPos ListPos) (moved V, err error) {
	const op = "list move"
	srcKey, err := l.key(src, op)
	if err != nil {
		return moved, err
	}
	dstKey, err := l.key(dst, op)
	if err != nil {
		return moved, err
	}

	res, err := do2(l.client, ctx, srcKey, dstKey, func(c cmdable) *redis.StringCmd {
		return c.LMove(ctx, srcKey, dstKey, string(fromPos), string(toPos))
	}).Result()

	if err == nil {
		moved, err = l.fromRedis(res)
	}
	err = toErr(err, op, srcKey)
	return moved, err
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
