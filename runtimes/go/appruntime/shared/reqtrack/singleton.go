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
	tracingEnabled := appconf.Runtime.TraceEndpoint != "" && len(appconf.Runtime.AuthKeys) > 0
	if tracingEnabled {
		// Use the new sampling config if set, otherwise fall back to the deprecated scalar rate.
		samplingConfig := appconf.Runtime.TraceSamplingConfig
		if len(samplingConfig) == 0 && appconf.Runtime.TraceSamplingRate != nil {
			samplingConfig = map[string]float64{"_": *appconf.Runtime.TraceSamplingRate}
		}
		traceFactory = traceprovider.NewDefaultFactory(samplingConfig)
	}

	Singleton = New(logging.RootLogger, platform.Singleton, traceFactory)
}
