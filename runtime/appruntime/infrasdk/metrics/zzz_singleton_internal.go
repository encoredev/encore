//go:build encore_app

package metrics

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/shutdown"
	usermetrics "encore.dev/metrics"
)

// This file is named "zzz_singleton_internal.go" so that it is the last file
// in the package, to ensure all other init functions are run before
// we instantiate the manager.

// publicapigen:drop
var Singleton *Manager

func init() {
	Singleton = NewManager(usermetrics.Singleton, appconf.Static, appconf.Runtime, logging.RootLogger)
	shutdown.Singleton.OnShutdown(Singleton.Shutdown)
	go Singleton.BeginCollection()
}
