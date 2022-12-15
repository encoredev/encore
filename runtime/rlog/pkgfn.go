//go:build encore_app

package rlog

//publicapigen:drop
var Singleton *Manager

// Debug logs a debug-level message.
// The variadic key-value pairs are treated as they are in With.
func Debug(msg string, keysAndValues ...any) {
	Singleton.Debug(msg, keysAndValues...)
}

// Info logs an info-level message.
// The variadic key-value pairs are treated as they are in With.
func Info(msg string, keysAndValues ...any) {
	Singleton.Info(msg, keysAndValues...)
}

// Warn logs a warn-level message.
// The variadic key-value pairs are treated as they are in With.
func Warn(msg string, keysAndValues ...any) {
	Singleton.Warn(msg, keysAndValues...)
}

// Error logs an error-level message.
// The variadic key-value pairs are treated as they are in With.
func Error(msg string, keysAndValues ...any) {
	Singleton.Error(msg, keysAndValues...)
}

// With adds a variadic number of fields to the logging context.
// The keysAndValues must be pairs of string keys and arbitrary data.
func With(keysAndValues ...any) Ctx {
	return Singleton.With(keysAndValues...)
}
