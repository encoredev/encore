//go:build encore_app

package et

import (
	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

//publicapigen:drop
var Singleton = NewManager(appconf.Static, reqtrack.Singleton, testsupport.Singleton, api.Singleton)
