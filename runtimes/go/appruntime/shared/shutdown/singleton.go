//go:build encore_app

package shutdown

import (
	"encore.dev/appruntime/shared/health"

	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/logging"
)

var Singleton *Tracker

func init() {
	Singleton = NewTracker(appconf.Runtime, logging.RootLogger)
	health.Singleton.Register(Singleton)
	Singleton.WatchForShutdownSignals()
}
