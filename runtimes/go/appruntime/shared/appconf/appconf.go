//go:build encore_app

package appconf

import (
	"encoding/json"
	"log"

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

	// Local test binaries get the commit from the environment, so a new commit
	// does not change them. A binary linked with its commit keeps it.
	if commit := encoreenv.Get(config.AppCommitEnvVar); commit != "" && Static.AppCommit == (config.CommitInfo{}) {
		if err := json.Unmarshal([]byte(commit), &Static.AppCommit); err != nil {
			log.Fatalln("encore runtime: fatal error: could not parse "+config.AppCommitEnvVar+":", err)
		}
	}

	Runtime = config.ParseRuntime(
		encoreenv.Get("ENCORE_RUNTIME_CONFIG"),
		encoreenv.Get("ENCORE_RUNTIME_CONFIG_PATH"),
		encoreenv.Get("ENCORE_PROCESS_CONFIG"),
		encoreenv.Get("ENCORE_INFRA_CONFIG_PATH"),
		encoreenv.Get("ENCORE_DEPLOY_ID"),
	)
}
