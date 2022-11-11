//go:build !encore_no_gcp

package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/gcp"
)

func init() {
	registerProvider(providerDesc{
		name: "gcp_cloud_monitoring",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudMonitoring != nil
		},
		newExporter: func(cfg *config.Metrics) exporter {
			return gcp.New(cfg.CloudMonitoring)
		},
	})
}
