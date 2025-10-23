use crate::metadata::ContainerMetaClient;
use crate::metrics::exporter::Exporter;
use crate::metrics::{CollectedMetric, MetricValue};
use anyhow::Context;
use aws_sdk_cloudwatch as cloudwatch;
use aws_sdk_cloudwatch::types::{Dimension, MetricDatum};
use std::sync::Arc;
use std::time::SystemTime;

#[derive(Debug)]
pub struct Aws {
    client: Arc<LazyCloudWatchClient>,
    namespace: String,
    container_meta_client: ContainerMetaClient,
    container_dims: tokio::sync::OnceCell<Arc<Vec<Dimension>>>,
}

#[derive(Debug)]
struct LazyCloudWatchClient {
    cell: tokio::sync::OnceCell<anyhow::Result<cloudwatch::Client>>,
}

impl LazyCloudWatchClient {
    fn new() -> Self {
        Self {
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get(&self) -> &anyhow::Result<cloudwatch::Client> {
        self.cell
            .get_or_init(|| async {
                let config = aws_config::defaults(aws_config::BehaviorVersion::v2025_08_07())
                    .load()
                    .await;

                Ok(cloudwatch::Client::new(&config))
            })
            .await
    }
}

impl Aws {
    pub fn new(namespace: String, container_meta_client: ContainerMetaClient) -> Self {
        Self {
            client: Arc::new(LazyCloudWatchClient::new()),
            namespace,
            container_meta_client,
            container_dims: tokio::sync::OnceCell::new(),
        }
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> anyhow::Result<()> {
        if metrics.is_empty() {
            return Ok(());
        }

        let client = match self.client.get().await {
            Ok(client) => client,
            Err(e) => {
                log::error!("failed to get CloudWatch client: {}", e);
                return Err(anyhow::anyhow!("failed to get CloudWatch client: {}", e));
            }
        };

        log::trace!(
            "Exporting {} metrics to AWS CloudWatch namespace {}",
            metrics.len(),
            self.namespace
        );

        let metric_data = self.get_metric_data(metrics).await;

        // Send metrics in batches (CloudWatch allows up to 1000 metrics per request)
        for batch in metric_data.chunks(1000) {
            if let Err(e) = self.send_metric_batch(client, batch.to_vec()).await {
                log::error!("failed to export metrics batch: {}", e);
            }
        }

        Ok(())
    }

    async fn container_dimensions(&self) -> Arc<Vec<Dimension>> {
        self.container_dims
            .get_or_try_init(|| async {
                let container_metadata = self.container_meta_client.collect().await?;
                anyhow::Ok(Arc::new(
                    container_metadata
                        .labels()
                        .into_iter()
                        .map(|(key, value)| Dimension::builder().name(key).value(value).build())
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
                        .map(|(key, value)| Dimension::builder().name(key).value(value).build())
                        .collect(),
                )
            })
    }

    async fn get_metric_data(&self, collected: Vec<CollectedMetric>) -> Vec<MetricDatum> {
        let now = SystemTime::now();
        let mut data: Vec<MetricDatum> = Vec::with_capacity(collected.len());

        let container_dims = self.container_dimensions().await;
        let container_dims_len = container_dims.len();

        for metric in collected {
            let metric_name = metric.key.name().to_string();

            let mut dimensions: Vec<Dimension> =
                Vec::with_capacity(container_dims_len + metric.key.labels().count());

            // Add container metadata dimensions
            dimensions.extend(container_dims.iter().cloned());

            for label in metric.key.labels() {
                let value = label.value();
                if value.is_empty() {
                    log::warn!(
                        "Skipping empty label '{}' for metric '{}' - CloudWatch does not support empty dimension values",
                        label.key(),
                        metric_name
                    );
                    continue;
                }

                dimensions.push(Dimension::builder().name(label.key()).value(value).build());
            }

            let value = match metric.value {
                MetricValue::CounterU64(val) => val as f64,
                MetricValue::CounterI64(val) => val as f64,
                MetricValue::GaugeF64(val) => val,
                MetricValue::GaugeU64(val) => val as f64,
                MetricValue::GaugeI64(val) => val as f64,
            };

            let mut datum_builder = MetricDatum::builder()
                .metric_name(metric_name)
                .timestamp(aws_smithy_types::DateTime::from(now))
                .value(value)
                .set_dimensions(Some(dimensions));

            // For cumulative counters, include the start time
            if matches!(
                metric.value,
                MetricValue::CounterU64(_) | MetricValue::CounterI64(_)
            ) {
                // CloudWatch uses storage resolution to determine how data is aggregated
                // For counters, we use high resolution (1 second) to better track cumulative values
                datum_builder = datum_builder.storage_resolution(1);
            }

            data.push(datum_builder.build())
        }

        data
    }

    async fn send_metric_batch(
        &self,
        client: &cloudwatch::Client,
        metric_data: Vec<MetricDatum>,
    ) -> Result<(), anyhow::Error> {
        client
            .put_metric_data()
            .namespace(&self.namespace)
            .set_metric_data(Some(metric_data))
            .send()
            .await
            .context("send metrics to CloudWatch")?;

        Ok(())
    }
}

#[async_trait::async_trait]
impl Exporter for Aws {
    async fn export(&self, metrics: Vec<CollectedMetric>) {
        if let Err(err) = self.export_metrics(metrics).await {
            log::error!("Failed to export metrics to AWS CloudWatch: {}", err);
        }
    }
}
