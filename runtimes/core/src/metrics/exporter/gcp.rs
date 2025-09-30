use crate::metrics::manager::Exporter;
use crate::metrics::CollectedMetric;
use std::collections::HashMap;
use std::time::UNIX_EPOCH;

#[derive(Clone, Debug)]
pub struct Gcp {
    // ProjectID is the GCP project id to send metrics to.
    project_id: String,
    // MonitoredResourceType is the enum value for the monitored resource this application is monitoring.
    // See https://cloud.google.com/monitoring/api/resources for valid values.
    monitored_resource_type: String,
    // MonitoredResourceLabels are the labels to specify for the monitored resource.
    // Each monitored resource type has a pre-defined set of labels that must be set.
    // See https://cloud.google.com/monitoring/api/resources for expected labels.
    monitored_resource_labels: HashMap<String, String>,
    // MetricNames contains the mapping between metric names in Encore and metric
    // names in GCP.
    metric_names: HashMap<String, String>,
}

impl Gcp {
    pub fn new(
        project_id: String,
        monitored_resource_type: String,
        monitored_resource_labels: HashMap<String, String>,
        metric_names: HashMap<String, String>,
    ) -> Self {
        eprintln!("=== GCP NEW ===");
        Self {
            project_id,
            monitored_resource_type,
            monitored_resource_labels,
            metric_names,
        }
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> Result<(), anyhow::Error> {
        // TODO implemet this for real
        log::info!(
            "Exporting {} metrics to GCP project {}",
            metrics.len(),
            self.project_id
        );

        for metric in metrics {
            let timestamp = metric
                .timestamp
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs();

            eprintln!(
                "### Metric: {} = {} (labels: {:?}) at {}",
                metric.info.name, metric.value, metric.labels, timestamp
            );
        }

        Ok(())
    }
}

impl Exporter for Gcp {
    fn export(&self, metrics: Vec<CollectedMetric>) {
        eprintln!("=== GCP EXPORT ===");
        let metrics_clone = metrics;
        let exporter = self.clone();

        tokio::spawn(async move {
            if let Err(err) = exporter.export_metrics(metrics_clone).await {
                log::error!("Failed to export metrics to GCP: {}", err);
            }
        });
    }
}
