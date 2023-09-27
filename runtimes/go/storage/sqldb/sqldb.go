// Package sqldb provides Encore services direct access to their databases.
//
// For the documentation on how to use databases within Encore see https://encore.dev/docs/develop/databases.
package sqldb

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/beta/errs"
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
	mgr *Manager
	std pgx.Tx

	startID model.TraceEventID
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
	err := tx.std.Commit(markTraced(context.Background()))
	err = convertErr(err)

	if curr := tx.mgr.rt.Current(); curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTransactionEnd(trace2.DBTransactionEndParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
				DefLoc:  0,
			},
			StartID: tx.startID,
			Commit:  true,
			Err:     err,
			Stack:   stack.Build(4),
		})
	}

	return err
}

func (tx *Tx) rollback() error {
	err := tx.std.Rollback(markTraced(context.Background()))
	err = convertErr(err)

	if curr := tx.mgr.rt.Current(); curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBTransactionEnd(trace2.DBTransactionEndParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
				DefLoc:  0,
			},
			StartID: tx.startID,
			Commit:  false,
			Err:     err,
			Stack:   stack.Build(4),
		})
	}

	return err
}

func (tx *Tx) Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.exec(ctx, query, args...)
}

func (tx *Tx) exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	curr := tx.mgr.rt.Current()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

	if curr.Req != nil && curr.Trace != nil {
		eventParams = trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			TxStartID:   tx.startID,
			Query:       query,
			Stack:       stack.Build(4),
		})
	}

	res, err := tx.std.Exec(markTraced(ctx), query, args...)
	err = convertErr(err)

	if startEventID > 0 {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return res, err
}

func (tx *Tx) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	curr := tx.mgr.rt.Current()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

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
			TxStartID:   tx.startID,
			Stack:       stack.Build(4),
		})
	}

	rows, err := tx.std.Query(markTraced(ctx), query, args...)
	err = convertErr(err)

	if startEventID > 0 {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func (tx *Tx) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	curr := tx.mgr.rt.Current()

	var (
		startEventID model.TraceEventID
		eventParams  trace2.EventParams
	)

	if curr.Req != nil && curr.Trace != nil {
		eventParams = trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			Query:       query,
			TxStartID:   tx.startID,
			Stack:       stack.Build(4),
		})
	}

	// pgx currently does not support .Err() on Row.
	// Work around this by using Query.
	rows, err := tx.std.Query(markTraced(ctx), query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if startEventID > 0 {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
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
