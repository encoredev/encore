package app

import (
	"github.com/rs/zerolog"

	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/apisdk/service"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/shutdown"

	// Initialize the metric subsystem
	_ "encore.dev/appruntime/infrasdk/metrics"
)

type App struct {
	runtime  *config.Runtime
	service  *service.Manager
	api      *api.Server
	shutdown *shutdown.Tracker
	logger   zerolog.Logger
}

func New(runtime *config.Runtime, service *service.Manager, api *api.Server, shutdown *shutdown.Tracker, logger zerolog.Logger) *App {
	app := &App{
		runtime:  runtime,
		service:  service,
		api:      api,
		shutdown: shutdown,
		logger:   logger,
	}

	return app
}

func (app *App) Run() error {
	ln, err := Listen()
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	app.Start()

	// Begin serving requests.
	serveCh := make(chan error, 1)
	go func() {
		serveCh <- app.api.Serve(ln)
	}()

	if err := app.service.InitializeServices(); err != nil {
		app.shutdown.Shutdown(nil, err)
		return err
	}

	// Wait for the Serve to return before triggering shutdown.
	serveErr := <-serveCh

	isGraceful := app.shutdown.ShutdownInitiated()
	app.shutdown.Shutdown(nil, serveErr)

	// If Serve returned due to graceful shutdown, ignore the error from serve.
	if isGraceful {
		serveErr = nil
	}
	return serveErr
}

func (app *App) Start() {
	app.logStartupInfo()
	app.shutdown.OnShutdown(app.api.Shutdown)
	app.shutdown.RegisterHandlerHook(app.api.StopHandlers)
}

func (app *App) logStartupInfo() {
	switch {
	case app.runtime.EnvType == "test":
		// Don't log during tests.
	case app.runtime.EnvCloud == "local" && len(app.runtime.Gateways) == 0:
		// The gateway will log this for us
	default:
		// If we have a lot of handlers, don't log each one being registered.
		handlers := app.api.RegisteredHandlers()
		logEachRegistration := len(handlers) < 8 // chosen by a fair dice roll

		if logEachRegistration {
			for _, h := range handlers {
				app.logger.Info().
					Str("service", h.ServiceName()).
					Str("endpoint", h.EndpointName()).
					Str("path", h.SemanticPath()).
					Msg("registered API endpoint")
			}
		} else {
			app.logger.Info().Msgf("registered %d API endpoints", len(handlers))
		}
	}
}
