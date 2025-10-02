use crate::{
    encore::runtime::v1 as pb,
    metadata::{process_env_substitution, ContainerMetadata},
    metrics::{exporter, registry::Registry},
};
use std::sync::Arc;
use std::time::Duration;

/// Rust-idiomatic provider system using enums and pattern matching
#[derive(Debug, Clone)]
enum ProviderType {
    Gcp(pb::metrics_provider::GcpCloudMonitoring),
    EncoreCloud(pb::metrics_provider::GcpCloudMonitoring),
    // Future providers will be added here:
    // Aws(pb::metrics_provider::AwsCloudWatch),
    // Prometheus(pb::metrics_provider::PrometheusRemoteWrite),
    // Datadog(pb::metrics_provider::Datadog),
}

impl ProviderType {
    fn from_config(provider: &pb::MetricsProvider) -> Option<Self> {
        match &provider.provider {
            Some(pb::metrics_provider::Provider::Gcp(config)) => Some(Self::Gcp(config.clone())),
            Some(pb::metrics_provider::Provider::EncoreCloud(config)) => {
                Some(Self::EncoreCloud(config.clone()))
            }
            _ => {
                log::warn!("Unsupported metrics provider: {:?}", provider.provider);
                None
            }
        }
    }

    fn create_exporter(&self) -> Arc<dyn Exporter + Send + Sync> {
        match self {
            Self::Gcp(config) => Self::create_gcp_exporter(config),
            Self::EncoreCloud(config) => Self::create_encore_cloud_exporter(config),
        }
    }

    fn create_gcp_exporter(
        config: &pb::metrics_provider::GcpCloudMonitoring,
    ) -> Arc<dyn Exporter + Send + Sync> {
        let mut labels = config.monitored_resource_labels.clone();

        // Process environment variable substitution
        process_env_substitution(&mut labels);

        // Add container instance ID to node_id if present
        if let Some(node_id) = labels.get("node_id").cloned() {
            let instance_id = ContainerMetadata::generate_instance_id();
            labels.insert(
                "node_id".to_string(),
                format!("{}-{}", node_id, instance_id),
            );
        }

        Arc::new(exporter::Gcp::new(
            config.project_id.clone(),
            config.monitored_resource_type.clone(),
            labels,
            config.metric_names.clone(),
        ))
    }

    fn create_encore_cloud_exporter(
        config: &pb::metrics_provider::GcpCloudMonitoring,
    ) -> Arc<dyn Exporter + Send + Sync> {
        let mut labels = config.monitored_resource_labels.clone();

        // Process environment variable substitution
        process_env_substitution(&mut labels);

        // For Encore Cloud, node_id is required
        if let Some(node_id) = labels.get("node_id").cloned() {
            let instance_id = ContainerMetadata::generate_instance_id();
            labels.insert(
                "node_id".to_string(),
                format!("{}-{}", node_id, instance_id),
            );
        } else {
            log::error!("Missing node_id in Encore Cloud metrics configuration");
            // Still create the exporter but it may not work properly
        }

        // Encore Cloud uses the same GCP exporter underneath
        Arc::new(exporter::Gcp::new(
            config.project_id.clone(),
            config.monitored_resource_type.clone(),
            labels,
            config.metric_names.clone(),
        ))
    }
}

#[derive(Clone)]
pub struct Manager {
    exporter: Option<std::sync::Arc<dyn Exporter>>,
    registry: Registry,
}

impl Manager {
    pub fn new() -> Self {
        let registry = Registry::new();

        // Set our registry as the global metrics recorder
        if let Err(e) = metrics::set_global_recorder(registry.clone()) {
            log::warn!("Failed to set metrics recorder: {}", e);
        }

        Self {
            exporter: None,
            registry,
        }
    }

    pub fn from_runtime_config(runtime_config: &pb::RuntimeConfig) -> Self {
        let mut manager = Self::new();

        eprintln!("=== FROM RUNTIME CONFIG ===");
        eprintln!("=== {runtime_config:?} ===");
        // Extract observability config
        if let Some(deployment) = &runtime_config.deployment {
            if let Some(observability) = &deployment.observability {
                // Process metrics providers using enum-based dispatch
                for metrics_provider in &observability.metrics {
                    if let Some(provider_type) = ProviderType::from_config(metrics_provider) {
                        manager.exporter = Some(provider_type.create_exporter());
                        break; // Take the first valid provider
                    }
                }

                // Start collection if we have an exporter
                if manager.exporter.is_some() {
                    let collection_interval = observability
                        .metrics
                        .first()
                        .and_then(|p| p.collection_interval.as_ref())
                        .map(|d| {
                            Duration::from_secs(d.seconds as u64 + d.nanos as u64 / 1_000_000_000)
                        })
                        .unwrap_or(Duration::from_secs(60)); // Default to 1 minute

                    manager.start_collection_loop(collection_interval);
                }
            }
        }

        // Manager created successfully (debug removed due to non-Debug Registry)
        manager
    }

    pub fn with_exporter(mut self, exporter: std::sync::Arc<dyn Exporter + Send + Sync>) -> Self {
        self.exporter = Some(exporter);
        self
    }

    pub fn collect_and_export(&self) {
        eprintln!("=== COLLECT and EXPORT ===");
        let metrics = self.registry.collect();
        if let Some(ref exporter) = self.exporter {
            eprintln!("=== EXPORTER {exporter:?} ===");
            exporter.export(metrics);
        }
    }

    pub fn collect_metrics(&self) -> Vec<crate::metrics::CollectedMetric> {
        self.registry.collect()
    }

    pub fn start_collection_loop(&self, interval: std::time::Duration) {
        eprintln!("=== START COLLECT LOOP, interval {interval:?} ===");
        let manager_clone = self.clone();
        tokio::spawn(async move {
            let mut interval_timer = tokio::time::interval(interval);
            loop {
                interval_timer.tick().await;
                manager_clone.collect_and_export();
            }
        });
    }
}

impl Default for Manager {
    fn default() -> Self {
        Self::new()
    }
}

pub trait Exporter: Send + Sync + std::fmt::Debug {
    fn export(&self, metrics: Vec<crate::metrics::CollectedMetric>);
}
