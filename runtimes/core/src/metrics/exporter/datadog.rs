use crate::{
    encore::runtime::v1 as pb,
    metadata::ContainerMetaClient,
    metrics::{exporter::Exporter, CollectedMetric, MetricValue},
    secrets,
};
use anyhow::Context;
use dashmap::DashMap;
use datadog_api_client::datadog;
use datadog_api_client::datadogV2::api_metrics::{MetricsAPI, SubmitMetricsOptionalParams};
use datadog_api_client::datadogV2::model::{
    MetricIntakeType, MetricPayload, MetricPoint, MetricSeries,
};
use std::sync::{Arc, Mutex};
use std::time::SystemTime;

pub struct Datadog {
    client: Arc<LazyDatadogClient>,
    container_meta_client: ContainerMetaClient,
    last_export: Arc<Mutex<i64>>,
    last_value: Arc<DashMap<u64, f64>>,
    container_tags: tokio::sync::OnceCell<Arc<Vec<String>>>,
}

#[derive(Clone)]
struct DatadogClient {
    config: datadog::Configuration,
}

impl DatadogClient {
    async fn send_metrics(&self, metric_series: Vec<MetricSeries>) -> Result<(), anyhow::Error> {
        let api = MetricsAPI::with_config(self.config.clone());
        let payload = MetricPayload::new(metric_series);

        api.submit_metrics(payload, SubmitMetricsOptionalParams::default())
            .await
            .context("submit metrics to Datadog")?;

        Ok(())
    }
}

struct LazyDatadogClient {
    cell: tokio::sync::OnceCell<DatadogClient>,
    site: String,
    api_key: String,
}

impl LazyDatadogClient {
    fn new(site: String, api_key: String) -> Self {
        Self {
            cell: tokio::sync::OnceCell::new(),
            site,
            api_key,
        }
    }

    async fn get(&self) -> &DatadogClient {
        self.cell
            .get_or_init(|| async {
                // Create Datadog API client configuration
                let mut configuration = datadog::Configuration::new();
                configuration
                    .server_variables
                    .insert("site".to_string(), self.site.clone());
                configuration.set_auth_key(
                    "apiKeyAuth",
                    datadog::APIKey {
                        key: self.api_key.clone(),
                        prefix: String::new(),
                    },
                );

                DatadogClient {
                    config: configuration,
                }
            })
            .await
    }
}

impl Datadog {
    pub fn new(
        provider_cfg: &pb::metrics_provider::Datadog,
        secrets: &secrets::Manager,
        container_meta_client: ContainerMetaClient,
    ) -> anyhow::Result<Self> {
        let api_key = match &provider_cfg.api_key {
            Some(data) => {
                let secret = secrets.load(data.clone());
                let api_key_bytes = secret.get().context("failed to resolve Datadog API key")?;
                std::str::from_utf8(api_key_bytes)
                    .context("Datadog API key is not valid UTF-8")?
                    .to_string()
            }
            None => {
                return Err(anyhow::anyhow!("Datadog API key not provided"));
            }
        };

        Ok(Self {
            client: Arc::new(LazyDatadogClient::new(provider_cfg.site.clone(), api_key)),
            container_meta_client,
            last_export: Arc::new(Mutex::new(
                SystemTime::now()
                    .duration_since(SystemTime::UNIX_EPOCH)
                    .expect("system time before Unix epoch")
                    .as_secs() as i64,
            )),
            last_value: Arc::new(DashMap::new()),
            container_tags: tokio::sync::OnceCell::new(),
        })
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> anyhow::Result<()> {
        if metrics.is_empty() {
            return Ok(());
        }

        let client = self.client.get().await;

        log::trace!(
            "Exporting {} metrics to Datadog site {}",
            metrics.len(),
            self.client.site
        );

        let now = SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .expect("system time before Unix epoch")
            .as_secs() as i64;

        let metric_series = self.get_metric_data(metrics, now).await;

        if !metric_series.is_empty() {
            client.send_metrics(metric_series).await?;
        }

        // Update last export time
        if let Ok(mut last_export) = self.last_export.lock() {
            *last_export = now;
        }

        Ok(())
    }

    async fn container_tags_vec(&self) -> Arc<Vec<String>> {
        self.container_tags
            .get_or_try_init(|| async {
                let container_metadata = self.container_meta_client.collect().await?;
                anyhow::Ok(Arc::new(
                    container_metadata
                        .labels()
                        .into_iter()
                        .map(|(key, value)| format!("{}:{}", key, value))
                        .collect(),
                ))
            })
            .await
            .map(Arc::clone)
            .unwrap_or_else(|e| {
                log::warn!("failed fetching container metadata: {e}, using fallback");
                Arc::new(
                    self.container_meta_client
                        .fallback()
                        .labels()
                        .into_iter()
                        .map(|(key, value)| format!("{}:{}", key, value))
                        .collect(),
                )
            })
    }

    async fn get_metric_data(
        &self,
        collected: Vec<CollectedMetric>,
        now: i64,
    ) -> Vec<MetricSeries> {
        let mut data: Vec<MetricSeries> = Vec::with_capacity(collected.len());

        let container_tags = self.container_tags_vec().await;
        let container_tags_len = container_tags.len();

        let last_export = self.last_export.lock().ok().map(|t| *t).unwrap_or(now);
        let interval = now - last_export;

        for metric in collected {
            let metric_name = metric.key.name().to_string();

            // Build tags: container metadata + metric labels
            let mut tags: Vec<String> =
                Vec::with_capacity(container_tags_len + metric.key.labels().count());
            // Add container metadata tags
            tags.extend(container_tags.iter().cloned());

            for label in metric.key.labels() {
                tags.push(format!("{}:{}", label.key(), label.value()));
            }

            let (metric_type, value) = match metric.value {
                MetricValue::CounterU64(val) => {
                    let value = val as f64;
                    let key = metric.key.get_hash();
                    let last_val = self.last_value.get(&key).map(|v| *v).unwrap_or(0.0);
                    self.last_value.insert(key, value);
                    let delta = value - last_val;
                    (MetricIntakeType::COUNT, delta)
                }
                MetricValue::CounterI64(val) => {
                    let value = val as f64;
                    let key = metric.key.get_hash();
                    let last_val = self.last_value.get(&key).map(|v| *v).unwrap_or(0.0);
                    self.last_value.insert(key, value);
                    let delta = value - last_val;
                    (MetricIntakeType::COUNT, delta)
                }
                MetricValue::GaugeF64(val) => (MetricIntakeType::GAUGE, val),
                MetricValue::GaugeU64(val) => (MetricIntakeType::GAUGE, val as f64),
                MetricValue::GaugeI64(val) => (MetricIntakeType::GAUGE, val as f64),
            };

            let point = MetricPoint::new().timestamp(now).value(value);

            let series = MetricSeries::new(metric_name, vec![point])
                .type_(metric_type)
                .interval(interval)
                .tags(tags);

            data.push(series);
        }

        data
    }
}

#[async_trait::async_trait]
impl Exporter for Datadog {
    async fn export(&self, metrics: Vec<CollectedMetric>) {
        if let Err(err) = self.export_metrics(metrics).await {
            log::error!("Failed to export metrics to Datadog: {}", err);
        }
    }
}
