//go:build !encore_no_azure

package metrics

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/azure"
)

func init() {
	registerProvider(providerDesc{
		name: "azure_monitor",
		matches: func(cfg *config.Metrics) bool {
			return cfg.AzureMonitor != nil
		},
		newExporter: func(m *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(m.runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return azure.New(m.static.BundledServices, m.runtime.Metrics.AzureMonitor, containerMetadata, m.rootLogger)
		},
	})
}
