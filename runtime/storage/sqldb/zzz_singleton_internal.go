//go:build encore_app

package sqldb

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
)

// Initialize the singleton instance.
// NOTE: This file is named zzz_singleton_internal.go so that
// the init function is initialized after all the providers
// have been registered.

//publicapigen:drop
var Singleton *Manager

func init() {
	Singleton = NewManager(appconf.Runtime, reqtrack.Singleton, testsupport.Singleton)
	shutdown.Singleton.RegisterShutdownHandler(Singleton.Shutdown)
}
