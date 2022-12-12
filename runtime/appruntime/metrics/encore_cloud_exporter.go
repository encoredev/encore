//go:build !encore_no_gcp

package metrics

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/gcp"
)

func init() {
	registerProvider(providerDesc{
		name: "encore_cloud",
		matches: func(cfg *config.Metrics) bool {
			return cfg.EncoreCloud != nil
		},
		newExporter: func(mgr *Manager) exporter {
			metricsCfg := mgr.cfg.Runtime.Metrics
			return gcp.NewEncoreCloudExporter(mgr.cfg.Static.BundledServices, metricsCfg.CloudMonitoring, mgr.rootLogger, metricsCfg.EncoreCloud.MetricNames)
		},
	})
}
