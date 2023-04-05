package sqldb

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5"

	"encore.dev/appruntime/exported/stack"
	trace2 "encore.dev/appruntime/exported/trace"
)

type pgxTracer struct {
	mgr *Manager
}

type ctxKey string

const (
	pgxQueryKey ctxKey = "pgx_query"

	// pgxAlreadyTracedKey is a context key that indicates
	// that the query is already traced through the sqldb integration.
	pgxAlreadyTracedKey ctxKey = "pgx_query"
)

func markTraced(ctx context.Context) context.Context {
	return context.WithValue(ctx, pgxAlreadyTracedKey, true)
}

type queryValue struct {
	trace trace2.Logger
	qid   uint64
}

func (t *pgxTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if ctx.Value(pgxAlreadyTracedKey) != nil {
		return ctx
	}

	qid := atomic.AddUint64(&t.mgr.queryCtr, 1)

	curr := t.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			Query:   data.SQL,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			QueryID: qid,
			TxID:    0,
			Stack:   stack.Build(5),
		})
		ctx = context.WithValue(ctx, pgxQueryKey, &queryValue{trace: curr.Trace, qid: qid})
	}
	return ctx
}

func (t *pgxTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if qv, ok := ctx.Value(pgxQueryKey).(*queryValue); ok {
		qv.trace.DBQueryEnd(qv.qid, data.Err)
	}
}

var (
	_ pgx.QueryTracer = (*pgxTracer)(nil)
	_ pgx.QueryTracer = (*pgxTracer)(nil)
)
