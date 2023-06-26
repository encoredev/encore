package cache

import (
	"context"
	"errors"
	"net"

	"github.com/go-redis/redis/v8"
)

var ErrNoopClient = errors.New(
	"cache: this service is not configured to use this cache",
)

func newNoopClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		MinIdleConns: 0,
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, ErrNoopClient
		},
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			return ErrNoopClient
		},
	})

	client.AddHook(&noopHook{})

	return client
}

type noopHook struct{}

func (n *noopHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return nil, ErrNoopClient
}

func (n *noopHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return ErrNoopClient
}

func (n *noopHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return nil, ErrNoopClient
}

func (n *noopHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return ErrNoopClient
}
