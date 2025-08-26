use std::collections::HashMap;

use crate::metrics::manager::Exporter;

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

impl Exporter for Gcp {
    fn export(&self) {
        todo!()
    }
}
