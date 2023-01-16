//go:build !encore_no_gcp

package metrics

import (
	"cloud.google.com/go/compute/metadata"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics/gcp"
)

func init() {
	registerProvider(providerDesc{
		name: "gcp_cloud_monitoring",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudMonitoring != nil
		},
		newExporter: func(mgr *Manager) exporter {
			cloudRunInstanceID, err := metadata.InstanceID()
			if err != nil {
				mgr.rootLogger.Err(err).Msg("unable to initialize metrics exporter")
				return nil
			}

			metricsCfg := mgr.cfg.Runtime.Metrics
			metricsCfg.EncoreCloud.MonitoredResourceLabels["node_id"] += "-" + cloudRunInstanceID
			return gcp.New(mgr.cfg.Static.BundledServices, metricsCfg.CloudMonitoring, mgr.rootLogger)
		},
	})
}
