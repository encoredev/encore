package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/metrics"
)

type Manager struct {
	ctx    context.Context
	cancel func()

	static     *config.Static
	runtime    *config.Runtime
	reg        *metrics.Registry
	rootLogger zerolog.Logger
	exp        exporter

	logsEmitter *logsBasedEmitter
}

func NewManager(reg *metrics.Registry, static *config.Static, rtConf *config.Runtime, rootLogger zerolog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:        ctx,
		cancel:     cancel,
		reg:        reg,
		static:     static,
		runtime:    rtConf,
		rootLogger: rootLogger,
	}

	// Metrics aren't configured, return.
	if rtConf.Metrics == nil {
		return mgr
	}

	for _, desc := range providerRegistry {
		if desc.matches(rtConf.Metrics) {
			mgr.exp = desc.newExporter(mgr)
			break
		}
	}

	if rtConf.Metrics.LogsBased != nil {
		mgr.logsEmitter = newLogsBasedEmitter(rootLogger)
	}

	return mgr
}

func (mgr *Manager) Shutdown(force context.Context) {
	mgr.collectNow(force)
	mgr.cancel()

	if mgr.exp != nil {
		mgr.exp.Shutdown(force)
	}
}

func (mgr *Manager) BeginCollection() {
	if mgr.exp == nil {
		return
	} else if mgr.runtime.EnvType == "test" {
		// Don't collect metrics when running tests.
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	mgr.collectNow(ctx)
	cancel()

	interval := mgr.runtime.Metrics.CollectionInterval
	if interval <= 0 {
		interval = time.Minute
	}
	timeoutDur := interval / 2

	ticker := time.NewTicker(interval)
	for {
		select {
		case <-mgr.ctx.Done():
			ticker.Stop()
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeoutDur)
			mgr.collectNow(ctx)
			cancel()
		}
	}
}

func (mgr *Manager) collectNow(ctx context.Context) {
	if mgr.exp == nil {
		return
	}

	m := mgr.reg.Collect()
	if err := mgr.exp.Export(ctx, m); err != nil {
		mgr.rootLogger.Error().Err(err).Msg("unable to emit metrics")
	} else {
		mgr.rootLogger.Trace().Int("num_metrics", len(m)).Msg("successfully emitted metrics")
	}
}

type exporter interface {
	Export(context.Context, []metrics.CollectedMetric) error
	Shutdown(force context.Context)
}

type providerDesc struct {
	name        string
	matches     func(cfg *config.Metrics) bool
	newExporter func(m *Manager) exporter
}

var providerRegistry []providerDesc

func registerProvider(desc providerDesc) {
	providerRegistry = append(providerRegistry, desc)
}
