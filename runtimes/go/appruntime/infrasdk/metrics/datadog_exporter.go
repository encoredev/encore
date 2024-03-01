//go:build !encore_no_datadog

package metrics

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/datadog"
)

func init() {
	registerProvider(providerDesc{
		name: "datadog",
		matches: func(cfg *config.Metrics) bool {
			return cfg.Datadog != nil
		},
		newExporter: func(m *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(m.runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return datadog.New(m.static.BundledServices, m.runtime.Metrics.Datadog, containerMetadata, m.rootLogger)
		},
	})
}
