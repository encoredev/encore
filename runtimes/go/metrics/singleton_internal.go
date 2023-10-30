//go:build encore_app

package metrics

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/reqtrack"
)

var Singleton = NewRegistry(reqtrack.Singleton, len(appconf.Static.BundledServices))
