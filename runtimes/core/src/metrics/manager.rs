use crate::{
    encore::runtime::v1::{self as pb, Environment},
    metadata::{process_env_substitution, ContainerMetaClient},
    metrics::{
        exporter::{self, Exporter},
        registry::Registry,
    },
    secrets,
};
use std::sync::Arc;
use std::time::Duration;

#[derive(Debug, Clone)]
enum ProviderType {
    Gcp(pb::metrics_provider::GcpCloudMonitoring),
    EncoreCloud(pb::metrics_provider::GcpCloudMonitoring),
    Aws(pb::metrics_provider::AwsCloudWatch),
    Datadog(pb::metrics_provider::Datadog),
    Prometheus(pb::metrics_provider::PrometheusRemoteWrite),
}

impl ProviderType {
    fn from_config(provider: &pb::MetricsProvider) -> Option<Self> {
        match &provider.provider {
            Some(pb::metrics_provider::Provider::Gcp(config)) => Some(Self::Gcp(config.clone())),
            Some(pb::metrics_provider::Provider::EncoreCloud(config)) => {
                Some(Self::EncoreCloud(config.clone()))
            }
            Some(pb::metrics_provider::Provider::Aws(config)) => Some(Self::Aws(config.clone())),
            Some(pb::metrics_provider::Provider::Datadog(config)) => {
                Some(Self::Datadog(config.clone()))
            }
            Some(pb::metrics_provider::Provider::PromRemoteWrite(config)) => {
                Some(Self::Prometheus(config.clone()))
            }
            None => {
                log::warn!("no metrics provider configured");
                None
            }
        }
    }

    fn create_exporter(
        &self,
        env: &Environment,
        secrets: &secrets::Manager,
        http_client: &reqwest::Client,
    ) -> anyhow::Result<Arc<dyn Exporter + Send + Sync>> {
        match self {
            Self::Gcp(config) | Self::EncoreCloud(config) => {
                Ok(Self::create_gcp_exporter(config, env, http_client))
            }
            Self::Aws(config) => Ok(Self::create_aws_exporter(config, env, http_client)),
            Self::Datadog(config) => {
                Self::create_datadog_exporter(config, secrets, env, http_client)
            }
            Self::Prometheus(config) => {
                Self::create_prometheus_exporter(config, secrets, env, http_client)
            }
        }
    }

    fn create_prometheus_exporter(
        provider_cfg: &pb::metrics_provider::PrometheusRemoteWrite,
        secrets: &secrets::Manager,
        env: &Environment,
        http_client: &reqwest::Client,
    ) -> anyhow::Result<Arc<dyn Exporter + Send + Sync>> {
        let container_meta_client = ContainerMetaClient::new(env.clone(), http_client.clone());
        Ok(Arc::new(exporter::Prometheus::new(
            provider_cfg,
            secrets,
            container_meta_client,
        )?))
    }

    fn create_datadog_exporter(
        provider_cfg: &pb::metrics_provider::Datadog,
        secrets: &secrets::Manager,
        env: &Environment,
        http_client: &reqwest::Client,
    ) -> anyhow::Result<Arc<dyn Exporter + Send + Sync>> {
        let container_meta_client = ContainerMetaClient::new(env.clone(), http_client.clone());
        Ok(Arc::new(exporter::Datadog::new(
            provider_cfg,
            secrets,
            container_meta_client,
        )?))
    }

    fn create_aws_exporter(
        provider_cfg: &pb::metrics_provider::AwsCloudWatch,
        env: &Environment,
        http_client: &reqwest::Client,
    ) -> Arc<dyn Exporter + Send + Sync> {
        let container_meta_client = ContainerMetaClient::new(env.clone(), http_client.clone());
        Arc::new(exporter::Aws::new(
            provider_cfg.namespace.clone(),
            container_meta_client,
        ))
    }

    fn create_gcp_exporter(
        provider_cfg: &pb::metrics_provider::GcpCloudMonitoring,
        env: &Environment,
        http_client: &reqwest::Client,
    ) -> Arc<dyn Exporter + Send + Sync> {
        let container_meta_client = ContainerMetaClient::new(env.clone(), http_client.clone());

        let mut labels = provider_cfg.monitored_resource_labels.clone();
        process_env_substitution(&mut labels);

        Arc::new(exporter::Gcp::new(
            provider_cfg.project_id.clone(),
            provider_cfg.monitored_resource_type.clone(),
            labels,
            provider_cfg.metric_names.clone(),
            container_meta_client,
        ))
    }
}

#[derive(Clone)]
pub struct Manager {
    exporter: Option<Arc<dyn Exporter>>,
    registry: Arc<Registry>,
}

impl Manager {
    pub fn new() -> Self {
        let registry = Arc::new(Registry::new());

        Self {
            exporter: None,
            registry,
        }
    }

    pub fn registry(&self) -> &Arc<Registry> {
        &self.registry
    }

    pub fn from_runtime_config(
        observability: &pb::Observability,
        environment: &pb::Environment,
        secrets: &secrets::Manager,
        http_client: &reqwest::Client,
        runtime_handle: tokio::runtime::Handle,
    ) -> Self {
        let mut manager = Self::new();

        for metrics_provider in &observability.metrics {
            if let Some(provider_type) = ProviderType::from_config(metrics_provider) {
                match provider_type.create_exporter(environment, secrets, http_client) {
                    Ok(exporter) => {
                        manager.exporter = Some(exporter);
                        break; // Take the first valid provider
                    }
                    Err(err) => {
                        log::error!("Failed to create metrics exporter: {}", err);
                    }
                }
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
