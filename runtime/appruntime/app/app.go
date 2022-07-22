package app

import (
	"context"
	"os"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/testsupport"
	"encore.dev/appruntime/trace"
	"encore.dev/beta/auth"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
)

type App struct {
	appCtx       context.Context
	cancelAppCtx func()

	cfg        *config.Config
	rt         *reqtrack.RequestTracker
	json       jsoniter.API
	rootLogger zerolog.Logger
	api        *api.Server
	ts         *testsupport.Manager
	shutdown   *shutdownTracker

	encore *encore.Manager
	auth   *auth.Manager
	rlog   *rlog.Manager
	sqldb  *sqldb.Manager
	pubsub *pubsub.Manager
}

func (app *App) Cfg() *config.Config                { return app.cfg }
func (app *App) ReqTrack() *reqtrack.RequestTracker { return app.rt }
func (app *App) RootLogger() *zerolog.Logger        { return &app.rootLogger }

type NewParams struct {
	Cfg         *config.Config
	APIHandlers []api.Handler
	AuthHandler api.AuthHandler // nil means no auth handler
}

func New(p *NewParams) *App {
	cfg := p.Cfg
	rootLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	pc := platform.NewClient(cfg)
	doTrace := trace.Enabled(cfg)
	rt := reqtrack.New(rootLogger, pc, doTrace)
	json := jsonAPI(cfg)
	shutdown := newShutdownTracker()

	apiSrv := api.NewServer(cfg, rt, pc, rootLogger, json)
	apiSrv.Register(p.APIHandlers)
	apiSrv.SetAuthHandler(p.AuthHandler)

	appCtx, cancel := context.WithCancel(context.Background())

	ts := testsupport.NewManager(cfg, rt, rootLogger)
	encore := encore.NewManager(cfg, rt)
	auth := auth.NewManager(rt)
	rlog := rlog.NewManager(rt)
	sqldb := sqldb.NewManager(cfg, rt)
	pubsub := pubsub.NewManager(appCtx, cfg, rt, ts, apiSrv, rootLogger)

	app := &App{
		appCtx, cancel,
		cfg, rt, json, rootLogger, apiSrv, ts,
		shutdown,
		encore, auth, rlog, sqldb, pubsub,
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
