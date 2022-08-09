package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
)

func NewSetKeyspace[K any, V BasicType](cluster *Cluster, cfg KeyspaceConfig) *SetKeyspace[K, V] {
	return &SetKeyspace[K, V]{
		newClient[K, V](cluster, cfg),
	}
}

type SetKeyspace[K any, V BasicType] struct {
	c *client[K, V]
}

func (s *SetKeyspace[K, V]) Add(ctx context.Context, key K, vals ...V) (added int64, err error) {
	args := make([]any, 2+len(vals))
	args[0] = "sadd"
	args[1] = s.c.key(key)
	for i, v := range vals {
		args[2+i] = v
	}
	cmd := redis.NewIntCmd(ctx, args...)
	_ = s.c.redis.Process(ctx, cmd)
	return cmd.Result()
}

func (s *SetKeyspace[K, V]) Remove(ctx context.Context, key K, vals ...V) (removed int64, err error) {
	args := make([]any, 2+len(vals))
	args[0] = "srem"
	args[1] = s.c.key(key)
	for i, v := range vals {
		args[2+i] = v
	}
	cmd := redis.NewIntCmd(ctx, args...)
	_ = s.c.redis.Process(ctx, cmd)
	return cmd.Result()
}

func (s *SetKeyspace[K, V]) Contains(ctx context.Context, key K, val V) (bool, error) {
	return s.c.redis.SIsMember(ctx, s.c.key(key), val).Result()
}

func (s *SetKeyspace[K, V]) Len(ctx context.Context, key K) (int64, error) {
	return s.c.redis.SCard(ctx, s.c.key(key)).Result()
}
