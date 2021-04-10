package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"

	"encore.dev/runtime"
	"encore.dev/runtime/config"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	txidCounter  uint64
	queryCounter uint64
)

// ExecResult is the result of an Exec query.
type ExecResult interface {
	// RowsAffected returns the number of rows affected. If the result was not
	// for a row affecting command (e.g. "CREATE TABLE") then it returns 0.
	RowsAffected() int64
}

func Exec(traceExpr int32, svc string, ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(0) // no tx
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	res, err := getDB(svc).Exec(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	return res, err
}

func Query(traceExpr int32, svc string, ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(0) // no tx
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	rows, err := getDB(svc).Query(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func QueryRow(traceExpr int32, svc string, ctx context.Context, query string, args ...interface{}) *Row {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(0) // no tx
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	rows, err := getDB(svc).Query(ctx, query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err := r.Err(); err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	return r
}

type Tx struct {
	txid uint64
	std  pgx.Tx
}

func Begin(traceExpr int32, svc string, ctx context.Context) (*Tx, error) {
	tx, err := getDB(svc).Begin(ctx)
	err = convertErr(err)
	if err != nil {
		return nil, err
	}
	txid := atomic.AddUint64(&txidCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(txid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		runtime.TraceLog(runtime.TxStart, tb.Buf())
	}

	return &Tx{txid: txid, std: tx}, nil
}

func Commit(traceExpr int32, svc string, tx *Tx) error {
	err := tx.std.Commit(context.Background())
	err = convertErr(err)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(tx.txid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(uint64(goid))
		tb.Bytes([]byte{1})
		tb.UVarint(uint64(traceExpr))
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.TxEnd, tb.Buf())
	}
	return err
}

func Rollback(traceExpr int32, svc string, tx *Tx) error {
	err := tx.std.Rollback(context.Background())
	err = convertErr(err)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(tx.txid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(uint64(goid))
		tb.Bytes([]byte{0})
		tb.UVarint(uint64(traceExpr))
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.TxEnd, tb.Buf())
	}
	return err
}

func ExecTx(traceExpr int32, svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(tx.txid)
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	res, err := tx.std.Exec(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	return res, err
}

func QueryTx(traceExpr int32, svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(tx.txid)
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func QueryRowTx(traceExpr int32, svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) *Row {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		tb.Bytes(req.SpanID[:])
		tb.UVarint(tx.txid)
		tb.UVarint(uint64(goid))
		tb.UVarint(uint64(traceExpr))
		tb.String(query)
		runtime.TraceLog(runtime.QueryStart, tb.Buf())
	}

	// pgx currently does not support .Err() on Row.
	// Work around this by using Query.
	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if req != nil && req.Traced {
		var tb runtime.TraceBuf
		tb.UVarint(qid)
		if err := r.Err(); err != nil {
			tb.String(err.Error())
		} else {
			tb.String("")
		}
		runtime.TraceLog(runtime.QueryEnd, tb.Buf())
	}

	return r
}

type Rows struct {
	std pgx.Rows
}

func (r *Rows) Close()                         { r.std.Close() }
func (r *Rows) Scan(dest ...interface{}) error { return r.std.Scan(dest...) }
func (r *Rows) Err() error                     { return r.std.Err() }
func (r *Rows) Next() bool                     { return r.std.Next() }

type Row struct {
	rows pgx.Rows
	err  error
}

func (r *Row) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return convertErr(err)
		}
		return sql.ErrNoRows
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

var dbMap atomic.Value

func setDBs(dbs map[string]*pgxpool.Pool) {
	dbMap.Store(dbs)
}

func getDB(svc string) *pgxpool.Pool {
	dbs, ok := dbMap.Load().(map[string]*pgxpool.Pool)
	if !ok {
		panic("sqldb: service not initialized: " + svc)
	}
	db, ok := dbs[svc]
	if !ok {
		panic("sqldb: could not find database for service " + svc)
	}
	return db
}

func Setup(cfg *config.ServerConfig) {
	addr := os.Getenv("ENCORE_SQLDB_ADDRESS")
	passwd := os.Getenv("ENCORE_SQLDB_PASSWORD")
	dbs := make(map[string]*pgxpool.Pool)
	for _, svc := range cfg.Services {
		if svc.SQLDB {
			if addr == "" {
				panic("sqldb: ENCORE_SQLDB_ADDRESS not set")
			}

			uri := fmt.Sprintf("postgresql://encore:%s@%s/%s?sslmode=disable",
				passwd, addr, svc.Name)
			cfg, err := pgxpool.ParseConfig(uri)
			if err != nil {
				panic("sqldb: invalid database uri: " + err.Error())
			}
			cfg.LazyConnect = true
			cfg.MaxConns = 30
			pool, err := pgxpool.ConnectConfig(context.Background(), cfg)
			if err != nil {
				panic("sqldb: setup db: " + err.Error())
			}
			dbs[svc.Name] = pool
		}
	}
	setDBs(dbs)
}

func convertErr(err error) error {
	switch err {
	case pgx.ErrNoRows:
		return sql.ErrNoRows
	default:
		return err
	}
}
