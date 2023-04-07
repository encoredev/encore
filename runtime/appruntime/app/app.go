package app

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/api"
	runtimeCfg "encore.dev/appruntime/config"
	rtmetrics "encore.dev/appruntime/metrics"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/secrets"
	"encore.dev/appruntime/service"
	"encore.dev/appruntime/testsupport"
	"encore.dev/appruntime/trace"
	"encore.dev/beta/auth"
	appCfg "encore.dev/config"
	"encore.dev/et"
	"encore.dev/internal/cloud"
	usermetrics "encore.dev/metrics"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/cache"
	"encore.dev/storage/sqldb"
)

type App struct {
	cfg        *runtimeCfg.Config
	rt         *reqtrack.RequestTracker
	json       jsoniter.API
	rootLogger zerolog.Logger
	api        *api.Server
	service    *service.Manager
	ts         *testsupport.Manager
	shutdown   *shutdownTracker

	encore          *encore.Manager
	secrets         *secrets.Manager
	auth            *auth.Manager
	rlog            *rlog.Manager
	sqldb           *sqldb.Manager
	pubsub          *pubsub.Manager
	cache           *cache.Manager
	config          *appCfg.Manager
	et              *et.Manager
	metrics         *rtmetrics.Manager
	metricsRegistry *usermetrics.Registry

	logMissingSecrets sync.Once
	missingSecrets    []string
}

func (app *App) Cfg() *runtimeCfg.Config            { return app.cfg }
func (app *App) ReqTrack() *reqtrack.RequestTracker { return app.rt }
func (app *App) RootLogger() *zerolog.Logger        { return &app.rootLogger }

type NewParams struct {
	Cfg         *runtimeCfg.Config
	APIHandlers []api.HandlerRegistration
	ServiceInit []service.Initializer
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
	tracingEnabled := trace.Enabled(cfg)
	var traceFactory trace.Factory = nil
	if tracingEnabled {
		traceFactory = trace.DefaultFactory
	}

	pc := platform.NewClient(cfg)

	rt := reqtrack.New(rootLogger, pc, traceFactory)
	json := jsonAPI(cfg)
	shutdown := newShutdownTracker()
	encore := encore.NewManager(cfg, rt)

	metricsRegistry := usermetrics.NewRegistry(rt, uint16(len(cfg.Static.BundledServices)))
	metrics := rtmetrics.NewManager(metricsRegistry, cfg, rootLogger)

	klock := clock.New()
	apiSrv := api.NewServer(cfg, rt, pc, encore, rootLogger, metricsRegistry, json, tracingEnabled, klock)
	apiSrv.Register(p.APIHandlers)
	apiSrv.SetAuthHandler(p.AuthHandler)
	service := service.NewManager(rt, rootLogger, p.ServiceInit)

	ts := testsupport.NewManager(cfg, rt, rootLogger)
	secrets := secrets.NewManager(cfg)
	auth := auth.NewManager(rt)
	rlog := rlog.NewManager(rt)
	sqldb := sqldb.NewManager(cfg, rt)
	pubsub := pubsub.NewManager(cfg, rt, ts, apiSrv, rootLogger, json)
	cache := cache.NewManager(cfg, rt, ts, json)
	appCfg := appCfg.NewManager(rt, json)
	etMgr := et.NewManager(cfg, rt)

	app := &App{
		cfg: cfg, rt: rt, json: json, rootLogger: rootLogger,
		api: apiSrv, service: service, ts: ts, shutdown: shutdown,
		encore: encore, secrets: secrets, auth: auth, rlog: rlog, sqldb: sqldb,
		pubsub: pubsub, cache: cache, config: appCfg, et: etMgr, metrics: metrics,
		metricsRegistry: metricsRegistry,
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

	app.Start()

	if err := app.service.InitializeServices(); err != nil {
		app.Shutdown()
		return err
	}
	serveErr := app.api.Serve(ln)

	isGraceful := app.ShutdownInitiated()
	app.Shutdown()

	// If Serve returned due to graceful shutdown, ignore the error from serve.
	if isGraceful {
		serveErr = nil
	}
	return serveErr
}

func (app *App) Start() {
	app.WatchForShutdownSignals()
	app.RegisterShutdown(app.api.Shutdown)
	app.RegisterShutdown(app.sqldb.Shutdown)
	app.RegisterShutdown(app.pubsub.Shutdown)
	app.RegisterShutdown(app.service.Shutdown)
	app.RegisterShutdown(app.metrics.Shutdown)

	go app.metrics.BeginCollection()
}

func jsonAPI(cfg *runtimeCfg.Config) jsoniter.API {
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

// ReconfigureZerologFormat reconfigures the zerolog Logger's output format
// based on the cloud provider.
func (app *App) ReconfigureZerologFormat() {
	// Note: if updating this function, also update
	// mapCloudFieldNamesToExpected in cli/cmd/encore/logs.go
	// as that reverses this for log streaming
	switch app.cfg.Runtime.EnvCloud {
	case cloud.GCP, cloud.Encore:
		zerolog.LevelFieldName = "severity"
		zerolog.TimestampFieldName = "timestamp"
		zerolog.TimeFieldFormat = time.RFC3339Nano
	default:
	}
}
