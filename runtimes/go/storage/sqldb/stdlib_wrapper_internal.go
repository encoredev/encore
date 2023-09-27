/*

This file is adapted from of github.com/ngrok/sqlmw, with license:

MIT License

Copyright (c) 2017 Expansive Worlds
Copyright (c) 2017 Avalanche Studios
Copyright (c) 2020 Alan Shreve

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

package sqldb

import (
	"context"
	"database/sql/driver"
	"errors"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
)

type middleware interface {
	ConnExec(context.Context, driver.ExecerContext, string, []driver.NamedValue) (driver.Result, error)
	ConnQuery(context.Context, driver.QueryerContext, string, []driver.NamedValue) (driver.Rows, error)
	StmtExec(context.Context, driver.StmtExecContext, string, []driver.NamedValue) (driver.Result, error)
	StmtQuery(context.Context, driver.StmtQueryContext, string, []driver.NamedValue) (driver.Rows, error)

	ConnBegin(tx driver.Tx) (driver.Tx, error)
	TxCommit(context.Context, driver.Tx) error
	TxRollback(context.Context, driver.Tx) error
}

type interceptor struct {
	mgr *Manager
}

var _ middleware = (*interceptor)(nil)

func (i *interceptor) ConnQuery(ctx context.Context, conn driver.QueryerContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	curr := i.mgr.rt.Current()

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
			Stack:       stack.Build(5),
		})
	}

	rows, err := conn.QueryContext(ctx, query, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return rows, err
}

func (i *interceptor) ConnExec(ctx context.Context, conn driver.ExecerContext, query string, args []driver.NamedValue) (driver.Result, error) {
	curr := i.mgr.rt.Current()

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
			Stack:       stack.Build(5),
		})
	}

	res, err := conn.ExecContext(ctx, query, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return res, err
}

func (i *interceptor) StmtQuery(ctx context.Context, conn driver.StmtQueryContext, query string, args []driver.NamedValue) (driver.Rows, error) {
	curr := i.mgr.rt.Current()

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
			Stack:       stack.Build(5),
		})
	}

	rows, err := conn.QueryContext(ctx, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return rows, err
}

func (i *interceptor) StmtExec(ctx context.Context, conn driver.StmtExecContext, query string, args []driver.NamedValue) (driver.Result, error) {
	curr := i.mgr.rt.Current()

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
			Stack:       stack.Build(5),
		})
	}

	res, err := conn.ExecContext(ctx, args)

	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryEnd(eventParams, startEventID, err)
	}

	return res, err
}

func (i *interceptor) ConnBegin(tx driver.Tx) (driver.Tx, error) {
	curr := i.mgr.rt.Current()

	var startEventID model.TraceEventID

	if curr.Req != nil && curr.Trace != nil {
		eventParams := trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBTransactionStart(eventParams, stack.Build(5))
	}

	return stdlibTx{Tx: tx, startID: startEventID}, nil
}

func (i *interceptor) ConnBeginTx(ctx context.Context, conn driver.ConnBeginTx, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	curr := i.mgr.rt.Current()

	var startEventID model.TraceEventID

	if curr.Req != nil && curr.Trace != nil {
		eventParams := trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startEventID = curr.Trace.DBTransactionStart(eventParams, stack.Build(5))
	}

	return stdlibTx{Tx: tx, startID: startEventID}, nil
}

type stdlibTx struct {
	driver.Tx

	startID model.TraceEventID
}

func (i *interceptor) TxCommit(ctx context.Context, tx driver.Tx) error {
	err := tx.Commit()

	if s, ok := tx.(stdlibTx); ok {
		curr := i.mgr.rt.Current()
		if curr.Req != nil && curr.Trace != nil {
			eventParams := trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
				DefLoc:  0,
			}
			curr.Trace.DBTransactionEnd(trace2.DBTransactionEndParams{
				EventParams: eventParams,
				StartID:     s.startID,
				Commit:      true,
				Err:         err,
				Stack:       stack.Build(5),
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
			eventParams := trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
				DefLoc:  0,
			}
			curr.Trace.DBTransactionEnd(trace2.DBTransactionEndParams{
				EventParams: eventParams,
				StartID:     s.startID,
				Commit:      false,
				Err:         err,
				Stack:       stack.Build(5),
			})
		}
	}
	return err
}

type wrappedDriver struct {
	parent driver.Driver
	mw     middleware
}

var (
	_ driver.Driver        = wrappedDriver{}
	_ driver.DriverContext = wrappedDriver{}
)

func (d wrappedDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.parent.Open(name)
	if err != nil {
		return nil, err
	}
	return wrappedConn{mw: d.mw, parent: conn}, nil
}

func (d wrappedDriver) OpenConnector(name string) (driver.Connector, error) {
	driver, ok := d.parent.(driver.DriverContext)
	if !ok {
		return wrappedConnector{
			parent:    dsnConnector{dsn: name, driver: d.parent},
			driverRef: &d,
		}, nil
	}
	conn, err := driver.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return wrappedConnector{parent: conn, driverRef: &d}, nil
}

type wrappedConnector struct {
	parent    driver.Connector
	driverRef *wrappedDriver
}

var (
	_ driver.Connector = wrappedConnector{}
)

func (c wrappedConnector) Connect(ctx context.Context) (conn driver.Conn, err error) {
	conn, err = c.parent.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return wrappedConn{mw: c.driverRef.mw, parent: conn}, nil
}

func (c wrappedConnector) Driver() driver.Driver {
	return c.driverRef
}

// dsnConnector is a fallback connector placed in position of wrappedConnector.parent
// when given Driver does not comply with DriverContext interface.
type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (t dsnConnector) Connect(_ context.Context) (driver.Conn, error) {
	return t.driver.Open(t.dsn)
}

func (t dsnConnector) Driver() driver.Driver {
	return t.driver
}

type wrappedConn struct {
	mw     middleware
	parent driver.Conn
}

// Compile time validation that our types implement the expected interfaces
var (
	_ driver.Conn               = wrappedConn{}
	_ driver.ConnBeginTx        = wrappedConn{}
	_ driver.ConnPrepareContext = wrappedConn{}
	_ driver.ExecerContext      = wrappedConn{}
	_ driver.Pinger             = wrappedConn{}
	_ driver.QueryerContext     = wrappedConn{}
	_ driver.SessionResetter    = wrappedConn{}
	_ driver.NamedValueChecker  = wrappedConn{}
)

func (c wrappedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.parent.Prepare(query)
	if err != nil {
		return nil, err
	}
	return wrappedStmt{mw: c.mw, query: query, parent: stmt, conn: c}, nil
}

func (c wrappedConn) Close() error {
	return c.parent.Close()
}

func (c wrappedConn) Begin() (tx driver.Tx, err error) {
	tx, err = c.parent.Begin()
	if err != nil {
		return nil, err
	}
	tx, err = c.mw.ConnBegin(tx)
	if err != nil {
		return nil, err
	}
	return wrappedTx{mw: c.mw, parent: tx}, nil
}

func (c wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	wrappedParent := wrappedParentConn{c.parent}
	tx, err = wrappedParent.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	tx, err = c.mw.ConnBegin(tx)
	if err != nil {
		return nil, err
	}
	return wrappedTx{mw: c.mw, ctx: ctx, parent: tx}, nil
}

func (c wrappedConn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	wrappedParent := wrappedParentConn{c.parent}
	stmt, err = wrappedParent.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return wrappedStmt{mw: c.mw, ctx: ctx, query: query, parent: stmt, conn: c}, nil
}

func (c wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (r driver.Result, err error) {
	wrappedParent := wrappedParentConn{c.parent}
	return c.mw.ConnExec(ctx, wrappedParent, query, args)
}

func (c wrappedConn) Ping(ctx context.Context) (err error) {
	if pinger, ok := c.parent.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	// Quick skip path: If the wrapped connection implements neither QueryerContext nor Queryer, we have absolutely nothing to do
	_, hasQueryerContext := c.parent.(driver.QueryerContext)
	_, hasQueryer := c.parent.(driver.Queryer)
	if !hasQueryerContext && !hasQueryer {
		return nil, driver.ErrSkip
	}
	wrappedParent := wrappedParentConn{c.parent}
	return c.mw.ConnQuery(ctx, wrappedParent, query, args)
}

func (c wrappedConn) ResetSession(ctx context.Context) error {
	if conn, ok := c.parent.(driver.SessionResetter); ok {
		return conn.ResetSession(ctx)
	}
	return nil
}

func defaultCheckNamedValue(nv *driver.NamedValue) (err error) {
	nv.Value, err = driver.DefaultParameterConverter.ConvertValue(nv.Value)
	return err
}

func (c wrappedConn) CheckNamedValue(v *driver.NamedValue) error {
	if checker, ok := c.parent.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(v)
	}

	return defaultCheckNamedValue(v)
}

type wrappedParentConn struct {
	driver.Conn
}

func (c wrappedParentConn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	if connBeginTx, ok := c.Conn.(driver.ConnBeginTx); ok {
		return connBeginTx.BeginTx(ctx, opts)
	}
	// Fallback implementation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return c.Conn.Begin()
	}
}

func (c wrappedParentConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if connPrepareCtx, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return connPrepareCtx.PrepareContext(ctx, query)
	}
	// Fallback implementation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return c.Conn.Prepare(query)
	}
}

func (c wrappedParentConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (r driver.Result, err error) {
	if execContext, ok := c.Conn.(driver.ExecerContext); ok {
		return execContext.ExecContext(ctx, query, args)
	}
	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return c.Conn.(driver.Execer).Exec(query, dargs)
	}
}

func (c wrappedParentConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	if queryerContext, ok := c.Conn.(driver.QueryerContext); ok {
		return queryerContext.QueryContext(ctx, query, args)
	}
	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return c.Conn.(driver.Queryer).Query(query, dargs)
	}
}

// namedValueToValue is a helper function copied from the database/sql package
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}

type wrappedStmt struct {
	mw     middleware
	ctx    context.Context
	query  string
	parent driver.Stmt
	conn   wrappedConn
}

// Compile time validation that our types implement the expected interfaces
var (
	_ driver.Stmt              = wrappedStmt{}
	_ driver.StmtExecContext   = wrappedStmt{}
	_ driver.StmtQueryContext  = wrappedStmt{}
	_ driver.ColumnConverter   = wrappedStmt{}
	_ driver.NamedValueChecker = wrappedStmt{}
)

func (s wrappedStmt) Close() (err error) {
	return s.parent.Close()
}

func (s wrappedStmt) NumInput() int {
	return s.parent.NumInput()
}

func (s wrappedStmt) Exec(args []driver.Value) (res driver.Result, err error) {
	return s.parent.Exec(args)
}

func (s wrappedStmt) Query(args []driver.Value) (rows driver.Rows, err error) {
	return s.parent.Query(args)
}

func (s wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (res driver.Result, err error) {
	wrappedParent := wrappedParentStmt{Stmt: s.parent}
	return s.mw.StmtExec(ctx, wrappedParent, s.query, args)
}

func (s wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	wrappedParent := wrappedParentStmt{Stmt: s.parent}
	return s.mw.StmtQuery(ctx, wrappedParent, s.query, args)
}

func (s wrappedStmt) ColumnConverter(idx int) driver.ValueConverter {
	if converter, ok := s.parent.(driver.ColumnConverter); ok {
		return converter.ColumnConverter(idx)
	}

	return driver.DefaultParameterConverter
}

func (s wrappedStmt) CheckNamedValue(v *driver.NamedValue) error {
	if checker, ok := s.parent.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(v)
	}

	if checker, ok := s.conn.parent.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(v)
	}

	return defaultCheckNamedValue(v)
}

type wrappedParentStmt struct {
	driver.Stmt
}

func (s wrappedParentStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	if stmtQueryContext, ok := s.Stmt.(driver.StmtQueryContext); ok {
		return stmtQueryContext.QueryContext(ctx, args)
	}
	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.Stmt.Query(dargs)
}

func (s wrappedParentStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (res driver.Result, err error) {
	if stmtExecContext, ok := s.Stmt.(driver.StmtExecContext); ok {
		return stmtExecContext.ExecContext(ctx, args)
	}
	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.Stmt.Exec(dargs)
}

type wrappedTx struct {
	mw     middleware
	ctx    context.Context
	parent driver.Tx
}

// Compile time validation that our types implement the expected interfaces
var (
	_ driver.Tx = wrappedTx{}
)

func (t wrappedTx) Commit() (err error) {
	return t.mw.TxCommit(t.ctx, t.parent)
}

func (t wrappedTx) Rollback() (err error) {
	return t.mw.TxRollback(t.ctx, t.parent)
}
