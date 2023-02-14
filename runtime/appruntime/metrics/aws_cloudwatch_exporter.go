//go:build !encore_no_aws

package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/aws"
)

func init() {
	registerProvider(providerDesc{
		name: "aws_cloudwatch",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudWatch != nil
		},
		newExporter: func(m *Manager) exporter {
			instanceID, err := metadata.InstanceID(m.cfg.Runtime)
			if err != nil {
				m.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting instance ID")
				return nil
			}

			return aws.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.CloudWatch, instanceID, m.rootLogger)
		},
	})
}
