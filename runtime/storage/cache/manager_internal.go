package cache

import (
	"context"

	"github.com/go-redis/redis/v8"

	"encore.dev/appruntime/config"
)

// Manager manages cache clients.
type Manager struct {
	cfg *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

func (mgr *Manager) Shutdown(force context.Context) {
	// The redis client does not have the concept of graceful shutdown,
	// so wait for the force shutdown before we close the connections.
	<-force.Done()

	// TODO
	//_ = mgr.redis.Close()
}

type (
	keyMapper[K any]   func(K) string
	valueMapper[V any] func(string) (V, error)
)

func newClient[K, V any](cache *Cache, cfg KeyspaceConfig) *client[K, V] {
	return &client[K, V]{
		redis:       cache.cl,
		cfg:         cfg,
		keyMapper:   cfg.EncoreInternal_KeyMapper.(keyMapper[K]),
		valueMapper: cfg.EncoreInternal_ValueMapper.(valueMapper[V]),
	}
}

type client[K, V any] struct {
	redis       *redis.Client
	cfg         KeyspaceConfig
	keyMapper   keyMapper[K]
	valueMapper valueMapper[V]
}

func (s *client[K, V]) key(k K) string {
	return s.keyMapper(k)
}

func (s *client[K, V]) val(res string) (V, error) {
	return s.valueMapper(res)
}

func (s *client[K, V]) valPtr(res string) (*V, error) {
	vv, err := s.val(res)
	if err != nil {
		return nil, err
	}
	return &vv, nil
}

func (s *client[K, V]) valOrNil(res string, err error) (*V, error) {
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return s.valPtr(res)
}
