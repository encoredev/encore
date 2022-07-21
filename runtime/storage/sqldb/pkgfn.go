//go:build encore_app

package sqldb

import (
	"context"
)

func Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return getCurrentDB().Exec(ctx, query, args...)
}

func Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return getCurrentDB().Query(ctx, query, args...)
}

func QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	return getCurrentDB().QueryRow(ctx, query, args...)
}

func Begin(ctx context.Context) (*Tx, error) {
	return getCurrentDB().Begin(ctx)
}

func Commit(tx *Tx) error {
	return tx.Commit()
}

func Rollback(tx *Tx) error {
	return tx.Rollback()
}

func ExecTx(tx *Tx, ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.Exec(ctx, query, args...)
}

func QueryTx(tx *Tx, ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return tx.Query(ctx, query, args...)
}

func QueryRowTx(tx *Tx, ctx context.Context, query string, args ...interface{}) *Row {
	return tx.QueryRow(ctx, query, args...)
}

type constStr string

func Named(name constStr) *Database {
	return Singleton.GetDB(string(name))
}

var Singleton *Manager

func getCurrentDB() *Database {
	return Singleton.GetCurrentDB()
}
