package rlog

import (
	"encore.dev/internal/rlog"
)

type Ctx = rlog.Ctx

func Debug(traceExpr int32, msg string, keysAndValues ...interface{}) {
	rlog.Debug(traceExpr, msg, keysAndValues...)
}

func Info(traceExpr int32, msg string, keysAndValues ...interface{}) {
	rlog.Info(traceExpr, msg, keysAndValues...)
}

func Error(traceExpr int32, msg string, keysAndValues ...interface{}) {
	rlog.Error(traceExpr, msg, keysAndValues...)
}

func With(keysAndValues ...interface{}) Ctx {
	return rlog.With(keysAndValues...)
}

func Debugc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Debugc(traceExpr, ctx, msg, keysAndValues...)
}

func Infoc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Infoc(traceExpr, ctx, msg, keysAndValues...)
}

func Errorc(traceExpr int32, ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Errorc(traceExpr, ctx, msg, keysAndValues...)
}
