package metrics

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/prometheus"
)

func init() {
	registerProvider(providerDesc{
		name: "prometheus",
		matches: func(cfg *config.Metrics) bool {
			return cfg.Prometheus != nil
		},
		newExporter: func(m *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(m.runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return prometheus.New(m.static.BundledServices, m.runtime.Metrics.Prometheus, containerMetadata, m.rootLogger)
		},
	})
}
