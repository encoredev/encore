//go:build encore_app

package sqldb

import (
	"context"

	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

// NewDatabase declares a new SQL database.
//
// Encore uses static analysis to identify databases and their configuration,
// so all parameters passed to this function must be constant literals.
//
// A call to NewDatabase can only be made when declaring a package level variable. Any
// calls to this function made outside a package level variable declaration will result
// in a compiler error.
//
// The database name must be unique within the Encore application. Database names must be defined
// in kebab-case (lowercase alphanumerics and hyphen seperated). Once created and deployed never
// change the database name, or else a new database will be created.
func NewDatabase(name string, config DatabaseConfig) *Database {
	return Singleton.GetDB(name)
}

// DatabaseConfig specifies configuration for declaring a new database.
type DatabaseConfig struct {
	// Migrations is the directory containing the migration files
	// for this database.
	//
	// The path must be slash-separated relative path, and must be rooted within
	// the package directory (it cannot contain "../").
	// Valid paths are, for example, "migrations" or "db/migrations".
	//
	// Migrations are an ordered sequence of sql files of the format <number>_<description>.up.sql.
	Migrations string
}

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
//
//publicapigen:keep
type constStr string

// Named returns a database object connected to the database with the given name.
//
// The name must be a string literal constant, to facilitate static analysis.
func Named(name constStr) *Database {
	return Singleton.GetDB(string(name))
}

//publicapigen:drop
var Singleton = NewManager(appconf.Runtime, reqtrack.Singleton, testsupport.Singleton)

func getCurrentDB() *Database {
	return Singleton.GetCurrentDB()
}
