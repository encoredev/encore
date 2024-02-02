//go:build !opentelemetry

package reqtrack

// configureOpenTelemetry is a no-op when OpenTelemetry is not enabled.
func configureOpenTelemetry(*RequestTracker) {}
