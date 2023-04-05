//go:build !encore_no_gcp

package metrics

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/gcp"
)

func init() {
	registerProvider(providerDesc{
		name: "encore_cloud",
		matches: func(cfg *config.Metrics) bool {
			return cfg.EncoreCloud != nil
		},
		newExporter: func(mgr *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(mgr.runtime)
			if err != nil {
				mgr.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			metricsCfg := mgr.runtime.Metrics
			nodeID, ok := metricsCfg.EncoreCloud.MonitoredResourceLabels["node_id"]
			if !ok {
				mgr.rootLogger.Err(err).Msg("unable to initialize metrics exporter: missing node_id")
				return nil
			}
			metricsCfg.EncoreCloud.MonitoredResourceLabels["node_id"] = nodeID + "-" + containerMetadata.InstanceID
			return gcp.New(mgr.static.BundledServices, metricsCfg.EncoreCloud, containerMetadata, mgr.rootLogger)
		},
	})
}
