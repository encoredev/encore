package sqldb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/storage/sqldb/internal/stdlibdriver"
)

type Database struct {
	name string
	mgr  *Manager

	noopDB bool // true if this is a dummy database that does nothing and returns errors for all operations

	initOnce sync.Once
	pool     *pgxpool.Pool
	connStr  string

	stdlibOnce sync.Once
	stdlib     *sql.DB
}

var errNoopDB = errors.New("sqldb: this service is not configured to use this database. Use sqldb.Named in this service to get a reference and access to the database from this service")

func (db *Database) init() {
	if db.noopDB {
		return
	}

	db.initOnce.Do(func() {
		if db.pool == nil {
			pool, found := db.mgr.getPool(db.name)
			db.pool, db.noopDB = pool, !found
		}

		if !db.noopDB {
			db.connStr = stdlibdriver.RegisterConnConfig(db.pool.Config().ConnConfig)
		}
	})
}

// Stdlib returns a *sql.DB object that is connected to the same db,
// for use with libraries that expect a *sql.DB.
func (db *Database) Stdlib() *sql.DB {
	// If this is a noop database, return a dummy *sql.DB that returns errors for all operations.
	if db.noopDB {
		registerNoopDriverOnce.Do(func() {
			sql.Register(noopDriverName, noopDriver{})
		})

		return sql.OpenDB(noopConnector{})
	}

	db.init()

	var openErr error
	db.stdlibOnce.Do(func() {
		c, err := registerStdlibDriver(db.mgr).(driver.DriverContext).OpenConnector(db.connStr)
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

func (db *Database) shutdown(p *shutdown.Process) {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.stdlib != nil {
		_ = db.stdlib.Close()
	}
}

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
	if db.noopDB {
		return nil, errNoopDB
	}

	db.init()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		eventParams = trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			Query:       query,
			TxStartID:   0,
			Stack:       stack.Build(4),
		})
	}

	res, err := db.pool.Exec(markTraced(ctx), query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return res, err
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
//
// See (*database/sql.DB).QueryContext() for additional documentation.
func (db *Database) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	if db.noopDB {
		return nil, errNoopDB
	}

	db.init()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		eventParams = trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			Query:       query,
			Stack:       stack.Build(4),
		})
	}

	rows, err := db.pool.Query(markTraced(ctx), query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
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
	if db.noopDB {
		return &Row{err: errNoopDB}
	}

	db.init()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		eventParams = trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			Query:       query,
			Stack:       stack.Build(4),
		})
	}

	rows, err := db.pool.Query(markTraced(ctx), query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return r
}

// Begin opens a new database transaction.
//
// See (*database/sql.DB).Begin() for additional documentation.
func (db *Database) Begin(ctx context.Context) (*Tx, error) {
	if db.noopDB {
		return nil, errNoopDB
	}

	db.init()
	tx, err := db.pool.Begin(markTraced(ctx))
	err = convertErr(err)
	if err != nil {
		return nil, err
	}

	var startID model.TraceEventID
	curr := db.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		startID = curr.Trace.DBTransactionStart(trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
		}, stack.Build(4))
	}

	return &Tx{mgr: db.mgr, std: tx, startID: startID}, nil
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
	if db.noopDB {
		var zero T
		return zero
	}

	return any(db.pool).(T)
}

// SupportedDrivers is a type list of all supported database drivers.
// Currently only [*pgxpool.Pool] is supported.
type SupportedDrivers interface {
	*pgxpool.Pool
}

// DriverConn provides access to the underlying driver connection given a stdlib
// *sql.Conn connection. The driverConn must not be used outside of f, and conn
// must be a *sql.Conn originating from sqldb.Database or this function panics.
//
//	conn, _ := db.Stdlib().Conn(ctx) // Checkout a connection from the pool
//	sqldb.DriverConn(conn, func(driverConn *pgx.Conn) error) error {
//	  // do stuff with *pgx.Conn
//	}
//
// This is defined as a generic function to allow compile-time type checking
// that the Encore application is expecting a driver that is supported.
//
// At some point in the future where Encore adds support for a different
// database driver this will be made with backwards compatibility in mind,
// providing ample notice and time to migrate in an opt-in fashion.
func DriverConn[T SupportedDriverConns](conn *sql.Conn, f func(driverConn T) error) error {
	return conn.Raw(func(c any) error {
		switch c := c.(type) {
		case wrappedConn:
			parentConn := c.parent
			rawConn := (parentConn).(*stdlibdriver.Conn).Conn()
			return f(rawConn)
		case noopConn:
			return errNoopDB
		default:
			panic(fmt.Sprintf("sqldb.DriverConn: unsupported connection type %T", c))
		}
	})
}

// SupportedDriverConns is a type list of all supported database drivers
// connections. Currently only [*pgx.Conn] is supported.
type SupportedDriverConns interface {
	*pgx.Conn
}
