// Package rlog provides a simple logging interface which is integrated with Encore's
// inbuilt distributed tracing.
//
// For more information about logging inside Encore applications see https://encore.dev/docs/observability/logging.
package rlog

import (
	"strings"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/types/uuid"
)

// InternalKeyPrefix is the prefix of log field keys that are reserved for
// internal use only. Log fields starting with this value have an additional "x_"
// prefix prepended to avoid interference with reserved names.
//
//publicapigen:drop
const InternalKeyPrefix = "encore_"

//publicapigen:drop
type Manager struct {
	rt *reqtrack.RequestTracker
}

//publicapigen:drop
func NewManager(rt *reqtrack.RequestTracker) *Manager {
	return &Manager{rt}
}

// Ctx holds additional logging context for use with the Infoc and family
// of logging functions.
type Ctx struct {
	ctx    zerolog.Context
	mgr    *Manager
	fields []any
}

func (l *Manager) Debug(msg string, keysAndValues ...any) {
	fields := pairs(keysAndValues)
	l.doLog(model.LevelDebug, l.rt.Logger().Debug(), msg, nil, fields)
}

func (l *Manager) Info(msg string, keysAndValues ...any) {
	fields := pairs(keysAndValues)
	l.doLog(model.LevelInfo, l.rt.Logger().Info(), msg, nil, fields)
}

func (l *Manager) Warn(msg string, keysAndValues ...any) {
	fields := pairs(keysAndValues)
	l.doLog(model.LevelWarn, l.rt.Logger().Warn(), msg, nil, fields)
}

func (l *Manager) Error(msg string, keysAndValues ...any) {
	fields := pairs(keysAndValues)
	l.doLog(model.LevelError, l.rt.Logger().Error(), msg, nil, fields)
}

func (l *Manager) With(keysAndValues ...any) Ctx {
	ctx := l.rt.Logger().With()
	fields := pairs(keysAndValues)
	for i := 0; i < len(fields); i += 2 {
		key := fields[i].(string)
		val := fields[i+1]
		ctx = addContext(ctx, key, val)
	}
	return Ctx{ctx: ctx, mgr: l, fields: fields}
}

// Debug logs a debug-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Debug(msg string, keysAndValues ...any) {
	l := ctx.ctx.Logger()
	fields := pairs(keysAndValues)
	ctx.mgr.doLog(model.LevelDebug, l.Debug(), msg, ctx.fields, fields)
}

// Info logs an info-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Info(msg string, keysAndValues ...any) {
	l := ctx.ctx.Logger()
	fields := pairs(keysAndValues)
	ctx.mgr.doLog(model.LevelInfo, l.Info(), msg, ctx.fields, fields)
}

// Warn logs a warn-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Warn(msg string, keysAndValues ...any) {
	l := ctx.ctx.Logger()
	fields := pairs(keysAndValues)
	ctx.mgr.doLog(model.LevelWarn, l.Warn(), msg, ctx.fields, fields)
}

// Error logs an error-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Error(msg string, keysAndValues ...any) {
	l := ctx.ctx.Logger()
	fields := pairs(keysAndValues)
	ctx.mgr.doLog(model.LevelError, l.Error(), msg, ctx.fields, fields)
}

// With creates a new logging context that inherits the context
// from the original ctx and adds additional context on top.
// The original ctx is not affected.
func (ctx Ctx) With(keysAndValues ...any) Ctx {
	c := ctx.ctx
	fields := pairs(keysAndValues)
	for i := 0; i < len(fields); i += 2 {
		key := fields[i].(string)
		val := fields[i+1]
		c = addContext(c, key, val)
	}
	fields = append(ctx.fields, fields...)
	return Ctx{ctx: c, mgr: ctx.mgr, fields: fields}
}

func (l *Manager) doLog(level model.LogLevel, ev *zerolog.Event, msg string, ctxFields, logFields []any) {
	var (
		tp     trace2.LogMessageParams
		traced bool
	)
	curr := l.rt.Current()
	numFields := len(ctxFields)/2 + len(logFields)/2

	if curr.Req != nil && curr.Trace != nil {
		traced = true
		tp = trace2.LogMessageParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Level:  level,
			Msg:    msg,
			Stack:  stack.Build(3),
			Fields: make([]trace2.LogField, 0, numFields),
		}

		for i := 0; i < len(ctxFields); i += 2 {
			key := ctxFields[i].(string)
			val := ctxFields[i+1]
			tp.Fields = append(tp.Fields, trace2.LogField{Key: key, Value: val})
		}
	}

	for i := 0; i < len(logFields); i += 2 {
		key := logFields[i].(string)
		val := logFields[i+1]
		addEventEntry(ev, key, val)
		if traced {
			tp.Fields = append(tp.Fields, trace2.LogField{Key: key, Value: val})
		}
	}

	ev.Msg(msg)

	if traced {
		curr.Trace.LogMessage(tp)
	}
}

func addEventEntry(ev *zerolog.Event, key string, val any) {
	if reserved(key) {
		key = "x_" + key
	}

	switch val := val.(type) {
	case error:
		ev.AnErr(key, val)
	case string:
		ev.Str(key, val)
	case bool:
		ev.Bool(key, val)

	case time.Time:
		ev.Time(key, val)
	case time.Duration:
		ev.Dur(key, val)
	case uuid.UUID:
		ev.Str(key, val.String())

	default:
		ev.Interface(key, val)

	case int8:
		ev.Int8(key, val)
	case int16:
		ev.Int16(key, val)
	case int32:
		ev.Int32(key, val)
	case int64:
		ev.Int64(key, val)
	case int:
		ev.Int(key, val)

	case uint8:
		ev.Uint8(key, val)
	case uint16:
		ev.Uint16(key, val)
	case uint32:
		ev.Uint32(key, val)
	case uint64:
		ev.Uint64(key, val)
	case uint:
		ev.Uint(key, val)

	case float32:
		ev.Float32(key, val)
	case float64:
		ev.Float64(key, val)
	}
}

func addContext(ctx zerolog.Context, key string, val any) zerolog.Context {
	if reserved(key) {
		key = "x_" + key
	}

	switch val := val.(type) {
	case error:
		return ctx.AnErr(key, val)
	case string:
		return ctx.Str(key, val)
	case bool:
		return ctx.Bool(key, val)

	case time.Time:
		return ctx.Time(key, val)
	case time.Duration:
		return ctx.Dur(key, val)
	case uuid.UUID:
		return ctx.Str(key, val.String())

	default:
		return ctx.Interface(key, val)

	case int8:
		return ctx.Int8(key, val)
	case int16:
		return ctx.Int16(key, val)
	case int32:
		return ctx.Int32(key, val)
	case int64:
		return ctx.Int64(key, val)
	case int:
		return ctx.Int(key, val)

	case uint8:
		return ctx.Uint8(key, val)
	case uint16:
		return ctx.Uint16(key, val)
	case uint32:
		return ctx.Uint32(key, val)
	case uint64:
		return ctx.Uint64(key, val)
	case uint:
		return ctx.Uint(key, val)

	case float32:
		return ctx.Float32(key, val)
	case float64:
		return ctx.Float64(key, val)
	}
}

func reserved(key string) bool {
	return strings.HasPrefix(key, InternalKeyPrefix)
}

// pairs ensures the key-values are in pairs.
// It drops the last entry if there's an odd number of entries.
func pairs(keysAndValues []any) []any {
	fields := keysAndValues
	num := len(fields)
	if num%2 == 1 {
		// Odd number of key-values, drop the last one
		num--
		fields = fields[:num]
	}
	return fields
}
