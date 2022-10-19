package app

import (
	"io"
	"os"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/service"
	"encore.dev/appruntime/testsupport"
	"encore.dev/appruntime/trace"
	"encore.dev/beta/auth"
	"encore.dev/custommetrics"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/cache"
	"encore.dev/storage/sqldb"
)

type App struct {
	cfg        *config.Config
	rt         *reqtrack.RequestTracker
	json       jsoniter.API
	rootLogger zerolog.Logger
	api        *api.Server
	service    *service.Manager
	ts         *testsupport.Manager
	shutdown   *shutdownTracker

	encore *encore.Manager
	auth   *auth.Manager
	rlog   *rlog.Manager
	sqldb  *sqldb.Manager
	pubsub *pubsub.Manager
	cache  *cache.Manager
}

func (app *App) Cfg() *config.Config                { return app.cfg }
func (app *App) ReqTrack() *reqtrack.RequestTracker { return app.rt }
func (app *App) RootLogger() *zerolog.Logger        { return &app.rootLogger }

type NewParams struct {
	Cfg         *config.Config
	APIHandlers []api.HandlerRegistration
	AuthHandler api.AuthHandler // nil means no auth handler
}

func New(p *NewParams) *App {
	cfg := p.Cfg
	var logOutput io.Writer = os.Stderr
	if cfg.Static.TestAsExternalBinary {
		// If we're running as a test and as a binary outside of the Encore Daemon, then we want to
		// log the output via a console logger, rather than the underlying JSON logs.
		logOutput = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = logOutput
		})
	}
	rootLogger := zerolog.New(logOutput).With().Timestamp().Logger()
	customMetrics := custommetrics.NewManager(cfg.Runtime.AppSlug, cfg.Runtime.EnvCloud, rootLogger)

	pc := platform.NewClient(cfg)
	doTrace := trace.Enabled(cfg)
	rt := reqtrack.New(rootLogger, pc, doTrace)
	json := jsonAPI(cfg)
	shutdown := newShutdownTracker()
	encore := encore.NewManager(cfg, rt)

	apiSrv := api.NewServer(cfg, rt, pc, encore, rootLogger, customMetrics, json)
	apiSrv.Register(p.APIHandlers)
	apiSrv.SetAuthHandler(p.AuthHandler)
	service := service.NewManager(rt)

	ts := testsupport.NewManager(cfg, rt, rootLogger)
	auth := auth.NewManager(rt)
	rlog := rlog.NewManager(rt)
	sqldb := sqldb.NewManager(cfg, rt)
	pubsub := pubsub.NewManager(cfg, rt, ts, apiSrv, rootLogger)
	cache := cache.NewManager(cfg, rt, ts, json)

	app := &App{
		cfg, rt, json, rootLogger, apiSrv, service, ts,
		shutdown,
		encore, auth, rlog, sqldb, pubsub, cache,
	}

	// If this is running inside an Encore app, initialize the singletons
	// that the package-level funcs rely on. Outside of apps this does nothing.
	initSingletonsForEncoreApp(app)

	return app
}

func (app *App) Run() error {
	ln, err := Listen()
	if err != nil {
		return err
	}
	defer ln.Close()

	app.WatchForShutdownSignals()
	app.RegisterShutdown(app.api.Shutdown)
	app.RegisterShutdown(app.sqldb.Shutdown)
	app.RegisterShutdown(app.pubsub.Shutdown)
	app.RegisterShutdown(app.service.Shutdown)

	serveErr := app.api.Serve(ln)

	isGraceful := app.ShutdownInitiated()
	app.Shutdown()

	// If Serve returned due to graceful shutdown, ignore the error from serve.
	if isGraceful {
		serveErr = nil
	}
	return serveErr
}

func (app *App) GetSecret(key string) (string, bool) {
	val, ok := app.cfg.Secrets[key]
	return val, ok
}

func jsonAPI(cfg *config.Config) jsoniter.API {
	indentStep := 2
	if cfg.Runtime.EnvType == "production" {
		indentStep = 0
	}
	return jsoniter.Config{
		EscapeHTML:             false,
		IndentionStep:          indentStep,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
	}.Froze()
}
