package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/runtimeutil/syncutil"
	"encore.dev/appruntime/testsupport"
)

// Manager manages cache clients.
type Manager struct {
	cfg *config.Config
	ts  *testsupport.Manager

	initTestSrv syncutil.Once
	testSrv     *miniredis.Miniredis

	clientMu sync.RWMutex
	clients  map[string]*redis.Client
}

func NewManager(cfg *config.Config, ts *testsupport.Manager) *Manager {
	return &Manager{
		cfg:     cfg,
		ts:      ts,
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

	// Are we in a test? If so, use the redismock library.
	if mgr.cfg.Static.Testing {
		cl, err := mgr.newTestClient()
		if err != nil {
			panic(fmt.Sprintf("cache: unable to start redis mock: %v", err))
		}
		mgr.clients[clusterName] = cl
		return cl
	}

	for _, rdb := range mgr.cfg.Runtime.RedisDatabases {
		if rdb.EncoreName == clusterName {
			cl, err := mgr.newClient(rdb)
			if err != nil {
				panic(fmt.Sprintf("cache: unable to create redis client: %v", err))
			}
			mgr.clients[clusterName] = cl
			return cl
		}
	}

	panic(fmt.Sprintf("cache: unknown cluster %q", clusterName))
}

func (mgr *Manager) newClient(rdb *config.RedisDatabase) (*redis.Client, error) {
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

	if srv.ServerCACert != "" || srv.ClientCert != "" {
		opts.TLSConfig = &tls.Config{}
		if srv.ServerCACert != "" {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(srv.ServerCACert)) {
				return nil, fmt.Errorf("invalid server ca cert")
			}
			opts.TLSConfig.RootCAs = caCertPool
		}
		if srv.ClientCert != "" {
			cert, err := tls.X509KeyPair([]byte(srv.ClientCert), []byte(srv.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("parse client cert: %v", err)
			}
			opts.TLSConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return redis.NewClient(opts), nil
}

func (mgr *Manager) newTestClient() (*redis.Client, error) {
	err := mgr.initTestSrv.Do(func() error {
		var err error
		mgr.testSrv, err = miniredis.Run()
		return err
	})
	if err != nil {
		return nil, err
	}

	opts := &redis.Options{
		Network:      "tcp",
		Addr:         mgr.testSrv.Addr(),
		DB:           0,
		MinIdleConns: 1,
		PoolSize:     runtime.GOMAXPROCS(0) * 10,
	}
	cl := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	err = cl.Ping(ctx).Err()
	cancel()
	return cl, err
}

func (mgr *Manager) Shutdown(force context.Context) {
	// The redis client does not have the concept of graceful shutdown,
	// so wait for the force shutdown before we close the connections.
	<-force.Done()

	// TODO
	//_ = mgr.redis.Close()
}

func newClient[K, V any](cluster *Cluster, cfg KeyspaceConfig) *client[K, V] {
	keyMapper := cfg.EncoreInternal_KeyMapper.(func(K) string)
	valueMapper := cfg.EncoreInternal_ValueMapper.(func(string) (V, error))

	if mgr := cluster.mgr; mgr.cfg.Static.Testing {
		// If we're running tests, map keys to a test-specific key.
		orig := keyMapper
		keyMapper = func(k K) string {
			key := orig(k)
			if t := mgr.ts.CurrentTest(); t != nil {
				key = t.Name() + "::" + key
			}
			return key
		}
	}

	// Determine the default expiry function.
	defaultExpiry := cfg.DefaultExpiry
	if defaultExpiry == nil {
		defaultExpiry = func(now time.Time) time.Time {
			return NeverExpire
		}
	}

	return &client[K, V]{
		redis:         cluster.cl,
		cfg:           cfg,
		defaultExpiry: defaultExpiry,
		keyMapper:     keyMapper,
		valueMapper:   valueMapper,
	}
}

type client[K, V any] struct {
	redis         *redis.Client
	cfg           KeyspaceConfig
	defaultExpiry ExpiryFunc
	keyMapper     func(K) string
	valueMapper   func(string) (V, error)
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

func (s *client[K, V]) expiryCmd(ctx context.Context, key string, opts []WriteOption) *redis.BoolCmd {
	now := time.Now()
	expTime := s.expiryTime(now, opts)
	if expTime == keepTTL {
		return nil
	} else if expTime == NeverExpire {
		return redis.NewBoolCmd(ctx, "persist", key)
	}

	expMs := expTime.UnixNano() / int64(time.Millisecond)
	return redis.NewBoolCmd(ctx, "pexpireat", key, expMs)
}

func (s *client[K, V]) expiryDur(opts []WriteOption) time.Duration {
	now := time.Now()
	expTime := s.expiryTime(now, opts)

	var exp time.Duration
	switch {
	case expTime == NeverExpire:
		exp = 0
	case expTime == keepTTL:
		exp = redis.KeepTTL
	default:
		exp = expTime.Sub(now)
	}
	return exp
}

func (s *client[K, V]) expiryTime(now time.Time, opts []WriteOption) time.Time {
	var expiry time.Time

	// Check if we have any option that overrides the expiry
	found := false
	for _, o := range opts {
		if exp, ok := o.(expiryOption); ok {
			exp.setExpiry(now, &expiry)
			found = true
		}
	}

	if !found {
		expiry = s.defaultExpiry(now)
	}

	return expiry
}

func toErr(err error) error {
	if err == redis.Nil {
		err = Nil
	}
	return err
}

func toErr2[T any](val T, err error) (T, error) {
	if err == redis.Nil {
		err = Nil
	}
	return val, err
}

func orDefault[T comparable](val, orDefault T) T {
	var zero T
	if val == zero {
		return orDefault
	}
	return val
}
