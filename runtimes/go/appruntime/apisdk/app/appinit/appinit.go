//go:build encore_app

package appinit

import (
	"io"

	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/apisdk/app"
	"encore.dev/appruntime/apisdk/service"
	"encore.dev/appruntime/shared/appconf"
	"encore.dev/appruntime/shared/logging"
	"encore.dev/appruntime/shared/shutdown"
)

// AppMain is the entrypoint to the Encore Application.
func AppMain() {
	inst := app.New(appconf.Runtime, service.Singleton, api.Singleton, shutdown.Singleton, logging.RootLogger)
	if err := inst.Run(); err != nil && err != io.EOF {
		logging.RootLogger.Fatal().Err(err).Msg("could not run")
	}
}
