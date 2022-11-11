package metrics

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/beta/errs"
	"encore.dev/metrics"
)

type Manager struct {
	ctx    context.Context
	cancel func()

	cfg        *config.Metrics
	reg        *metrics.Registry
	rootLogger zerolog.Logger
	exp        exporter
}

func NewManager(reg *metrics.Registry, cfg *config.Metrics, rootLogger zerolog.Logger) *Manager {
	var exp exporter
	var tried []string
	for _, desc := range providerRegistry {
		tried = append(tried, desc.name)
		if desc.matches(cfg) {
			exp = desc.newExporter(cfg)
			break
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:        ctx,
		cancel:     cancel,
		reg:        reg,
		cfg:        cfg,
		exp:        exp,
		rootLogger: rootLogger,
	}
}

func (mgr *Manager) Shutdown(force context.Context) {
	mgr.cancel()
	mgr.exp.Shutdown(force)
}

func (mgr *Manager) BeginCollection() {
	if mgr.exp == nil {
		return
	}

	mgr.collectNow()
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-mgr.ctx.Done():
			ticker.Stop()
		case <-ticker.C:
			mgr.collectNow()
		}
	}
}

func (mgr *Manager) collectNow() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	m := mgr.reg.Collect()
	if err := mgr.exp.Export(ctx, m); err != nil {
		mgr.rootLogger.Error().Err(err).Msg("unable to emit metrics")
	} else {
		mgr.rootLogger.Trace().Int("num_metrics", len(m)).Msg("successfully emitted metrics")
	}
}

func (m *Manager) ReqEnd(service, endpoint string, err error, httpStatus int, durSecs float64) {
	//code := code(err, httpStatus)
	//m.exp.IncCounter(
	//	"e_requests_total",
	//	"service", service,
	//	"endpoint", endpoint,
	//	"code", code,
	//)
	//m.exp.Observe(
	//	"e_request_duration_seconds",
	//	"duration", durSecs,
	//	"service", service,
	//	"endpoint", endpoint,
	//	"code", code,
	//)
}

func code(err error, httpStatus int) string {
	if err != nil {
		e := errs.Convert(err).(*errs.Error)
		return e.Code.String()
	}

	if httpStatus == 0 {
		return errs.OK.String()
	}

	if code := errs.HTTPStatusToCode(httpStatus); code != errs.Unknown {
		return code.String()
	}
	return "http_" + strconv.Itoa(httpStatus)
}

type exporter interface {
	Export(context.Context, []metrics.CollectedMetric) error
	Shutdown(force context.Context)
}

type providerDesc struct {
	name        string
	matches     func(cfg *config.Metrics) bool
	newExporter func(cfg *config.Metrics) exporter
}

var providerRegistry []providerDesc

func registerProvider(desc providerDesc) {
	providerRegistry = append(providerRegistry, desc)
}
