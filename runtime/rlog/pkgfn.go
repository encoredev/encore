//go:build encore_app

// Package rlog provides a simple logging interface which is integrated with Encore's
// inbuilt distributed tracing.
//
// For more information about logging inside Encore applications see https://encore.dev/docs/observability/logging.
package rlog

//publicapigen:drop
var Singleton *Manager

// Debug logs a debug-level message.
// The variadic key-value pairs are treated as they are in With.
func Debug(msg string, keysAndValues ...interface{}) {
	Singleton.Debug(msg, keysAndValues...)
}

// Info logs an info-level message.
// The variadic key-value pairs are treated as they are in With.
func Info(msg string, keysAndValues ...interface{}) {
	Singleton.Info(msg, keysAndValues...)
}

// Error logs an error-level message.
// The variadic key-value pairs are treated as they are in With.
func Error(msg string, keysAndValues ...interface{}) {
	Singleton.Error(msg, keysAndValues...)
}

// With adds a variadic number of fields to the logging context.
// The keysAndValues must be pairs of string keys and arbitrary data.
func With(keysAndValues ...interface{}) Ctx {
	return Singleton.With(keysAndValues...)
}
