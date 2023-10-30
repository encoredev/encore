//go:build encore_app

package et

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
var Singleton = NewManager(appconf.Static, reqtrack.Singleton)
