//go:build encore_app

package sqldb

import (
	"context"

	_ "encore.dev/appruntime/app/appinit" // Force the app to initialise all singletons before these functions can be used
)

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
//
// See (*database/sql.DB).ExecContext() for additional documentation.
func Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return getCurrentDB().Exec(ctx, query, args...)
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
//
// See (*database/sql.DB).QueryContext() for additional documentation.
func Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return getCurrentDB().Query(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
//
// See (*database/sql.DB).QueryRowContext() for additional documentation.
func QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	return getCurrentDB().QueryRow(ctx, query, args...)
}

// Begin opens a new database transaction.
//
// See (*database/sql.DB).Begin() for additional documentation.
func Begin(ctx context.Context) (*Tx, error) {
	return getCurrentDB().Begin(ctx)
}

// Commit commits the given transaction.
//
// See (*database/sql.Tx).Commit() for additional documentation.
// Deprecated: use tx.Commit() instead.
func Commit(tx *Tx) error {
	return tx.Commit()
}

// Rollback rolls back the given transaction.
//
// See (*database/sql.Tx).Rollback() for additional documentation.
// Deprecated: use tx.Rollback() instead.
func Rollback(tx *Tx) error {
	return tx.Rollback()
}

// ExecTx is like Exec but executes the query in the given transaction.
//
// See (*database/sql.Tx).ExecContext() for additional documentation.
// Deprecated: use tx.Exec() instead.
func ExecTx(tx *Tx, ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.Exec(ctx, query, args...)
}

// QueryTx is like Query but executes the query in the given transaction.
//
// See (*database/sql.Tx).QueryContext() for additional documentation.
// Deprecated: use tx.Query() instead.
func QueryTx(tx *Tx, ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return tx.Query(ctx, query, args...)
}

// QueryRowTx is like QueryRow but executes the query in the given transaction.
//
// See (*database/sql.Tx).QueryRowContext() for additional documentation.
// Deprecated: use tx.QueryRow() instead.
func QueryRowTx(tx *Tx, ctx context.Context, query string, args ...interface{}) *Row {
	return tx.QueryRow(ctx, query, args...)
}

// constStr is a string that can only be provided as a constant.
//publicapigen:keep
type constStr string

// Named returns a database object connected to the database with the given name.
//
// The name must be a string literal constant, to facilitate static analysis.
func Named(name constStr) *Database {
	return Singleton.GetDB(string(name))
}

//publicapigen:drop
var Singleton *Manager

func getCurrentDB() *Database {
	return Singleton.GetCurrentDB()
}
