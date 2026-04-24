use crate::encore::runtime::v1 as pb;
use crate::metrics::exporter::Exporter;
use crate::metrics::{CollectedMetric, MetricValue};
use anyhow::Context;
use azure_core::credentials::{TokenCredential, TokenRequestOptions};
use azure_identity::DefaultAzureCredential;
use serde::Serialize;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::OnceCell;

#[derive(Debug)]
pub struct AzureMonitor {
    config: pb::metrics_provider::AzureMonitor,
    http_client: reqwest::Client,
    credential: Arc<LazyCredential>,
}

#[derive(Debug)]
struct LazyCredential {
    cell: OnceCell<anyhow::Result<Arc<dyn TokenCredential>>>,
}

impl LazyCredential {
    fn new() -> Self {
        Self {
            cell: OnceCell::new(),
        }
    }

    async fn get(&self) -> &anyhow::Result<Arc<dyn TokenCredential>> {
        self.cell
            .get_or_init(|| async {
                let cred: Arc<dyn TokenCredential> = DefaultAzureCredential::new()
                    .context("create Azure DefaultAzureCredential")?;
                Ok(cred)
            })
            .await
    }
}

// Internal types for grouping metrics into per-name batches.
struct MetricSeries {
    dim_values: Vec<String>,
    value: f64,
}

struct MetricBatch {
    dim_names: Vec<String>,
    series: Vec<MetricSeries>,
}

// JSON payload types matching the Azure Monitor Custom Metrics REST API.
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-custom-overview
#[derive(Serialize)]
struct AzureCustomMetricPayload<'a> {
    time: &'a str,
    data: AzureCustomMetricData<'a>,
}

#[derive(Serialize)]
struct AzureCustomMetricData<'a> {
    #[serde(rename = "baseData")]
    base_data: AzureCustomMetricBaseData<'a>,
}

#[derive(Serialize)]
struct AzureCustomMetricBaseData<'a> {
    metric: &'a str,
    namespace: &'a str,
    #[serde(rename = "dimNames", skip_serializing_if = "Vec::is_empty")]
    dim_names: Vec<String>,
    series: Vec<AzureCustomMetricSeries>,
}

#[derive(Serialize)]
struct AzureCustomMetricSeries {
    #[serde(rename = "dimValues", skip_serializing_if = "Vec::is_empty")]
    dim_values: Vec<String>,
    sum: f64,
    count: i64,
    min: f64,
    max: f64,
}

impl AzureMonitor {
    pub fn new(config: pb::metrics_provider::AzureMonitor, http_client: reqwest::Client) -> Self {
        Self {
            config,
            http_client,
            credential: Arc::new(LazyCredential::new()),
        }
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> anyhow::Result<()> {
        if metrics.is_empty() {
            return Ok(());
        }

        log::trace!(
            "Exporting {} metrics to Azure Monitor namespace {}",
            metrics.len(),
            self.config.namespace
        );

        let now = chrono::Utc::now();
        let time_str = now.to_rfc3339();

        let batches = self.build_batches(metrics);

        let token = self.get_token().await?;

        for (metric_name, batch) in &batches {
            if let Err(e) = self
                .send_batch(&token, &time_str, metric_name, batch)
                .await
            {
                log::error!(
                    "Failed to send Azure Monitor metric {}: {}",
                    metric_name,
                    e
                );
            }
        }

        Ok(())
    }

    fn build_batches(&self, metrics: Vec<CollectedMetric>) -> HashMap<String, MetricBatch> {
        let mut batches: HashMap<String, MetricBatch> = HashMap::new();

        for metric in metrics {
            let name = metric.key.name().to_string();

            let labels: Vec<_> = metric.key.labels().collect();
            let dim_names: Vec<String> = labels.iter().map(|l| l.key().to_string()).collect();
            let dim_values: Vec<String> = labels.iter().map(|l| l.value().to_string()).collect();

            let value = match metric.value {
                MetricValue::CounterU64(v) => v as f64,
                MetricValue::CounterI64(v) => v as f64,
                MetricValue::GaugeF64(v) => v,
                MetricValue::GaugeU64(v) => v as f64,
                MetricValue::GaugeI64(v) => v as f64,
            };

            let batch = batches.entry(name).or_insert_with(|| MetricBatch {
                dim_names,
                series: Vec::new(),
            });

            batch.series.push(MetricSeries { dim_values, value });
        }

        batches
    }

    async fn get_token(&self) -> anyhow::Result<String> {
        let cred = match self.credential.get().await {
            Ok(cred) => cred,
            Err(e) => return Err(anyhow::anyhow!("azure credential unavailable: {}", e)),
        };

        let access_token = cred
            .get_token(
                &["https://monitoring.azure.com/.default"],
                None::<TokenRequestOptions>,
            )
            .await
            .context("get Azure Monitor bearer token")?;

        Ok(access_token.token.secret().to_string())
    }

    async fn send_batch(
        &self,
        token: &str,
        time_str: &str,
        metric_name: &str,
        batch: &MetricBatch,
    ) -> anyhow::Result<()> {
        if batch.series.is_empty() {
            return Ok(());
        }

        let api_series: Vec<AzureCustomMetricSeries> = batch
            .series
            .iter()
            .map(|s| AzureCustomMetricSeries {
                dim_values: s.dim_values.clone(),
                sum: s.value,
                count: 1,
                min: s.value,
                max: s.value,
            })
            .collect();

        let payload = AzureCustomMetricPayload {
            time: time_str,
            data: AzureCustomMetricData {
                base_data: AzureCustomMetricBaseData {
                    metric: metric_name,
                    namespace: &self.config.namespace,
                    dim_names: batch.dim_names.clone(),
                    series: api_series,
                },
            },
        };

        let url = format!(
            "https://{}.monitoring.azure.com/subscriptions/{}/resourceGroups/{}/providers/{}/{}/metrics",
            self.config.location,
            self.config.subscription_id,
            self.config.resource_group,
            self.config.resource_namespace,
            self.config.resource_name,
        );

        let resp = self
            .http_client
            .post(&url)
            .bearer_auth(token)
            .json(&payload)
            .send()
            .await
            .context("send Azure Monitor custom metric")?;

        let status = resp.status();
        if !status.is_success() {
            return Err(anyhow::anyhow!(
                "Azure Monitor returned status {} for metric {}",
                status,
                metric_name
            ));
        }

        Ok(())
    }
}

#[async_trait::async_trait]
impl Exporter for AzureMonitor {
    async fn export(&self, metrics: Vec<CollectedMetric>) {
        if let Err(err) = self.export_metrics(metrics).await {
            log::error!("Failed to export metrics to Azure Monitor: {}", err);
        }
    }
}
