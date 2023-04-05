//go:build !encore_no_aws

package metrics

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/aws"
)

func init() {
	registerProvider(providerDesc{
		name: "aws_cloudwatch",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudWatch != nil
		},
		newExporter: func(m *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(m.runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			return aws.New(m.static.BundledServices, m.runtime.Metrics.CloudWatch, containerMetadata, m.rootLogger)
		},
	})
}
