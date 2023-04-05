//go:build encore_app

package appconf

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/encoreenv"
)

// Initialize the config inside a file named "zzz_initialize.go" so that it is
// executed after all other files in the package. This is necessary because
// we inject a generated file into the package that sets environment
// variables, and we want those to be read before we parse the config.

func init() {
	Runtime = config.ParseRuntime(
		encoreenv.Get("ENCORE_RUNTIME_CONFIG"),
		encoreenv.Get("ENCORE_DEPLOY_ID"),
	)
}
