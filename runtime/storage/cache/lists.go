package cache

import (
	"context"
	"errors"
)

func NewListKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *ListKeyspace[K, V] {
	return nil
}

type BasicType interface {
	~string | ~int | ~int64 | ~float64
}

type ListKeyspace[K any, V BasicType] struct {
	*client[K, V]
}

func (l *ListKeyspace[K, V]) LPush(ctx context.Context, key K, val V) (newLen int64, err error) {
	return toErr2(l.redis.LPush(ctx, l.key(key), val).Result())
}

func (l *ListKeyspace[K, V]) RPush(ctx context.Context, key K, val V) (newLen int64, err error) {
	return toErr2(l.redis.RPush(ctx, l.key(key), val).Result())
}

func (l *ListKeyspace[K, V]) LAdd(ctx context.Context, key K, val V) (newLen int64, err error) {
	return toErr2(l.redis.LPushX(ctx, l.key(key), val).Result())
}

func (l *ListKeyspace[K, V]) RAdd(ctx context.Context, key K, val V) (newLen int64, err error) {
	return toErr2(l.redis.RPushX(ctx, l.key(key), val).Result())
}

func (l *ListKeyspace[K, V]) LPop(ctx context.Context, key K) (val *V, err error) {
	res, err := toErr2(l.redis.LPop(ctx, l.key(key)).Result())
	return l.valOrNil(res, err)
}

func (l *ListKeyspace[K, V]) RPop(ctx context.Context, key K) (val *V, err error) {
	res, err := toErr2(l.redis.RPop(ctx, l.key(key)).Result())
	return l.valOrNil(res, err)
}

func (l *ListKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	return toErr2(l.redis.LLen(ctx, l.key(key)).Result())
}

func (l *ListKeyspace[K, V]) Trim(ctx context.Context, key K, start, stop int64) error {
	return toErr(l.redis.LTrim(ctx, l.key(key), start, stop).Err())
}

func (l *ListKeyspace[K, V]) Set(ctx context.Context, key K, idx int64, val V) error {
	return toErr(l.redis.LSet(ctx, l.key(key), idx, val).Err())
}

func (l *ListKeyspace[K, V]) Get(ctx context.Context, key K, idx int64) (*V, error) {
	res, err := toErr2(l.redis.LIndex(ctx, l.key(key), idx).Result())
	return l.valOrNil(res, err)
}

var ErrNotFound = errors.New("element not found")

func (l *ListKeyspace[K, V]) InsertBefore(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	newLen, err = toErr2(l.redis.LInsertBefore(ctx, l.key(key), needle, newVal).Result())
	if newLen == -1 {
		return 0, ErrNotFound
	}
	return newLen, err
}

func (l *ListKeyspace[K, V]) InsertAfter(ctx context.Context, key K, needle, newVal V) (newLen int64, err error) {
	newLen, err = toErr2(l.redis.LInsertAfter(ctx, l.key(key), needle, newVal).Result())
	if newLen == -1 {
		return 0, ErrNotFound
	}
	return newLen, err
}

func (l *ListKeyspace[K, V]) RemoveAll(ctx context.Context, key K, needle V) (removed int64, err error) {
	return toErr2(l.redis.LRem(ctx, l.key(key), 0, needle).Result())
}

func (l *ListKeyspace[K, V]) RemoveFirst(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	if count < 0 {
		panic("RemoveFirst: negative count")
	} else if count == 0 {
		return 0, nil
	}
	return toErr2(l.redis.LRem(ctx, l.key(key), count, needle).Result())
}

func (l *ListKeyspace[K, V]) RemoveLast(ctx context.Context, key K, count int64, needle V) (removed int64, err error) {
	if count < 0 {
		panic("RemoveFirst: negative count")
	} else if count == 0 {
		return 0, nil
	}
	return toErr2(l.redis.LRem(ctx, l.key(key), -count, needle).Result())
}

type ListPos string

const (
	Left  ListPos = "LEFT"
	Right ListPos = "RIGHT"
)

func (l *ListKeyspace[K, V]) Move(ctx context.Context, src, dst K, fromPos, toPos ListPos) (moved *V, err error) {
	res, err := toErr2(l.redis.LMove(ctx, l.key(src), l.key(dst), string(fromPos), string(toPos)).Result())
	return l.valOrNil(res, err)
}
