package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/prometheus"
)

func init() {
	registerProvider(providerDesc{
		name: "prometheus",
		matches: func(cfg *config.Metrics) bool {
			return cfg.Prometheus != nil
		},
		newExporter: func(m *Manager) exporter {
			return prometheus.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.Prometheus, m.rootLogger)
		},
	})
}
