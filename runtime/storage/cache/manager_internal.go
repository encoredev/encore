package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/syncutil"
	"encore.dev/appruntime/shared/testsupport"
)

// Manager manages cache clients.
type Manager struct {
	static  *config.Static
	runtime *config.Runtime
	rt      *reqtrack.RequestTracker
	ts      *testsupport.Manager
	json    jsoniter.API

	initTestSrv syncutil.Once
	testSrv     *miniredis.Miniredis

	clientMu sync.RWMutex
	clients  map[string]*redis.Client
}

func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker, ts *testsupport.Manager, json jsoniter.API) *Manager {
	return &Manager{
		static:  static,
		runtime: runtime,
		rt:      rt,
		ts:      ts,
		json:    json,
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

	// Are we in a test or running in Encore Cloud? If so, use the redismock library.
	if mgr.static.Testing || mgr.runningInEncoreCloud() {
		cl, err := mgr.newMiniredisClient()
		if err != nil {
			panic(fmt.Sprintf("cache: unable to start redis mock: %v", err))
		}
		mgr.clients[clusterName] = cl
		return cl
	}

	for _, rdb := range mgr.runtime.RedisDatabases {
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

func (mgr *Manager) runningInEncoreCloud() bool {
	if mgr.runtime != nil && mgr.runtime.EnvCloud == "encore" {
		return true
	}
	return false
}

func (mgr *Manager) newClient(rdb *config.RedisDatabase) (*redis.Client, error) {
	srv := mgr.runtime.RedisServers[rdb.ServerID]
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

	if srv.EnableTLS || srv.ServerCACert != "" || srv.ClientCert != "" {
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

func (mgr *Manager) newMiniredisClient() (*redis.Client, error) {
	err := mgr.initTestSrv.Do(func() error {
		var err error
		mgr.testSrv, err = miniredis.Run()

		// Periodically clean up cache keys if running in Encore Cloud.
		if err == nil && mgr.runningInEncoreCloud() {
			go miniredisCleanup(mgr.testSrv, 15*time.Second, 100)
		}

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

	mgr.clientMu.Lock()
	mgr.clientMu.Unlock()
	for _, c := range mgr.clients {
		_ = c.Close()
	}
}

func newClient[K, V any](cluster *Cluster, cfg KeyspaceConfig,
	fromRedis func(string) (V, error),
	toRedis func(V) (any, error),
) *client[K, V] {
	keyMapper := cfg.EncoreInternal_KeyMapper.(func(K) string)
	if mgr := cluster.mgr; mgr.static.Testing {
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
			return neverExpire
		}
	}

	return &client[K, V]{
		rt:        cluster.mgr.rt,
		redis:     cluster.cl,
		cfg:       cfg,
		expiry:    defaultExpiry,
		keyMapper: keyMapper,
		toRedis:   toRedis,
		fromRedis: fromRedis,
	}
}

type client[K, V any] struct {
	rt        *reqtrack.RequestTracker
	redis     *redis.Client
	cfg       KeyspaceConfig
	expiry    ExpiryFunc
	keyMapper func(K) string
	toRedis   func(V) (any, error)
	fromRedis func(string) (V, error)
	traceOpID uint64
}

func (c *client[K, V]) with(opts []WriteOption) *client[K, V] {
	expFunc := c.expiry
	for _, opt := range opts {
		if eo, ok := opt.(expiryOption); ok {
			expFunc = eo.expiry
		}
	}

	c2 := *c
	c2.expiry = expFunc
	return &c2
}

func (s *client[K, V]) key(k K, op string) (string, error) {
	res := s.keyMapper(k)
	if strings.HasPrefix(res, "__encore") {
		return "", &OpError{
			Operation: op,
			RawKey:    res,
			Err:       errors.New(`use of reserved key prefix "encore"`),
		}
	}
	return res, nil
}

func (s *client[K, V]) keys(keys []K, op string) ([]string, error) {
	strs := make([]string, len(keys))
	var err error
	for i, k := range keys {
		strs[i], err = s.key(k, op)
		if err != nil {
			return nil, err
		}
	}
	return strs, nil
}

func (s *client[K, V]) fromRedisMulti(res []string) ([]V, error) {
	vals := make([]V, len(res))
	for i, r := range res {
		v, err := s.fromRedis(r)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return vals, nil
}

func (s *client[K, V]) valPtr(res string) (*V, error) {
	vv, err := s.fromRedis(res)
	if err != nil {
		return nil, err
	}
	return &vv, nil
}

func (s *client[K, V]) valOrNil(res string, err error) (*V, error) {
	if err == redis.Nil || errors.Is(err, Miss) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return s.valPtr(res)
}

func (s *client[K, V]) expiryCmd(ctx context.Context, key string) *redis.BoolCmd {
	now := time.Now()
	expTime := s.expiry(now)
	if expTime == keepTTL {
		return nil
	} else if expTime == neverExpire {
		return redis.NewBoolCmd(ctx, "persist", key)
	}

	expMs := expTime.UnixNano() / int64(time.Millisecond)
	return redis.NewBoolCmd(ctx, "pexpireat", key, expMs)
}

func (s *client[K, V]) expiryDur() time.Duration {
	now := time.Now()
	expTime := s.expiry(now)

	var exp time.Duration
	switch {
	case expTime == neverExpire:
		exp = 0
	case expTime == keepTTL:
		exp = redis.KeepTTL
	default:
		exp = expTime.Sub(now)
	}
	return exp
}

func (c *client[K, V]) doTrace(op string, write bool, keys ...string) func(error) {
	opID := c.traceStart(op, write, keys...)
	return func(err error) {
		c.traceEnd(opID, err)
	}
}

func (c *client[K, V]) traceStart(op string, write bool, keys ...string) (opID uint64) {
	if curr := c.rt.Current(); curr.Trace != nil && curr.Req != nil {
		opID = atomic.AddUint64(&c.traceOpID, 1)
		if opID == 0 {
			// We wrapped around. Increment again since we use the zero opID
			// to indicate that nothing was traced.
			opID = atomic.AddUint64(&c.traceOpID, 1)
		}

		curr.Trace.CacheOpStart(trace.CacheOpStartParams{
			DefLoc:    c.cfg.EncoreInternal_DefLoc,
			Operation: op,
			IsWrite:   write,
			Keys:      keys,
			Inputs:    nil,
			SpanID:    curr.Req.SpanID,
			Goid:      curr.Goctr,
			OpID:      opID,
			Stack:     stack.Build(3),
		})
	}

	return opID
}

func (c *client[K, V]) traceEnd(opID uint64, err error, values ...any) {
	if opID == 0 { // indicates the operation start was not traced
		return
	}

	if curr := c.rt.Current(); curr.Trace != nil && curr.Req != nil {
		var cacheErr error
		var res trace.CacheOpResult
		switch {
		case err == nil:
			res = trace.CacheOK
		case errors.Is(err, Miss):
			res = trace.CacheNoSuchKey
		case errors.Is(err, KeyExists):
			res = trace.CacheConflict
		case err != nil:
			res = trace.CacheErr
			cacheErr = err
		}

		curr.Trace.CacheOpEnd(trace.CacheOpEndParams{
			OpID:    opID,
			Res:     res,
			Err:     cacheErr,
			Outputs: nil, // TODO(andre) add
		})
	}
}

type errWrapper struct {
	err error
}

func (ew errWrapper) Error() string {
	return ew.err.Error()
}

func (ew errWrapper) Unwrap() error {
	return ew.err
}

func toErr(err error, op, key string) error {
	if err == nil {
		return nil
	}

	// Convert redis.Nil to cache.Miss.
	if errors.Is(err, redis.Nil) {
		err = Miss
	}

	// Is it already an OpError? If so, do nothing.
	var opErr *OpError
	if errors.As(err, &opErr) {
		return err
	}

	err = &OpError{Operation: op, RawKey: key, Err: err}

	// Wrap the error in an opaque type to ensure callers check for errors
	// with errors.Is and errors.As.
	return errWrapper{err}
}

func toErr2[T any](val T, err error, op, key string) (T, error) {
	return val, toErr(err, op, key)
}

func orDefault[T comparable](val, orDefault T) T {
	var zero T
	if val == zero {
		return orDefault
	}
	return val
}

func miniredisCleanup(srv *miniredis.Miniredis, every time.Duration, maxKeys int) {
	var acc time.Duration
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		srv.FastForward(time.Second)

		// Clean up keys every so often
		acc += every
		if acc > every {
			acc -= every

			keys := srv.Keys()
			for len(keys) > 100 {
				id := rand.Intn(len(keys))
				if keys[id] != "" {
					srv.Del(keys[id])
					keys[id] = "" // mark it as deleted
				}
			}
		}
	}
}
