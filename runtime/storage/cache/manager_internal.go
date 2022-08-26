package cache

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"

	"encore.dev/appruntime/config"
)

// Manager manages cache clients.
type Manager struct {
	cfg *config.Config

	clientMu sync.RWMutex
	clients  map[string]*redis.Client
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		clients: make(map[string]*redis.Client),
	}
}

func (mgr *Manager) getClient(clusterName string) *redis.Client {
	mgr.clientMu.RLock()
	cl := mgr.clients[clusterName]
	mgr.clientMu.RUnlock()
	if cl != nil {
		return cl
	}

	// Client not found; acquire a write lock and set up the client
	mgr.clientMu.Lock()
	defer mgr.clientMu.Unlock()

	// Did we race someone else and they set up the client first?
	if cl := mgr.clients[clusterName]; cl != nil {
		return cl
	}

	for _, rdb := range mgr.cfg.Runtime.RedisDatabases {
		if rdb.EncoreName == clusterName {
			cl := mgr.newClient(rdb)
			mgr.clients[clusterName] = cl
			return cl
		}
	}

	panic(fmt.Sprintf("cache: unknown cluster name %q", clusterName))
}

func (mgr *Manager) newClient(rdb *config.RedisDatabase) *redis.Client {
	srv := mgr.cfg.Runtime.RedisServers[rdb.ServerID]
	opts := &redis.Options{
		Network:      "tcp",
		Addr:         srv.Host,
		Username:     srv.User,
		Password:     srv.Password,
		DB:           rdb.Database,
		MinIdleConns: orDefault(rdb.MinConnections, 1),
		PoolSize:     orDefault(rdb.MaxConnections, runtime.GOMAXPROCS(0)*10),
	}
	if strings.HasPrefix(srv.Host, "/") {
		opts.Network = "unix"
	}

	// TODO(andre) handle TLS config

	return redis.NewClient(opts)
}

func (mgr *Manager) Shutdown(force context.Context) {
	// The redis client does not have the concept of graceful shutdown,
	// so wait for the force shutdown before we close the connections.
	<-force.Done()

	// TODO
	//_ = mgr.redis.Close()
}

func newClient[K, V any](cluster *Cluster, cfg KeyspaceConfig) *client[K, V] {
	return &client[K, V]{
		redis:       cluster.cl,
		cfg:         cfg,
		keyMapper:   cfg.EncoreInternal_KeyMapper.(func(K) string),
		valueMapper: cfg.EncoreInternal_ValueMapper.(func(string) (V, error)),
	}
}

type client[K, V any] struct {
	redis       *redis.Client
	cfg         KeyspaceConfig
	keyMapper   func(K) string
	valueMapper func(string) (V, error)
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
	if err == redis.Nil || err == Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return s.valPtr(res)
}

func orDefault[T comparable](val, orDefault T) T {
	var zero T
	if val == zero {
		return orDefault
	}
	return val
}
