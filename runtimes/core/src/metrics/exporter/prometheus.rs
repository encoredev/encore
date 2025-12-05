use crate::encore::runtime::v1 as pb;
use crate::metadata::ContainerMetaClient;
use crate::metrics::exporter::Exporter;
use crate::metrics::{CollectedMetric, MetricValue};
use crate::secrets;
use anyhow::Context;
use prost::Message;
use std::sync::Arc;
use std::time::SystemTime;
use tokio::sync::OnceCell;
use url::Url;

#[derive(Debug)]
pub struct Prometheus {
    client: reqwest::Client,
    remote_write_url: Url,
    container_meta_client: ContainerMetaClient,
    container_labels: OnceCell<Arc<Vec<prompb::Label>>>,
}

impl Prometheus {
    pub fn new(
        provider_cfg: &pb::metrics_provider::PrometheusRemoteWrite,
        secrets: &secrets::Manager,
        container_meta_client: ContainerMetaClient,
    ) -> anyhow::Result<Self> {
        let remote_write_url = match &provider_cfg.remote_write_url {
            Some(data) => {
                let secret = secrets.load(data.clone());
                let url_bytes = secret
                    .get()
                    .context("failed to resolve Prometheus Remote Write Url")?;
                let s = std::str::from_utf8(url_bytes)
                    .context("Prometheus Remote Write Url is not valid UTF-8")?;
                Url::parse(s).context("Prometheus Remote Write Url not valid")?
            }
            None => {
                return Err(anyhow::anyhow!("Prometheus Remote Write Url not provided"));
            }
        };

        Ok(Self {
            client: reqwest::Client::new(),
            remote_write_url,
            container_meta_client,
            container_labels: OnceCell::new(),
        })
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> anyhow::Result<()> {
        if metrics.is_empty() {
            return Ok(());
        }

        log::trace!(
            "Exporting {} metrics to Prometheus remote write host {}",
            metrics.len(),
            self.remote_write_url.host_str().unwrap_or_default()
        );

        let time_series = self.get_metric_data(metrics).await;

        // Create WriteRequest with the time series
        let write_request = prompb::WriteRequest {
            timeseries: time_series,
            metadata: vec![],
        };

        // Serialize to protobuf
        let mut proto_buf = Vec::new();
        write_request
            .encode(&mut proto_buf)
            .context("marshal metrics into Protobuf")?;

        // Compress with Snappy
        let mut encoder = snap::raw::Encoder::new();
        let encoded = encoder
            .compress_vec(&proto_buf)
            .context("compress metrics with Snappy")?;

        // Send HTTP request
        let response = self
            .client
            .post(self.remote_write_url.clone())
            .header("Content-Type", "application/x-protobuf")
            .header("Content-Encoding", "snappy")
            .header("User-Agent", "encore")
            .header("X-Prometheus-Remote-Write-Version", "0.1.0")
            .body(encoded)
            .send()
            .await
            .context("send metrics to Prometheus remote write destination")?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response
                .text()
                .await
                .unwrap_or_else(|_| "<unable to read response body>".to_string());
            anyhow::bail!(
                "Prometheus remote write returned non-success status {}: {}",
                status,
                body
            );
        }

        Ok(())
    }

    async fn container_labels(&self) -> Arc<Vec<prompb::Label>> {
        self.container_labels
            .get_or_try_init(|| async {
                let container_metadata = self.container_meta_client.collect().await?;
                anyhow::Ok(Arc::new(
                    container_metadata
                        .labels()
                        .into_iter()
                        .map(|(name, value)| prompb::Label { name, value })
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
                        .map(|(name, value)| prompb::Label { name, value })
                        .collect(),
                )
            })
    }

    async fn get_metric_data(&self, collected: Vec<CollectedMetric>) -> Vec<prompb::TimeSeries> {
        let now = SystemTime::now();
        let timestamp = from_time(now);
        let mut data: Vec<prompb::TimeSeries> = Vec::with_capacity(collected.len());

        let container_labels = self.container_labels().await;
        let container_labels_len = container_labels.len();

        for metric in collected {
            let metric_name = metric.key.name().to_string();

            // Build labels: container metadata + metric labels + __name__
            let mut labels: Vec<prompb::Label> =
                Vec::with_capacity(container_labels_len + metric.key.labels().len() + 1);

            // Add container metadata labels
            labels.extend(container_labels.iter().cloned());

            // Add metric-specific labels
            for label in metric.key.labels() {
                labels.push(prompb::Label {
                    name: label.key().to_string(),
                    value: label.value().to_string(),
                });
            }

            // Add __name__ label for the metric name
            labels.push(prompb::Label {
                name: "__name__".to_string(),
                value: metric_name,
            });

            // Convert metric value to float64
            let value = match metric.value {
                MetricValue::CounterU64(val) => val as f64,
                MetricValue::CounterI64(val) => val as f64,
                MetricValue::GaugeF64(val) => val,
                MetricValue::GaugeU64(val) => val as f64,
                MetricValue::GaugeI64(val) => val as f64,
            };

            data.push(prompb::TimeSeries {
                labels,
                samples: vec![prompb::Sample { value, timestamp }],
                exemplars: vec![],
                histograms: vec![],
            });
        }

        data
    }
}

#[async_trait::async_trait]
impl Exporter for Prometheus {
    async fn export(&self, metrics: Vec<CollectedMetric>) {
        if let Err(err) = self.export_metrics(metrics).await {
            log::error!("Failed to export metrics to Prometheus: {}", err);
        }
    }
}

/// Convert SystemTime to Prometheus timestamp (milliseconds since Unix epoch)
fn from_time(t: SystemTime) -> i64 {
    match t.duration_since(SystemTime::UNIX_EPOCH) {
        Ok(duration) => {
            let secs = duration.as_secs() as i64;
            let nanos = duration.subsec_nanos() as i64;
            secs * 1000 + nanos / 1_000_000
        }
        Err(_) => 0, // If time is before Unix epoch, return 0
    }
}

#[allow(dead_code)]
mod prompb {
    include!(concat!(env!("OUT_DIR"), "/prometheus.rs"));
}
