//go:build !encore_no_gcp

package metrics

import (
	"os"
	"strings"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/gcp"
)

func init() {
	registerProvider(providerDesc{
		name: "gcp_cloud_monitoring",
		matches: func(cfg *config.Metrics) bool {
			return cfg.CloudMonitoring != nil
		},
		newExporter: func(mgr *Manager) exporter {
			containerMetadata, err := metadata.GetContainerMetadata(mgr.runtime)
			if err != nil {
				mgr.rootLogger.Err(err).Msg("unable to initialize metrics exporter: error getting container metadata")
				return nil
			}

			metricsCfg := mgr.runtime.Metrics

			nodeID, ok := metricsCfg.CloudMonitoring.MonitoredResourceLabels["node_id"]
			if ok {
				metricsCfg.CloudMonitoring.MonitoredResourceLabels["node_id"] = nodeID + "-" + containerMetadata.InstanceID
			}

			// Replace $ENV: prefix with the actual environment variable value
			for k, v := range metricsCfg.CloudMonitoring.MonitoredResourceLabels {
				if strings.HasPrefix(v, "$ENV:") {
					metricsCfg.CloudMonitoring.MonitoredResourceLabels[k] = os.Getenv(v[5:])
				}
			}

			return gcp.New(mgr.static.BundledServices, metricsCfg.CloudMonitoring, containerMetadata, mgr.rootLogger)
		},
	})
}
