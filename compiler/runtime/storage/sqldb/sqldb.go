package sqldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"encore.dev/beta/errs"
	"encore.dev/internal/stack"
	"encore.dev/runtime"
	"encore.dev/runtime/config"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgx/v4/stdlib"
)

var (
	txidCounter  uint64
	queryCounter uint64
)

// An error satisfying ErrNoRows is reported by Scan
// when QueryRow doesn't return a row.
// It must be tested against with errors.Is.
var ErrNoRows = sql.ErrNoRows

// ExecResult is the result of an Exec query.
type ExecResult interface {
	// RowsAffected returns the number of rows affected. If the result was not
	// for a row affecting command (e.g. "CREATE TABLE") then it returns 0.
	RowsAffected() int64
}

func Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return getDB().exec(ctx, query, args...)
}

func Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return getDB().query(ctx, query, args...)
}

func QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	return getDB().queryRow(ctx, query, args...)
}

type Tx struct {
	txid uint64
	std  pgx.Tx
}

func Begin(ctx context.Context) (*Tx, error) {
	return getDB().begin(ctx)
}

func Commit(tx *Tx) error {
	return tx.Commit()
}

func Rollback(tx *Tx) error {
	return tx.rollback()
}

func (tx *Tx) Commit() error { return tx.commit() }

func (tx *Tx) Rollback() error { return tx.rollback() }

func (tx *Tx) commit() error {
	err := tx.std.Commit(context.Background())
	err = convertErr(err)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceCompleteTxEnd(req.SpanID, uint64(goid), tx.txid, true, err, 4)
	}
	return err
}

func (tx *Tx) rollback() error {
	err := tx.std.Rollback(context.Background())
	err = convertErr(err)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceCompleteTxEnd(req.SpanID, uint64(goid), tx.txid, false, err, 4)
	}
	return err
}

func ExecTx(svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.exec(ctx, query, args...)
}

func (tx *Tx) Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return tx.exec(ctx, query, args...)
}

func (tx *Tx) exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, tx.txid, 4)
	}

	res, err := tx.std.Exec(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}

	return res, err
}

func QueryTx(svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return tx.query(ctx, query, args...)
}

func (tx *Tx) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return tx.query(ctx, query, args...)
}

func (tx *Tx) query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, tx.txid, 4)
	}

	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func QueryRowTx(svc string, tx *Tx, ctx context.Context, query string, args ...interface{}) *Row {
	return tx.queryRow(ctx, query, args...)
}

func (tx *Tx) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	return tx.queryRow(ctx, query, args...)
}

func (tx *Tx) queryRow(ctx context.Context, query string, args ...interface{}) *Row {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, tx.txid, 4)
	}

	// pgx currently does not support .Err() on Row.
	// Work around this by using Query.
	rows, err := tx.std.Query(ctx, query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if req != nil && req.Traced {
		traceQueryEnd(qid, r.Err())
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

var (
	dbMu  sync.RWMutex
	dbMap = make(map[string]*Database)
)

func getDB() *Database {
	req, _, _ := runtime.CurrentRequest()
	if req == nil {
		panic("sqldb: no current request")
	}
	name := req.Service

	dbMu.RLock()
	db, ok := dbMap[name]
	dbMu.RUnlock()
	if ok {
		return db
	}

	dbMu.Lock()
	defer dbMu.Unlock()
	db = &Database{name: name, pool: getPool(name)}
	dbMap[name] = db
	return db
}

func getPool(name string) *pgxpool.Pool {
	addr := os.Getenv("ENCORE_SQLDB_ADDRESS")
	passwd := os.Getenv("ENCORE_SQLDB_PASSWORD")
	if addr == "" {
		panic("sqldb: ENCORE_SQLDB_ADDRESS not set")
	}

	uri := fmt.Sprintf("postgresql://encore:%s@%s/%s?sslmode=disable", passwd, addr, name)
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
	return pool
}

func Setup(cfg *config.ServerConfig) {}

func convertErr(err error) error {
	switch err {
	case pgx.ErrNoRows, sql.ErrNoRows:
		err = errs.WrapCode(sql.ErrNoRows, errs.NotFound, "")
	case pgx.ErrTxClosed, pgx.ErrInvalidLogLevel, pgx.ErrTxCommitRollback, sql.ErrTxDone, sql.ErrConnDone:
		err = errs.WrapCode(err, errs.Internal, "")
	default:
		err = errs.WrapCode(err, errs.Unavailable, "")
	}
	return errs.DropStackFrame(err)
}

type constStr string

func Named(name constStr) *Database {
	return &Database{name: string(name)}
}

type Database struct {
	name string

	initOnce sync.Once
	pool     *pgxpool.Pool
	connStr  string

	stdlibOnce sync.Once
	stdlib     *sql.DB
}

func (db *Database) Exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	return db.exec(ctx, query, args...)
}

func (db *Database) exec(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	db.init()
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 4)
	}

	res, err := db.pool.Exec(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}

	return res, err
}

func (db *Database) Query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	return db.query(ctx, query, args...)
}

func (db *Database) QueryRow(ctx context.Context, query string, args ...interface{}) *Row {
	return db.queryRow(ctx, query, args...)
}

func (db *Database) query(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	db.init()
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 4)
	}

	rows, err := db.pool.Query(ctx, query, args...)
	err = convertErr(err)

	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}

	if err != nil {
		return nil, err
	}
	return &Rows{std: rows}, nil
}

func (db *Database) queryRow(ctx context.Context, query string, args ...interface{}) *Row {
	db.init()
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 4)
	}

	rows, err := db.pool.Query(ctx, query, args...)
	err = convertErr(err)
	r := &Row{rows: rows, err: err}

	if req != nil && req.Traced {
		traceQueryEnd(qid, r.Err())
	}

	return r
}

func (db *Database) Begin(ctx context.Context) (*Tx, error) {
	return db.begin(ctx)
}

func (db *Database) begin(ctx context.Context) (*Tx, error) {
	db.init()
	tx, err := db.pool.Begin(ctx)
	err = convertErr(err)
	if err != nil {
		return nil, err
	}
	txid := atomic.AddUint64(&txidCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceBeginTxEnd(req.SpanID, uint64(goid), txid, 4)
	}

	return &Tx{txid: txid, std: tx}, nil
}

func (db *Database) init() {
	db.initOnce.Do(func() {
		if db.pool == nil {
			db.pool = getPool(db.name)
		}
		db.connStr = stdlib.RegisterConnConfig(db.pool.Config().ConnConfig)
	})
}

func (db *Database) Stdlib() *sql.DB {
	db.init()
	registerDriver.Do(func() {
		stdlibDriver = &wrappedDriver{parent: stdlib.GetDefaultDriver(), mw: &interceptor{}}
		sql.Register(driverName, stdlibDriver)
	})

	var openErr error
	db.stdlibOnce.Do(func() {
		c, err := stdlibDriver.(driver.DriverContext).OpenConnector(db.connStr)
		if err == nil {
			db.stdlib = sql.OpenDB(c)
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

type interceptor struct{}

var _ middleware = (*interceptor)(nil)

func (*interceptor) ConnQuery(ctx context.Context, conn driver.QueryerContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 5)
	}
	rows, err := conn.QueryContext(ctx, query, args)
	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}
	return rows, err
}

func (*interceptor) ConnExec(ctx context.Context, conn driver.ExecerContext, query string, args []driver.NamedValue) (driver.Result, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 5)
	}
	res, err := conn.ExecContext(ctx, query, args)
	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}
	return res, err
}

func (*interceptor) StmtQuery(ctx context.Context, conn driver.StmtQueryContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 5)
	}
	rows, err := conn.QueryContext(ctx, args)
	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}
	return rows, err
}

func (*interceptor) StmtExec(ctx context.Context, conn driver.StmtExecContext, query string, args []driver.NamedValue) (driver.Result, error) {
	qid := atomic.AddUint64(&queryCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceQueryStart(query, req.SpanID, uint64(goid), qid, 0, 5)
	}
	res, err := conn.ExecContext(ctx, args)
	if req != nil && req.Traced {
		traceQueryEnd(qid, err)
	}
	return res, err
}

func (*interceptor) ConnBegin(tx driver.Tx) (driver.Tx, error) {
	txid := atomic.AddUint64(&txidCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceBeginTxEnd(req.SpanID, uint64(goid), txid, 5)
	}
	return stdlibTx{Tx: tx, txid: txid}, nil
}

func (*interceptor) ConnBeginTx(ctx context.Context, conn driver.ConnBeginTx, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	txid := atomic.AddUint64(&txidCounter, 1)
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		traceBeginTxEnd(req.SpanID, uint64(goid), txid, 5)
	}
	return stdlibTx{Tx: tx, txid: txid}, nil
}

type stdlibTx struct {
	driver.Tx
	txid uint64
}

func (*interceptor) TxCommit(ctx context.Context, tx driver.Tx) error {
	err := tx.Commit()
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		if s, ok := tx.(stdlibTx); ok {
			traceCompleteTxEnd(req.SpanID, uint64(goid), s.txid, true, err, 5)
		}
	}
	return err
}

func (*interceptor) TxRollback(ctx context.Context, tx driver.Tx) error {
	err := tx.Rollback()
	req, goid, _ := runtime.CurrentRequest()
	if req != nil && req.Traced {
		if s, ok := tx.(stdlibTx); ok {
			traceCompleteTxEnd(req.SpanID, uint64(goid), s.txid, false, err, 5)
		}
	}
	return err
}

var (
	registerDriver sync.Once
	stdlibDriver   driver.Driver
)

const driverName = "__encore_stdlib"

func traceQueryStart(query string, spanID runtime.SpanID, goid, qid, txid uint64, skipFrames int) {
	var tb runtime.TraceBuf
	tb.UVarint(qid)
	tb.Bytes(spanID[:])
	tb.UVarint(txid)
	tb.UVarint(goid)
	tb.String(query)
	tb.Stack(stack.Build(skipFrames))
	runtime.TraceLog(runtime.QueryStart, tb.Buf())
}

func traceQueryEnd(qid uint64, err error) {
	var tb runtime.TraceBuf
	tb.UVarint(qid)
	if err != nil {
		tb.String(err.Error())
	} else {
		tb.String("")
	}
	runtime.TraceLog(runtime.QueryEnd, tb.Buf())
}

func traceBeginTxEnd(spanID runtime.SpanID, goid, txid uint64, skipFrames int) {
	var tb runtime.TraceBuf
	tb.UVarint(txid)
	tb.Bytes(spanID[:])
	tb.UVarint(goid)
	tb.Stack(stack.Build(skipFrames))
	runtime.TraceLog(runtime.TxStart, tb.Buf())
}

func traceCompleteTxEnd(spanID runtime.SpanID, goid, txid uint64, commit bool, err error, skipFrames int) {
	var tb runtime.TraceBuf
	tb.UVarint(txid)
	tb.Bytes(spanID[:])
	tb.UVarint(uint64(goid))
	if commit {
		tb.Bytes([]byte{1})
	} else {
		tb.Bytes([]byte{0})
	}
	if err != nil {
		tb.String(err.Error())
	} else {
		tb.String("")
	}
	tb.Stack(stack.Build(skipFrames))
	runtime.TraceLog(runtime.TxEnd, tb.Buf())
}
