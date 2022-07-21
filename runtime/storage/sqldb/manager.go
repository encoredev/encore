package sqldb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgx/v4/stdlib"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
)

type Database struct {
	name string
	mgr  *Manager

	initOnce sync.Once
	pool     *pgxpool.Pool
	connStr  string

	stdlibOnce sync.Once
	stdlib     *sql.DB
}

// Manager manages database connections.
type Manager struct {
	rt  *reqtrack.RequestTracker
	cfg *config.Config

	mu  sync.RWMutex
	dbs map[string]*Database

	// Accessed atomically
	txidCtr  uint64
	queryCtr uint64
}

func NewManager(cfg *config.Config, rt *reqtrack.RequestTracker) *Manager {
	return &Manager{
		rt:  rt,
		cfg: cfg,
		dbs: make(map[string]*Database),
	}
}

// GetCurrentDB gets the database for the current request.
func (mgr *Manager) GetCurrentDB() *Database {
	var dbName string
	if curr := mgr.rt.Current(); curr.Req != nil {
		dbName = curr.Req.Service
	} else if testSvc := mgr.cfg.Static.TestService; testSvc != "" {
		dbName = testSvc
	} else {
		panic("sqldb: no current request")
	}
	return mgr.GetDB(dbName)
}

func (mgr *Manager) GetDB(dbName string) *Database {
	mgr.mu.RLock()
	db, ok := mgr.dbs[dbName]
	mgr.mu.RUnlock()
	if ok {
		return db
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	// Check again now that we've re-acquired the mutex
	if db, ok := mgr.dbs[dbName]; ok {
		return db
	}
	db = &Database{
		name: dbName,
		mgr:  mgr,
		pool: mgr.getPool(dbName),
	}
	mgr.dbs[dbName] = db
	return db
}

// getPool returns a database connection pool for the given database name.
// Each time it's called it returns a new pool.
func (mgr *Manager) getPool(dbName string) *pgxpool.Pool {
	var db *config.SQLDatabase
	for _, d := range mgr.cfg.Runtime.SQLDatabases {
		if d.EncoreName == dbName {
			db = d
			break
		}
	}
	if db == nil {
		panic("sqldb: unknown database: " + dbName)
	}

	srv := mgr.cfg.Runtime.SQLServers[db.ServerID]
	cfg, err := dbConf(srv, db)
	if err != nil {
		panic("sqldb: " + err.Error())
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), cfg)
	if err != nil {
		panic("sqldb: setup db: " + err.Error())
	}

	return pool
}

func (mgr *Manager) Shutdown(force context.Context) {
	var wg sync.WaitGroup
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	wg.Add(len(mgr.dbs))
	for _, db := range mgr.dbs {
		db := db
		go func() {
			defer wg.Done()
			db.shutdown(force)
		}()
	}
	wg.Wait()
}

func (mgr *Manager) Named(name string) *Database {
	return mgr.GetDB(name)
}

func (db *Database) init() {
	db.initOnce.Do(func() {
		if db.pool == nil {
			db.pool = db.mgr.getPool(db.name)
		}
		db.connStr = stdlib.RegisterConnConfig(db.pool.Config().ConnConfig)
	})
}

func (db *Database) Stdlib() *sql.DB {
	db.init()
	registerDriver.Do(func() {
		stdlibDriver = &wrappedDriver{
			parent: stdlib.GetDefaultDriver(),
			mw:     &interceptor{mgr: db.mgr},
		}
		sql.Register(driverName, stdlibDriver)
	})

	var openErr error
	db.stdlibOnce.Do(func() {
		c, err := stdlibDriver.(driver.DriverContext).OpenConnector(db.connStr)
		if err == nil {
			db.stdlib = sql.OpenDB(c)

			// Set the pool size based on the config.
			cfg := db.pool.Config()
			maxConns := int(cfg.MaxConns)
			db.stdlib.SetMaxOpenConns(maxConns)
			db.stdlib.SetConnMaxIdleTime(cfg.MaxConnIdleTime)
			db.stdlib.SetMaxIdleConns(maxConns)
		}
		openErr = err
	})
	if openErr != nil {
		// This should never happen as (*stdlib.Driver).OpenConnector is hard-coded to never return nil.
		// Guard it with a panic so we detect it as early as possible in case this changes.
		panic("sqldb: stdlib.OpenConnector failed: " + openErr.Error())
	}
	return db.stdlib
}

func (db *Database) shutdown(force context.Context) {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.stdlib != nil {
		_ = db.stdlib.Close()
	}
}

var (
	registerDriver sync.Once
	stdlibDriver   driver.Driver
)

const driverName = "__encore_stdlib"

// dbConf computes a suitable pgxpool config given a database config.
func dbConf(srv *config.SQLServer, db *config.SQLDatabase) (*pgxpool.Config, error) {
	uri := fmt.Sprintf("user=%s password=%s dbname=%s", db.User, db.Password, db.DatabaseName)

	// Handle different ways of expressing the host
	if strings.HasPrefix(srv.Host, "/") {
		uri += " host=" + srv.Host // unix socket
	} else if host, port, err := net.SplitHostPort(srv.Host); err == nil {
		uri += fmt.Sprintf(" host=%s port=%s", host, port) // host:port
	} else {
		uri += " host=" + srv.Host // hostname
	}

	if srv.ServerCACert != "" {
		uri += " sslmode=verify-ca"
	} else {
		uri += " sslmode=prefer"
	}

	cfg, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid database uri: %v", err)
	}
	cfg.LazyConnect = true

	// Set the pool size based on the config.
	cfg.MaxConns = 30
	if n := db.MaxConnections; n > 0 {
		cfg.MaxConns = int32(n)
	}

	// If we have a server CA, set it in the TLS config.
	if srv.ServerCACert != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(srv.ServerCACert)) {
			return nil, fmt.Errorf("invalid server ca cert")
		}
		cfg.ConnConfig.TLSConfig.RootCAs = caCertPool
		cfg.ConnConfig.TLSConfig.ClientCAs = caCertPool
	}

	// If we have a client cert, set it in the TLS config.
	if srv.ClientCert != "" {
		cert, err := tls.X509KeyPair([]byte(srv.ClientCert), []byte(srv.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("parse client cert: %v", err)
		}
		cfg.ConnConfig.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}
