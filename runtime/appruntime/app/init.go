//go:build encore_app

package app

import (
	"encore.dev/appruntime/api"
	"encore.dev/appruntime/app/appinit"
	"encore.dev/appruntime/config"

	_ "unsafe"
)

// load is provided by the code-generated main package
// and linked here using go:linkname.
func load() *LoadData

type LoadData struct {
	StaticCfg   *config.Static
	APIHandlers []api.Handler
	AuthHandler api.AuthHandler
}

// We load everything during init so that the whole runtime is available to the Encore app
// even from within the app's init functions. The AppMain function runs later.
//go:linkname loadAppInstance encore.dev/appruntime/app/appinit.loadAppInstance
func loadAppInstance() appinit.AppInstance {
	data := load()

	cfg := &config.Config{
		Runtime: config.ParseRuntime(config.GetAndClearEnv("ENCORE_RUNTIME_CONFIG")),
		Secrets: config.ParseSecrets(config.GetAndClearEnv("ENCORE_APP_SECRETS")),
		Static:  data.StaticCfg,
	}
	return New(&NewParams{
		Cfg:         cfg,
		APIHandlers: data.APIHandlers,
		AuthHandler: data.AuthHandler,
	})
}
