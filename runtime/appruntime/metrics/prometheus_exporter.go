package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/prometheus"
)

func init() {
	registerProvider(providerDesc{
		name: "prometheus",
		matches: func(cfg *config.Metrics) bool {
			return cfg.Prometheus != nil
		},
		newExporter: func(m *Manager) exporter {
			instanceID, err := metadata.InstanceID(m.cfg.Runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting instance ID")
				return nil
			}

			return prometheus.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.Prometheus, instanceID, m.rootLogger)
		},
	})
}
