package trace

import (
	"encore.dev/appruntime/config"
)

type Version int

// CurrentVersion is the trace protocol version this package produces traces in.
const CurrentVersion Version = 10

// Enabled reports whether tracing is enabled.
// It is always enabled except for running tests and for ejected applications.
func Enabled(cfg *config.Config) bool {
	return cfg.Runtime.TraceEndpoint != "" && len(cfg.Runtime.AuthKeys) > 0 && !cfg.Static.Testing
}
