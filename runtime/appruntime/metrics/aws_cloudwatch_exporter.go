//go:build !encore_no_aws

package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/aws"
)

func init() {
	registerProvider(providerDesc{
		name: "aws_cloudwatch",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudWatch != nil
		},
		newExporter: func(m *Manager) exporter {
			return aws.New(m.cfg.Static.BundledServices, m.cfg.Runtime.Metrics.CloudWatch, m.rootLogger)
		},
	})
}
