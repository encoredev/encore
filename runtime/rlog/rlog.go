// Package rlog provides a simple logging interface which is integrated with Encore's
// inbuilt distributed tracing.
//
// For more information about logging inside Encore applications see https://encore.dev/docs/observability/logging.
package rlog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/trace"
	"encore.dev/beta/errs"
	"encore.dev/internal/stack"
	"encore.dev/types/uuid"
)

type logLevel byte

const (
	levelTrace logLevel = 0 // unused; reserve for future use
	levelDebug logLevel = 1
	levelInfo  logLevel = 2
	levelWarn  logLevel = 3
	levelError logLevel = 4
)

const (
	// InternalKeyPrefix is the prefix of log field keys that are reserved for
	// internal use only. For example, logs-based metrics rely on a log field named
	// 'encore_metric_name' to determine which metric is being emitted.
	//
	// If Encore apps add log fields that start with this prefix, we'll prefix them
	// with 'x_' to make sure they don't interfere with internal log fields.
	//
	//publicapigen:drop
	InternalKeyPrefix = "encore_"
)

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
	ctx zerolog.Context
	mgr *Manager
}

func (l *Manager) Debug(msg string, keysAndValues ...interface{}) {
	l.doLog(levelDebug, l.rt.Logger().Debug(), msg, keysAndValues...)
}

func (l *Manager) Info(msg string, keysAndValues ...interface{}) {
	l.doLog(levelInfo, l.rt.Logger().Info(), msg, keysAndValues...)
}

func (l *Manager) Warn(msg string, keysAndValues ...interface{}) {
	l.doLog(levelWarn, l.rt.Logger().Warn(), msg, keysAndValues...)
}

func (l *Manager) Error(msg string, keysAndValues ...interface{}) {
	l.doLog(levelError, l.rt.Logger().Error(), msg, keysAndValues...)
}

func (l *Manager) With(keysAndValues ...interface{}) Ctx {
	ctx := l.rt.Logger().With()
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i].(string)
		val := keysAndValues[i+1]
		ctx = addContext(ctx, key, val)
	}
	return Ctx{ctx: ctx, mgr: l}
}

// Debug logs a debug-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Debug(msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	ctx.mgr.doLog(levelDebug, l.Debug(), msg, keysAndValues...)
}

// Info logs an info-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Info(msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	ctx.mgr.doLog(levelInfo, l.Info(), msg, keysAndValues...)
}

// Warn logs a warn-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Warn(msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	ctx.mgr.doLog(levelWarn, l.Warn(), msg, keysAndValues...)
}

// Error logs an error-level message, merging the context from ctx
// with the additional context provided as key-value pairs.
// The variadic key-value pairs are treated as they are in With.
func (ctx Ctx) Error(msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	ctx.mgr.doLog(levelError, l.Error(), msg, keysAndValues...)
}

// With creates a new logging context that inherits the context
// from the original ctx and adds additional context on top.
// The original ctx is not affected.
func (ctx Ctx) With(keysAndValues ...interface{}) Ctx {
	c := ctx.ctx
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i].(string)
		val := keysAndValues[i+1]
		c = addContext(c, key, val)
	}
	return Ctx{ctx: c, mgr: ctx.mgr}
}

func (l *Manager) doLog(level logLevel, ev *zerolog.Event, msg string, keysAndValues ...interface{}) {
	var tb *trace.Buffer
	curr := l.rt.Current()
	fields := len(keysAndValues) / 2

	if curr.Req != nil && curr.Trace != nil {
		t := trace.NewBuffer(16 + 8 + len(msg) + 4 + fields*50)
		tb = &t
		tb.Bytes(curr.Req.SpanID[:])
		tb.UVarint(uint64(curr.Goctr))
		tb.Byte(byte(level))
		tb.String(msg)
		tb.UVarint(uint64(fields))
	}

	for i := 0; i < fields; i++ {
		key := keysAndValues[2*i].(string)
		val := keysAndValues[2*i+1]
		addEventEntry(ev, key, val)
		if tb != nil {
			addTraceBufEntry(tb, key, val)
		}
	}
	ev.Msg(msg)

	if curr.Trace != nil {
		tb.Stack(stack.Build(3))
		curr.Trace.Add(trace.LogMessage, tb.Buf())
	}
}

func addEventEntry(ev *zerolog.Event, key string, val interface{}) {
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

func addContext(ctx zerolog.Context, key string, val interface{}) zerolog.Context {
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

const (
	errType     byte = 1
	strType     byte = 2
	boolType    byte = 3
	timeType    byte = 4
	durType     byte = 5
	uuidType    byte = 6
	jsonType    byte = 7
	intType     byte = 8
	uintType    byte = 9
	float32Type byte = 10
	float64Type byte = 11
)

func addTraceBufEntry(tb *trace.Buffer, key string, val interface{}) {
	switch val := val.(type) {
	case error:
		tb.Byte(errType)
		tb.String(key)
		tb.Err(val)
		tb.Stack(errs.Stack(val))
	case string:
		tb.Byte(strType)
		tb.String(key)
		tb.String(val)
	case bool:
		tb.Byte(boolType)
		tb.String(key)
		tb.Bool(val)
	case time.Time:
		tb.Byte(timeType)
		tb.String(key)
		tb.Time(val)
	case time.Duration:
		tb.Byte(durType)
		tb.String(key)
		tb.Int64(int64(val))
	case uuid.UUID:
		tb.Byte(uuidType)
		tb.String(key)
		tb.Bytes(val[:])

	default:
		tb.Byte(jsonType)
		tb.String(key)
		data, err := json.Marshal(val)
		if err != nil {
			tb.ByteString(nil)
			tb.Err(err)
		} else {
			tb.ByteString(data)
			tb.Err(nil)
		}

	case int8:
		tb.Byte(intType)
		tb.String(key)
		tb.Varint(int64(val))
	case int16:
		tb.Byte(intType)
		tb.String(key)
		tb.Varint(int64(val))
	case int32:
		tb.Byte(intType)
		tb.String(key)
		tb.Varint(int64(val))
	case int64:
		tb.Byte(intType)
		tb.String(key)
		tb.Varint(int64(val))
	case int:
		tb.Byte(intType)
		tb.String(key)
		tb.Varint(int64(val))

	case uint8:
		tb.Byte(uintType)
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint16:
		tb.Byte(uintType)
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint32:
		tb.Byte(uintType)
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint64:
		tb.Byte(uintType)
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint:
		tb.Byte(uintType)
		tb.String(key)
		tb.UVarint(uint64(val))

	case float32:
		tb.Byte(float32Type)
		tb.String(key)
		tb.Float32(val)
	case float64:
		tb.Byte(float64Type)
		tb.String(key)
		tb.Float64(val)
	}
}
