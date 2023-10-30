package sqldb

import (
	"context"
	"database/sql/driver"
	"sync"
)

var (
	registerNoopDriverOnce sync.Once
)

const noopDriverName = "__encore_noop"

// noopDriver is a driver.Driver that returns will "connect" and manage
// a connection to a database without an error, however whenever you try and use
// the database, it will return a noop error.
//
// This is because we expect people to use drivers as package level variables,
// but some services may not be running
type noopDriver struct{}

var (
	_ driver.Driver        = noopDriver{}
	_ driver.DriverContext = noopDriver{}
)

func (n noopDriver) Open(name string) (driver.Conn, error) {
	return noopConn{}, nil
}

func (n noopDriver) OpenConnector(name string) (driver.Connector, error) {
	return noopConnector{}, nil
}

// noopConnector is a driver.Connector that returns will "connect" and
// manage a connection to a database without an error using the noopConn
type noopConnector struct{}

var (
	_ driver.Connector = noopConnector{}
)

func (n noopConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return noopConn{}, nil
}

func (n noopConnector) Driver() driver.Driver {
	return noopDriver{}
}

// noopConn is a driver.Conn that will error on any operation not
// related to managing the connection
type noopConn struct{}

var (
	_ driver.Conn               = noopConn{}
	_ driver.ConnBeginTx        = noopConn{}
	_ driver.ConnPrepareContext = noopConn{}
	_ driver.ExecerContext      = noopConn{}
	_ driver.Pinger             = noopConn{}
	_ driver.QueryerContext     = noopConn{}
	_ driver.SessionResetter    = noopConn{}
	_ driver.NamedValueChecker  = noopConn{}
)

func (n noopConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errNoopDB
}

func (n noopConn) Close() error {
	return nil // Don't return an error here as we want "connections" to act normal until they are used
}

func (n noopConn) Begin() (driver.Tx, error) {
	return nil, errNoopDB
}

func (n noopConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return nil, errNoopDB
}

func (n noopConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return nil, errNoopDB
}

func (n noopConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, errNoopDB
}

func (n noopConn) Ping(ctx context.Context) error {
	return nil // Don't return an error here as we want "connections" to act normal until they are used
}

func (n noopConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return nil, errNoopDB
}

func (n noopConn) ResetSession(ctx context.Context) error {
	return nil // Don't return an error here as we want "connections" to act normal until they are used
}

func (n noopConn) CheckNamedValue(value *driver.NamedValue) error {
	return errNoopDB
}
