//go:build encore_app

package appinit

import (
	"io"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/app"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/service"
)

// AppMain is the entrypoint to the Encore Application.
func AppMain() {
	singleton.ReconfigureZerologFormat()
	if err := singleton.Run(); err != nil && err != io.EOF {
		singleton.RootLogger().Fatal().Err(err).Msg("could not run")
	}
}

// AppStart starts the Encore Application and doesn't block.
func AppStart() {
	singleton.ReconfigureZerologFormat()
	singleton.Start()
}

// singleton is the instance of the Encore app.
var singleton *app.App

// load is provided by the code-generated main package
// and linked here using go:linkname.
func load() *LoadData

type LoadData struct {
	StaticCfg   *config.Static
	APIHandlers []api.HandlerRegistration
	ServiceInit []service.Initializer
	AuthHandler api.AuthHandler
}

// We load everything during init so that the whole runtime is available to the Encore app
// even from within the app's init functions. The AppMain function runs later.
func init() {
	data := load()
	cfg := &config.Config{
		Runtime: config.ParseRuntime(config.GetAndClearEnv("ENCORE_RUNTIME_CONFIG")),
		Secrets: config.ParseSecrets(config.GetAndClearEnv("ENCORE_APP_SECRETS")),
		Static:  data.StaticCfg,
	}
	singleton = app.New(&app.NewParams{
		Cfg:         cfg,
		APIHandlers: data.APIHandlers,
		ServiceInit: data.ServiceInit,
		AuthHandler: data.AuthHandler,
	})
}

// LoadSecret loads the secret with the given key.
// If it is not defined it logs a fatal error and exits the process.
func LoadSecret(key string) string {
	return singleton.GetSecret(key)
}

var missingSecrets []string
