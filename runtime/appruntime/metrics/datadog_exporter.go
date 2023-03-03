package metrics

import (
	"os"

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

			cfg := m.cfg.Runtime.Metrics.Datadog
			if cfg.Site == "" {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: Datadog site unset")
				return nil
			}

			err = os.Setenv("DD_SITE", cfg.Site)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error setting env variable")
				return nil
			}

			if cfg.APIKey == "" {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: Datadog API key unset")
				return nil
			}

			err = os.Setenv("DD_API_KEY", cfg.APIKey)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error setting env variable")
				return nil
			}

			return datadog.New(m.cfg.Static.BundledServices, containerMetadata, m.rootLogger)
		},
	})
}
