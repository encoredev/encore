//go:build encore_app

package appconf

import (
	"encore.dev/appruntime/exported/config"
)

var (
	// Static is embedded at compile time using overlays.
	Static *config.Static

	// Runtime is injected at runtime using environment variables.
	Runtime *config.Runtime
)
