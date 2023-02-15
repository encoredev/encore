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
			containerMetadata, err := metadata.GetContainerMetadata(m.cfg.Runtime, m.rootLogger)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return prometheus.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.Prometheus, containerMetadata, m.rootLogger)
		},
	})
}
