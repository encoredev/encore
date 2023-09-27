//go:build encore_app

package platform

import "encore.dev/appruntime/shared/appconf"

var Singleton = NewClient(appconf.Static, appconf.Runtime)
