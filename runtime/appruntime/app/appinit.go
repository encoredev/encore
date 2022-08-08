//go:build encore_app

package app

import (
	"fmt"
	"os"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"

	// Import json-iterator to make sure its type registry is initialized.
	_ "github.com/json-iterator/go"
)

// Main is the entrypoint to the Encore Application.
func Main() {
	if err := singleton.Run(); err != nil {
		singleton.RootLogger().Fatal().Err(err).Msg("could not run")
	}
}

// singleton is the instance of the Encore app.
var singleton *App

// load is provided by the code-generated main package
// and linked here using go:linkname.
func load() *LoadData

type LoadData struct {
	StaticCfg   *config.Static
	APIHandlers []api.Handler
	AuthHandler api.AuthHandler
}

// LoadSecret loads the secret with the given key.
// If it is not defined it logs a fatal error and exits the process.
func LoadSecret(key string) string {
	if val, ok := singleton.GetSecret(key); ok {
		return val
	}

	fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
	os.Exit(2)
	panic("unreachable")
}

// initOnce ensures we only initialize things once.
var initOnce sync.Once

// We load everything during init so that the whole runtime is available to the Encore app
// even from within the app's init functions. The Main function runs later.
//
// This is a not in an `init` so that we can call it from pkg `appinit` with go:linkname.
func doInit() {
	initOnce.Do(func() {
		data := load()
		cfg := &config.Config{
			Runtime: config.ParseRuntime(config.GetAndClearEnv("ENCORE_RUNTIME_CONFIG")),
			Secrets: config.ParseSecrets(config.GetAndClearEnv("ENCORE_APP_SECRETS")),
			Static:  data.StaticCfg,
		}
		singleton = New(&NewParams{
			Cfg:         cfg,
			APIHandlers: data.APIHandlers,
			AuthHandler: data.AuthHandler,
		})
	})
}

func init() {
	doInit()
}
