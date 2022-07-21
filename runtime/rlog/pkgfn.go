//go:build encore_app

package rlog

var Singleton *Manager

func Debug(msg string, keysAndValues ...interface{}) {
	Singleton.Debug(msg, keysAndValues...)
}

func Info(msg string, keysAndValues ...interface{}) {
	Singleton.Info(msg, keysAndValues...)
}

func Error(msg string, keysAndValues ...interface{}) {
	Singleton.Error(msg, keysAndValues...)
}

func With(keysAndValues ...interface{}) Ctx {
	return Singleton.With(keysAndValues...)
}
