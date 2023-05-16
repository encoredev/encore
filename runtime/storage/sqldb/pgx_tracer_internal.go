package sqldb

import (
	"context"

	"github.com/jackc/pgx/v5"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
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
	trace       trace2.Logger
	eventParams trace2.EventParams
	startID     model.TraceEventID
}

func (t *pgxTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if ctx.Value(pgxAlreadyTracedKey) != nil {
		return ctx
	}

	curr := t.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		eventParams := trace2.EventParams{
			TraceID: curr.Req.TraceID,
			SpanID:  curr.Req.SpanID,
			Goid:    curr.Goctr,
			DefLoc:  0,
		}
		startID := curr.Trace.DBQueryStart(trace2.DBQueryStartParams{
			EventParams: eventParams,
			Query:       data.SQL,
			Stack:       stack.Build(5),
		})
		ctx = context.WithValue(ctx, pgxQueryKey, &queryValue{
			trace:       curr.Trace,
			eventParams: eventParams,
			startID:     startID,
		})
	}
	return ctx
}

func (t *pgxTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if qv, ok := ctx.Value(pgxQueryKey).(*queryValue); ok {
		qv.trace.DBQueryEnd(qv.eventParams, qv.startID, data.Err)
	}
}

var (
	_ pgx.QueryTracer = (*pgxTracer)(nil)
	_ pgx.QueryTracer = (*pgxTracer)(nil)
)
