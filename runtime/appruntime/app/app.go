package app

import (
	"fmt"
	"io"
	"os"
	"strings"
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
	service := service.NewManager(rt)

	ts := testsupport.NewManager(cfg, rt, rootLogger)
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
		encore: encore, auth: auth, rlog: rlog, sqldb: sqldb, pubsub: pubsub,
		cache: cache, config: appCfg, et: etMgr, metrics: metrics,
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

	app.WatchForShutdownSignals()
	app.RegisterShutdown(app.api.Shutdown)
	app.RegisterShutdown(app.sqldb.Shutdown)
	app.RegisterShutdown(app.pubsub.Shutdown)
	app.RegisterShutdown(app.service.Shutdown)
	app.RegisterShutdown(app.metrics.Shutdown)

	go app.metrics.BeginCollection()

	serveErr := app.api.Serve(ln)

	isGraceful := app.ShutdownInitiated()
	app.Shutdown()

	// If Serve returned due to graceful shutdown, ignore the error from serve.
	if isGraceful {
		serveErr = nil
	}
	return serveErr
}

func (app *App) GetSecret(key string) string {
	if val, ok := app.cfg.Secrets[key]; ok {
		return val
	}

	// For anything but local development, a missing secret is a fatal error.
	if app.cfg.Runtime.EnvCloud != "local" {
		fmt.Fprintln(os.Stderr, "encore: could not find secret", key)
		os.Exit(2)
		panic("unreachable")
	}

	app.missingSecrets = append(app.missingSecrets, key)
	app.logMissingSecrets.Do(func() {
		// Wait one second before logging all the missing secrets.
		go func() {
			time.Sleep(1 * time.Second)
			fmt.Fprintln(os.Stderr, "\n\033[31mwarning: secrets not defined:", strings.Join(app.missingSecrets, ", "), "\033[0m")
			fmt.Fprintln(os.Stderr, "\033[2mnote: undefined secrets are left empty for local development only.")
			fmt.Fprint(os.Stderr, "see https://encore.dev/docs/primitives/secrets for more information\033[0m\n\n")
		}()
	})

	return ""
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
	case cloud.GCP:
		zerolog.LevelFieldName = "severity"
		zerolog.TimestampFieldName = "timestamp"
		zerolog.TimeFieldFormat = time.RFC3339Nano
	default:
	}
}
