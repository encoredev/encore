// Package sqldb provides Encore services direct access to their databases.
//
// For the documentation on how to use databases within Encore see https://encore.dev/docs/develop/databases.
package sqldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync/atomic"

	"github.com/jackc/pgx/v4"

	"encore.dev/appruntime/trace"
	"encore.dev/beta/errs"
	"encore.dev/internal/stack"
)

// ErrNoRows is an error reported by Scan when QueryRow doesn't return a row.
// It must be tested against with errors.Is.
var ErrNoRows = sql.ErrNoRows

// ExecResult is the result of an Exec query.
type ExecResult interface {
	// RowsAffected returns the number of rows affected. If the result was not
	// for a row affecting command (e.g. "CREATE TABLE") then it returns 0.
	RowsAffected() int64
}

// Tx is a handle to a database transaction.
//
// See *database/sql.Tx for additional documentation.
type Tx struct {
	mgr  *Manager
	txid uint64
	std  pgx.Tx
}

// Commit commits the given transaction.
//
// See (*database/sql.Tx).Commit() for additional documentation.
func (tx *Tx) Commit() error { return tx.commit() }

// Rollback rolls back the given transaction.
//
// See (*database/sql.Tx).Rollback() for additional documentation.
func (tx *Tx) Rollback() error { return tx.rollback() }

func (tx *Tx) commit() error {
	err := tx.std.Commit(context.Background())
	err = convertErr(err)

	if curr := tx.mgr.rt.Current(); curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTxEnd(trace.DBTxEndParams{
			SpanID: curr.Req.SpanID,
			Goid:   curr.Goctr,
			TxID:   tx.txid,
			Commit: true,
			Err:    err,
			Stack:  stack.Build(4),
		})
	}

	return err
}

func (tx *Tx) rollback() error {
	err := tx.std.Rollback(context.Background())
	err = convertErr(err)

	if curr := tx.mgr.rt.Current(); curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTxEnd(trace.DBTxEndParams{
			SpanID: curr.Req.SpanID,
			Goid:   curr.Goctr,
			TxID:   tx.txid,
			Commit: false,
			Err:    err,
			Stack:  stack.Build(4),
		})
	}

	return err
}

func (tx *Tx) Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.exec(ctx, query, args...)
}

func (tx *Tx) exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	qid := atomic.AddUint64(&tx.mgr.queryCtr, 1)

	curr := tx.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    tx.txid,
			Stack:   stack.Build(4),
		})
	}

	res, err := tx.std.Exec(ctx, query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return res, err
}

func (tx *Tx) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	qid := atomic.AddUint64(&tx.mgr.queryCtr, 1)

	curr := tx.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    tx.txid,
			Stack:   stack.Build(4),
		})
	}

	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func (tx *Tx) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	qid := atomic.AddUint64(&tx.mgr.queryCtr, 1)

	curr := tx.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    tx.txid,
			Stack:   stack.Build(4),
		})
	}

	// pgx currently does not support .Err() on Row.
	// Work around this by using Query.
	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return r
}

// Rows is the result of a query. Its cursor starts before the first row
// of the result set. Use Next to advance from row to row.
//
// See *database/sql.Rows for additional documentation.
type Rows struct {
	std pgx.Rows
}

// Close closes the Rows, preventing further enumeration.
//
// See (*database/sql.Rows).Close() for additional documentation.
func (r *Rows) Close() { r.std.Close() }

// Scan copies the columns in the current row into the values pointed
// at by dest. The number of values in dest must be the same as the
// number of columns in Rows.
//
// See (*database/sql.Rows).Scan() for additional documentation.
func (r *Rows) Scan(dest ...interface{}) error { return r.std.Scan(dest...) }

// Err returns the error, if any, that was encountered during iteration.
// Err may be called after an explicit or implicit Close.
//
// See (*database/sql.Rows).Err() for additional documentation.
func (r *Rows) Err() error { return r.std.Err() }

// Next prepares the next result row for reading with the Scan method. It
// returns true on success, or false if there is no next result row or an error
// happened while preparing it. Err should be consulted to distinguish between
// the two cases.
//
// Every call to Scan, even the first one, must be preceded by a call to Next.
//
// See (*database/sql.Rows).Next() for additional documentation.
func (r *Rows) Next() bool { return r.std.Next() }

// Row is the result of calling QueryRow to select a single row.
//
// See *database/sql.Row for additional documentation.
type Row struct {
	rows pgx.Rows
	err  error
}

// Scan copies the columns from the matched row into the values
// pointed at by dest.
//
// See (*database/sql.Row).Scan() for additional documentation.
func (r *Row) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return convertErr(err)
		}
		return errs.DropStackFrame(errs.WrapCode(sql.ErrNoRows, errs.NotFound, ""))
	}
	r.rows.Scan(dest...)
	r.rows.Close()
	return convertErr(r.rows.Err())
}

func (r *Row) Err() error {
	if r.err != nil {
		return r.err
	}
	return convertErr(r.rows.Err())
}

type interceptor struct {
	mgr *Manager
}

var _ middleware = (*interceptor)(nil)

func (i *interceptor) ConnQuery(ctx context.Context, conn driver.QueryerContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	qid := atomic.AddUint64(&i.mgr.queryCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(5),
		})
	}

	rows, err := conn.QueryContext(ctx, query, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return rows, err
}

func (i *interceptor) ConnExec(ctx context.Context, conn driver.ExecerContext, query string, args []driver.NamedValue) (driver.Result, error) {
	qid := atomic.AddUint64(&i.mgr.queryCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(5),
		})
	}

	res, err := conn.ExecContext(ctx, query, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return res, err
}

func (i *interceptor) StmtQuery(ctx context.Context, conn driver.StmtQueryContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	qid := atomic.AddUint64(&i.mgr.queryCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(5),
		})
	}

	rows, err := conn.QueryContext(ctx, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return rows, err
}

func (i *interceptor) StmtExec(ctx context.Context, conn driver.StmtExecContext, query string, args []driver.NamedValue) (driver.Result, error) {
	qid := atomic.AddUint64(&i.mgr.queryCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace.DBQueryStartParams{
			Query:   query,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(5),
		})
	}

	res, err := conn.ExecContext(ctx, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(qid, err)
	}

	return res, err
}

func (i *interceptor) ConnBegin(tx driver.Tx) (driver.Tx, error) {
	txid := atomic.AddUint64(&i.mgr.txidCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTxStart(trace.DBTxStartParams{
			SpanID: curr.Req.SpanID,
			Goid:   curr.Goctr,
			TxID:   txid,
			Stack:  stack.Build(5),
		})
	}

	return stdlibTx{Tx: tx, txid: txid}, nil
}

func (i *interceptor) ConnBeginTx(ctx context.Context, conn driver.ConnBeginTx, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	txid := atomic.AddUint64(&i.mgr.txidCtr, 1)

	curr := i.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTxStart(trace.DBTxStartParams{
			SpanID: curr.Req.SpanID,
			Goid:   curr.Goctr,
			TxID:   txid,
			Stack:  stack.Build(5),
		})
	}

	return stdlibTx{Tx: tx, txid: txid}, nil
}

type stdlibTx struct {
	driver.Tx
	txid uint64
}

func (i *interceptor) TxCommit(ctx context.Context, tx driver.Tx) error {
	err := tx.Commit()

	if s, ok := tx.(stdlibTx); ok {
		curr := i.mgr.rt.Current()
		if curr.Req != nil && curr.Trace != nil {
			curr.Trace.DBTxEnd(trace.DBTxEndParams{
				SpanID: curr.Req.SpanID,
				Goid:   curr.Goctr,
				TxID:   s.txid,
				Commit: true,
				Err:    err,
				Stack:  stack.Build(5),
			})
		}
	}

	return err
}

func (i *interceptor) TxRollback(ctx context.Context, tx driver.Tx) error {
	err := tx.Rollback()

	if s, ok := tx.(stdlibTx); ok {
		curr := i.mgr.rt.Current()
		if curr.Req != nil && curr.Trace != nil {
			curr.Trace.DBTxEnd(trace.DBTxEndParams{
				SpanID: curr.Req.SpanID,
				Goid:   curr.Goctr,
				TxID:   s.txid,
				Commit: false,
				Err:    err,
				Stack:  stack.Build(5),
			})
		}
	}
	return err
}
