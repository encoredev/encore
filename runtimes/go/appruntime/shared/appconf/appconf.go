//go:build encore_app

package appconf

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/encoreenv"
)

// static is set using linker flags.
var static string

var (
	// Static is embedded at compile time using overlays.
	Static *config.Static

	// Runtime is injected at runtime using environment variables.
	Runtime *config.Runtime
)

func init() {
	Static = config.ParseStatic(static)

	// Set the embedded environment variables.
	for k, v := range Static.EmbeddedEnvs {
		encoreenv.Set(k, v)
	}

	Runtime = config.ParseRuntime(
		encoreenv.Get("ENCORE_RUNTIME_CONFIG"),
		encoreenv.Get("ENCORE_RUNTIME_CONFIG_PATH"),
		encoreenv.Get("ENCORE_PROCESS_CONFIG"),
		encoreenv.Get("ENCORE_INFRA_CONFIG_PATH"),
		encoreenv.Get("ENCORE_DEPLOY_ID"),
	)
}
