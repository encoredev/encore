package rlog

import (
	"encore.dev/internal/rlog"
)

type Ctx = rlog.Ctx

func Debug(msg string, keysAndValues ...interface{}) {
	rlog.Debug(msg, keysAndValues...)
}

func Info(msg string, keysAndValues ...interface{}) {
	rlog.Info(msg, keysAndValues...)
}

func Error(msg string, keysAndValues ...interface{}) {
	rlog.Error(msg, keysAndValues...)
}

func With(keysAndValues ...interface{}) Ctx {
	return rlog.With(keysAndValues...)
}

func Debugc(ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Debugc(ctx, msg, keysAndValues...)
}

func Infoc(ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Infoc(ctx, msg, keysAndValues...)
}

func Errorc(ctx Ctx, msg string, keysAndValues ...interface{}) {
	rlog.Errorc(ctx, msg, keysAndValues...)
}
