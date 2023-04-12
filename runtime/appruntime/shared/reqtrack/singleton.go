//go:build encore_app

package reqtrack

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/platform"
	"encore.dev/appruntime/shared/traceprovider"
)

var Singleton *RequestTracker

func init() {
	var traceFactory traceprovider.Factory
	tracingEnabled := appconf.Runtime.TraceEndpoint != "" && len(appconf.Runtime.AuthKeys) > 0 && !appconf.Static.Testing
	if tracingEnabled {
		traceFactory = &traceprovider.DefaultFactory{}
	}

	Singleton = New(logging.RootLogger, platform.Singleton, traceFactory)
}
