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
			return cfg.CloudMonitoring != nil || cfg.EncoreCloud != nil
		},
		newExporter: func(mgr *Manager) exporter {
			metricsCfg := mgr.cfg.Runtime.Metrics
			if metricsCfg.EncoreCloud != nil {
				return gcp.NewEncoreCloudExporter(mgr.cfg.Static.BundledServices, metricsCfg.CloudMonitoring, mgr.rootLogger, metricsCfg.EncoreCloud.MetricNames)
			}
			return gcp.New(mgr.cfg.Static.BundledServices, metricsCfg.CloudMonitoring, mgr.rootLogger)
		},
	})
}
