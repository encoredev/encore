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
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/trace"
	"encore.dev/internal/stack"
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

func (db *Database) init() {
	db.initOnce.Do(func() {
		if db.pool == nil {
			db.pool = db.mgr.getPool(db.name)
		}
		db.connStr = stdlib.RegisterConnConfig(db.pool.Config().ConnConfig)
	})
}

// Stdlib returns a *sql.DB object that is connected to the same db,
// for use with libraries that expect a *sql.DB.
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

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
//
// See (*database/sql.DB).ExecContext() for additional documentation.
func (db *Database) Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	db.init()
	qid := atomic.AddUint64(&db.mgr.queryCtr, 1)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(4),
		})
	}

	res, err := db.pool.Exec(markTraced(ctx), query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return res, err
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
//
// See (*database/sql.DB).QueryContext() for additional documentation.
func (db *Database) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	db.init()
	qid := atomic.AddUint64(&db.mgr.queryCtr, 1)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(4),
		})
	}

	rows, err := db.pool.Query(markTraced(ctx), query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

// QueryRow executes a query that is expected to return at most one row.
//
// See (*database/sql.DB).QueryRowContext() for additional documentation.
func (db *Database) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	db.init()
	qid := atomic.AddUint64(&db.mgr.queryCtr, 1)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(4),
		})
	}

	rows, err := db.pool.Query(markTraced(ctx), query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return r
}

// Begin opens a new database transaction.
//
// See (*database/sql.DB).Begin() for additional documentation.
func (db *Database) Begin(ctx context.Context) (*Tx, error) {
	db.init()
	tx, err := db.pool.Begin(markTraced(ctx))
	err = convertErr(err)
	if err != nil {
		return nil, err
	}
	txid := atomic.AddUint64(&db.mgr.txidCtr, 1)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTxStart(trace.DBTxStartParams{
			SpanID: curr.Req.SpanID,
			Goid:   curr.Goctr,
			TxID:   txid,
			Stack:  stack.Build(4),
		})
	}

	return &Tx{mgr: db.mgr, txid: txid, std: tx}, nil
}

// Driver returns the underlying database driver for this database connection pool.
//
//	var db = sqldb.Driver[*pgxpool.Pool](sqldb.Named("mydatabase"))
//
// This is defined as a generic function to allow compile-time type checking
// that the Encore application is expecting a driver that is supported.
//
// At some point in the future where Encore adds support for a different database driver
// this will be made with backwards compatibility in mind, providing ample notice and
// time to migrate in an opt-in fashion.
func Driver[T SupportedDrivers](db *Database) T {
	return any(db.pool).(T)
}

// SupportedDrivers is a type list of all supported database drivers.
// Currently only [*pgxpool.Pool] is supported.
type SupportedDrivers interface {
	*pgxpool.Pool
}
