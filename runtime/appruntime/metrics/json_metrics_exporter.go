package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/json_based"
)

func init() {
	registerProvider(providerDesc{
		name: "json_based",
		matches: func(cfg *config.Metrics) bool {
			return cfg.JSONBased != nil
		},
		newExporter: func(m *Manager) exporter {
			return json_based.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.JSONBased, m.rootLogger)
		},
	})
}
