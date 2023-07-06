//go:build encore_app

package secrets

import (
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/encoreenv"
)

var singleton = NewManager(
	appconf.Runtime,
	encoreenv.Get("ENCORE_APP_SECRETS"),
)

func Load(key string, inService string) string {
	return singleton.Load(key, inService)
}
