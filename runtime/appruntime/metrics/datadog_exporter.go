package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/datadog"
)

func init() {
	registerProvider(providerDesc{
		name: "datadog",
		matches: func(cfg *config.Metrics) bool {
			return cfg.Datadog != nil
		},
		newExporter: func(m *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(m.cfg.Runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return datadog.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.Datadog, containerMetadata, m.rootLogger)
		},
	})
}
