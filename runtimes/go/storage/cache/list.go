package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
)

// BasicType is a constraint for basic types that
// can be used as element types in Redis lists and sets.
type BasicType interface {
	string | int | int64 | float64
}

// NewListKeyspace creates a keyspace that stores ordered lists in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
//
// The type parameter V specifies the value type, which is the type
// of the elements in each list. It must be a basic type (string, int, int64, or float64).
func NewListKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *ListKeyspace[K, V] {
	fromRedis := basicFromRedisFactory[V]()
	toRedis := basicToRedisFactory[V]()

	return &ListKeyspace[K, V]{
		client: newClient[K, V](cluster, cfg, fromRedis, toRedis),
	}
}

// ListKeyspace represents a set of cache keys,
// each containing an ordered list of values of type V.
type ListKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//
//	myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *ListKeyspace[K, V]) With(opts ...WriteOption) *ListKeyspace[K, V] {
	return &ListKeyspace[K, V]{k.client.with(opts)}
}

// Delete deletes the specified keys.
//
// If a key does not exist it is ignored.
//
// It reports the number of keys that were deleted.
//
// See https://redis.io/commands/del/ for more information.
func (s *ListKeyspace[K, V]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	return s.client.Delete(ctx, keys...)
}

// PushLeft pushes one or more values at the head of the list stored at key.
// If the key does not already exist, it is first created as an empty list.
//
// If multiple values are given, they are inserted one after another,
// starting with the leftmost value. For instance,
//
//	PushLeft(ctx, "mylist", "a", "b", "c")
//
// will result in a list containing "c" as its first element,
// "b" as its second element, and "a" as its third element.
//
// See https://redis.io/commands/lpush/ for more information.
func (l *ListKeyspace[K, V]) PushLeft(ctx context.Context, key K, values ...V) (newLen int64, err error) {
	const op = "push left"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// PushRight pushes one or more values at the tail of the list stored at key.
// If the key does not already exist, it is first created as an empty list.
//
// If multiple values are given, they are inserted one after another,
// starting with the leftmost value. For instance,
//
//	PushRight(ctx, "mylist", "a", "b", "c")
//
// will result in a list containing "c" as its last element,
// "b" as its second to last element, and "a" as its third-to-last element.
//
// See https://redis.io/commands/rpush/ for more information.
func (l *ListKeyspace[K, V]) PushRight(ctx context.Context, key K, values ...V) (newLen int64, err error) {
	const op = "push right"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// PopLeft pops a single element off the head of the list stored at key and returns it.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/lpop/ for more information.
func (l *ListKeyspace[K, V]) PopLeft(ctx context.Context, key K) (val V, err error) {
	const op = "pop left"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// PopRight pops a single element off the tail of the list stored at key and returns it.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/rpop/ for more information.
func (l *ListKeyspace[K, V]) PopRight(ctx context.Context, key K) (val V, err error) {
	const op = "pop right"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// Len reports the length of the list stored at key.
//
// Non-existing keys are considered as empty lists.
//
// See https://redis.io/commands/llen/ for more information.
func (l *ListKeyspace[K, V]) Len(ctx context.Context, key K) (length int64, err error) {
	const op = "list len"
	k, err := l.key(key, op)
	defer l.doTrace(op, false, k)(err)
	if err != nil {
		return 0, err
	}

	res, err := l.redis.LLen(ctx, k).Result()
	err = toErr(err, op, k)
	return res, err
}

// Trim trims the list stored at key to only contain the elements between the indices
// start and stop (inclusive). Both start and stop are zero-based indices.
//
// Negative indices can be used to indicate offsets from the end of the list,
// where -1 is the last element of the list, -2 the penultimate element, and so on.
//
// Out of range indices are valid and are treated as if they specify the start or end of the list,
// respectively. If start > stop the end result is an empty list.
//
// See https://redis.io/commands/ltrim/ for more information.
func (l *ListKeyspace[K, V]) Trim(ctx context.Context, key K, start, stop int64) error {
	const op = "list trim"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
	if err != nil {
		return err
	}
	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LTrim(ctx, k, start, stop)
	})
	return toErr(res.Err(), op, k)
}

// Set updates the list element with the given idx to val.
//
// Negative indices can be used to indicate offsets from the end of the list,
// where -1 is the last element of the list, -2 the penultimate element, and so on.
//
// An error is returned for out of bounds indices.
//
// See https://redis.io/commands/lset/ for more information.
func (l *ListKeyspace[K, V]) Set(ctx context.Context, key K, idx int64, val V) (err error) {
	const op = "list set"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
	if err != nil {
		return err
	}

	res := do(l.client, ctx, k, func(c cmdable) *redis.StatusCmd {
		return c.LSet(ctx, k, idx, val)
	})
	return toErr(res.Err(), op, k)
}

// Get returns the value of list element with the given idx.
//
// Negative indices can be used to indicate offsets from the end of the list,
// where -1 is the last element of the list, -2 the penultimate element, and so on.
//
// For out of bounds indices or the key not existing in the cache, an error matching Miss is returned.
//
// See https://redis.io/commands/lget/ for more information.
func (l *ListKeyspace[K, V]) Get(ctx context.Context, key K, idx int64) (val V, err error) {
	const op = "list get"
	k, err := l.key(key, op)
	defer l.doTrace(op, false, k)(err)
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

// Items returns all the elements in the list stored at key.
//
// If the key does not exist it returns an empty list.
//
// See https://redis.io/commands/lrange/ for more information.
func (l *ListKeyspace[K, V]) Items(ctx context.Context, key K) ([]V, error) {
	return l.getRange(ctx, key, 0, -1, "items")
}

// GetRange returns all the elements in the list stored at key between start and stop.
//
// Negative indices can be used to indicate offsets from the end of the list,
// where -1 is the last element of the list, -2 the penultimate element, and so on.
//
// If the key does not exist it returns an empty list.
//
// See https://redis.io/commands/lrange/ for more information.
func (l *ListKeyspace[K, V]) GetRange(ctx context.Context, key K, start, stop int64) ([]V, error) {
	return l.getRange(ctx, key, start, stop, "get range")
}

func (l *ListKeyspace[K, V]) getRange(ctx context.Context, key K, from, to int64, op string) (vals []V, err error) {
	k, err := l.key(key, op)
	defer l.doTrace(op, false, k)(err)
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

// InsertBefore inserts newVal into the list stored at key, at the position just before needle.
//
// If the list stored at key does not contain needle the value is not inserted,
// and an error matching Miss is returned.
//
// It returns the new list length.
//
// See https://redis.io/commands/linsert/ for more information.
func (l *ListKeyspace[K, V]) InsertBefore(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	const op = "insert before"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// InsertAfter inserts newVal into the list stored at key, at the position just after needle.
//
// It reports the new list length.
//
// If the list stored at key does not contain needle the value is not inserted,
// and an error matching Miss is returned.
//
// See https://redis.io/commands/linsert/ for more information.
func (l *ListKeyspace[K, V]) InsertAfter(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	const op = "insert after"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// RemoveAll removes all values equal to needle in the list stored at key.
//
// It reports the number of elements removed.
//
// If the list does not contain needle, or the list does not exist, it reports 0, nil.
//
// See https://redis.io/commands/lrem/ for more information.
func (l *ListKeyspace[K, V]) RemoveAll(ctx context.Context, key K, needle V) (removed int64, err error) {
	const op = "remove all"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
	if err != nil {
		return 0, err
	}

	res, err := do(l.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.LRem(ctx, k, 0, needle)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

// RemoveFirst removes the first 'count' values equal to needle in the list stored at key.
//
// It reports the number of elements removed.
//
// If the list does not contain needle, or the list does not exist, it reports 0, nil.
//
// See https://redis.io/commands/lrem/ for more information.
func (l *ListKeyspace[K, V]) RemoveFirst(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	const op = "remove first"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// RemoveLast removes the last 'count' values equal to needle in the list stored at key.
//
// It reports the number of elements removed.
//
// If the list does not contain needle, or the list does not exist, it reports 0, nil.
//
// See https://redis.io/commands/lrem/ for more information.
func (l *ListKeyspace[K, V]) RemoveLast(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	const op = "remove last"
	k, err := l.key(key, op)
	defer l.doTrace(op, true, k)(err)
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

// Move atomically moves an element from the list stored at src to the list stored at dst.
//
// The value moved can be either the head (fromPos == Left) or tail (fromPos == Right) of the list at src.
// Similarly, the value can be placed either at the head (toPos == Left) or tail (toPos == Right) of the list at dst.
//
// If src does not exist it reports an error matching Miss.
//
// If src and dst are the same list, the value is atomically rotated from one end to the other when fromPos != toPos,
// or if fromPos == toPos nothing happens.
func (l *ListKeyspace[K, V]) Move(ctx context.Context, src, dst K, fromPos, toPos ListPos) (moved V, err error) {
	const op = "list move"
	ks, err := l.keys([]K{src, dst}, op)
	defer l.doTrace(op, true, ks...)(err)
	if err != nil {
		return moved, err
	}
	srcKey, dstKey := ks[0], ks[1]

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
