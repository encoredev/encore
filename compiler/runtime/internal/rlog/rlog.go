package rlog

import (
	"encoding/binary"
	"time"

	"encore.dev/runtime"
	"encore.dev/types/uuid"
	"github.com/rs/zerolog"
)

type Ctx struct {
	ctx zerolog.Context
}

func Debug(traceExpr int32, msg string, keysAndValues ...interface{}) {
	log := runtime.Logger()
	doLog(log.Debug(), msg, keysAndValues...)
}

func Info(traceExpr int32, msg string, keysAndValues ...interface{}) {
	log := runtime.Logger()
	doLog(log.Info(), msg, keysAndValues...)
}

func Error(traceExpr int32, msg string, keysAndValues ...interface{}) {
	log := runtime.Logger()
	doLog(log.Error(), msg, keysAndValues...)
}

func With(keysAndValues ...interface{}) Ctx {
	ctx := runtime.Logger().With()
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i].(string)
		val := keysAndValues[i+1]
		ctx = addContext(ctx, key, val)
	}
	return Ctx{ctx: ctx}
}

func Debugc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	doLog(l.Debug(), msg, keysAndValues...)
}

func Infoc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	doLog(l.Info(), msg, keysAndValues...)
}

func Errorc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	l := ctx.ctx.Logger()
	doLog(l.Error(), msg, keysAndValues...)
}

func doLog(ev *zerolog.Event, msg string, keysAndValues ...interface{}) {
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i].(string)
		val := keysAndValues[i+1]
		addEventEntry(ev, key, val)
	}
	ev.Msg(msg)
}

func addEventEntry(ev *zerolog.Event, key string, val interface{}) {
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

var bin = binary.BigEndian
