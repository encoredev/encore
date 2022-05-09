package run

import (
	_ "unsafe"

	"encore.dev/runtime/config"
)

// loadBlankConfigInstance is linked into the encore.devruntime/config package
// such that the package init will work outside a compiled encore app.
// This allows us to reuse the config structures between the daemon and
// the compiled app.
//
//go:linkname loadBlankConfigInstance encore.dev/runtime/config.loadConfig
func loadBlankConfigInstance() (*config.Config, error) {
	return &config.Config{
		Static:  &config.Static{},
		Runtime: &config.Runtime{},
		Secrets: nil,
	}, nil
}
