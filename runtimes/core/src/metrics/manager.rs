use crate::{
    encore::runtime::v1::{self as pb, Environment},
    metadata::{process_env_substitution, ContainerMetadata},
    metrics::{
        exporter::{self, Exporter},
        registry::Registry,
    },
};
use std::sync::Arc;
use std::time::Duration;

#[derive(Debug, Clone)]
enum ProviderType {
    Gcp(pb::metrics_provider::GcpCloudMonitoring),
    EncoreCloud(pb::metrics_provider::GcpCloudMonitoring),
    // TODO(fredr) add these:
    // - Aws(pb::metrics_provider::AwsCloudWatch),
    // - Prometheus(pb::metrics_provider::PrometheusRemoteWrite),
    // - Datadog(pb::metrics_provider::Datadog),
}

impl ProviderType {
    fn from_config(provider: &pb::MetricsProvider) -> Option<Self> {
        match &provider.provider {
            Some(pb::metrics_provider::Provider::Gcp(config)) => Some(Self::Gcp(config.clone())),
            Some(pb::metrics_provider::Provider::EncoreCloud(config)) => {
                Some(Self::EncoreCloud(config.clone()))
            }
            _ => {
                log::warn!("unsupported metrics provider: {:?}", provider.provider);
                None
            }
        }
    }

    fn create_exporter(
        &self,
        env: &Environment,
        http_client: &reqwest::Client,
        runtime_handle: tokio::runtime::Handle,
    ) -> Arc<dyn Exporter + Send + Sync> {
        match self {
            Self::Gcp(config) | Self::EncoreCloud(config) => {
                runtime_handle.block_on(Self::create_gcp_exporter(config, env, http_client))
            }
        }
    }

    async fn create_gcp_exporter(
        provider_cfg: &pb::metrics_provider::GcpCloudMonitoring,
        env: &Environment,
        http_client: &reqwest::Client,
    ) -> Arc<dyn Exporter + Send + Sync> {
        let container_meta = ContainerMetadata::collect(env, http_client)
            .await
            .unwrap_or_else(|_| ContainerMetadata {
                env_name: env.env_name.clone(),
                ..Default::default()
            });

        let mut labels = provider_cfg.monitored_resource_labels.clone();

        // Add container instance ID to node_id if present
        if let Some(node_id) = labels.get("node_id").cloned() {
            labels.insert(
                "node_id".to_string(),
                format!("{}-{}", node_id, container_meta.instance_id),
            );
        }

        process_env_substitution(&mut labels);

        Arc::new(exporter::Gcp::new(
            provider_cfg.project_id.clone(),
            provider_cfg.monitored_resource_type.clone(),
            labels,
            provider_cfg.metric_names.clone(),
            container_meta,
        ))
    }
}

#[derive(Clone)]
pub struct Manager {
    exporter: Option<Arc<dyn Exporter>>,
    registry: Registry,
}

impl Manager {
    pub fn new() -> Self {
        let registry = Registry::new();

        Self {
            exporter: None,
            registry,
        }
    }

    /// Get direct access to the registry
    pub fn registry(&self) -> &Registry {
        &self.registry
    }

    pub fn from_runtime_config(
        observability: &pb::Observability,
        environment: &pb::Environment,
        http_client: &reqwest::Client,
        runtime_handle: tokio::runtime::Handle,
    ) -> Self {
        let mut manager = Self::new();

        for metrics_provider in &observability.metrics {
            if let Some(provider_type) = ProviderType::from_config(metrics_provider) {
                manager.exporter = Some(provider_type.create_exporter(
                    environment,
                    http_client,
                    runtime_handle.clone(),
                ));
                break; // Take the first valid provider
            }
        }

        // Start collection if we have an exporter
        if manager.exporter.is_some() {
            let collection_interval = observability
                .metrics
                .first()
                .and_then(|p| p.collection_interval.as_ref())
                .and_then(|d| Duration::try_from(d.clone()).ok())
                .unwrap_or(Duration::from_secs(60)); // Default to 1 minute

            manager.start_collection_loop(runtime_handle, collection_interval);
        }

        manager
    }

    pub fn with_exporter(mut self, exporter: std::sync::Arc<dyn Exporter + Send + Sync>) -> Self {
        self.exporter = Some(exporter);
        self
    }

    pub async fn collect_and_export(&self) {
        let metrics = self.registry.collect();
        if let Some(ref exporter) = self.exporter {
            exporter.export(metrics).await;
        }
    }

    pub fn collect_metrics(&self) -> Vec<crate::metrics::CollectedMetric> {
        self.registry.collect()
    }

    pub fn start_collection_loop(
        &self,
        runtime_handle: tokio::runtime::Handle,
        interval: std::time::Duration,
    ) {
        let manager = self.clone();
        runtime_handle.spawn(async move {
            let mut interval_timer = tokio::time::interval(interval);
            loop {
                interval_timer.tick().await;
                manager.collect_and_export().await;
            }
        });
    }
}

impl Default for Manager {
    fn default() -> Self {
        Self::new()
    }
}
