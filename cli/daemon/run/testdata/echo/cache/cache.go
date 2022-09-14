package cache

import (
	"context"

	"encore.dev/beta/errs"
	"encore.dev/storage/cache"
)

var cluster = cache.NewCluster("cluster", cache.ClusterConfig{
	EvictionPolicy: cache.AllKeysLFU,
})

var ints = cache.NewIntKeyspace[string](cluster, cache.KeyspaceConfig{
	KeyPattern: "int/:key",
})

type IncrResponse struct {
	Val int64
}

//encore:api public path=/cache/incr/:key
func Incr(ctx context.Context, key string) (*IncrResponse, error) {
	val, err := ints.Incr(ctx, key, 1)
	return &IncrResponse{Val: val}, err
}

type StructKey struct {
	Key   int
	Dummy string
}

type StructVal struct {
	Val string
}

var structs = cache.NewStructKeyspace[StructKey, StructVal](cluster, cache.KeyspaceConfig{
	KeyPattern: "struct/:Key/dummy/:Dummy",
})

//encore:api public method=POST path=/cache/struct/:key/:val
func PostStruct(ctx context.Context, key int, val string) error {
	err := structs.Set(ctx, StructKey{Key: key, Dummy: "x"}, StructVal{Val: val})
	return err
}

//encore:api public method=GET path=/cache/struct/:key
func GetStruct(ctx context.Context, key int) (StructVal, error) {
	val, err := structs.Get(ctx, StructKey{Key: key, Dummy: "x"})
	if err == cache.Miss {
		return StructVal{}, &errs.Error{Code: errs.NotFound}
	} else if err != nil {
		return StructVal{}, err
	}
	return val, nil
}

var lists = cache.NewListKeyspace[int, string](cluster, cache.KeyspaceConfig{
	KeyPattern: "list/:key/foo/:key",
})

//encore:api public method=POST path=/cache/list/:key/:val
func PostList(ctx context.Context, key int, val string) error {
	_, err := lists.PushRight(ctx, key, val)
	return err
}

type ListResponse struct {
	Vals []string
}

//encore:api public method=GET path=/cache/list/:key
func GetList(ctx context.Context, key int) (ListResponse, error) {
	vals, err := lists.Items(ctx, key)
	return ListResponse{vals}, err
}
