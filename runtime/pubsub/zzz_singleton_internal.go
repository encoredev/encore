//go:build encore_app

package pubsub

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/jsonapi"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

// Initialize the singleton instance.
// NOTE: This file is named zzz_singleton_internal.go so that
// the init function is initialized after all the providers
// have been registered.

//publicapigen:drop
var Singleton *Manager

func init() {
	Singleton = NewManager(
		appconf.Static, appconf.Runtime, reqtrack.Singleton, testsupport.Singleton,
		logging.RootLogger, jsonapi.Default,
	)
}
