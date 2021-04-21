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
