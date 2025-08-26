use crate::metadata::ContainerMetadata;
use crate::metrics::manager::Exporter;
use crate::metrics::{CollectedMetric, MetricValue};
use anyhow::Context;
use dashmap::DashMap;
use google_cloud_api::model::metric_descriptor::{MetricKind, ValueType};
use google_cloud_api::model::{Metric, MonitoredResource};
use google_cloud_monitoring_v3::client::MetricService;
use google_cloud_monitoring_v3::model::{Point, TimeInterval, TimeSeries, TypedValue};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, SystemTime};

#[derive(Clone)]
pub struct Gcp {
    client: Arc<LazyMonitoringClient>,
    project_id: String,
    monitored_resource_type: String,
    monitored_resource_labels: HashMap<String, String>,
    metric_names: HashMap<String, String>,
    container_meta: ContainerMetadata,
    first_seen: Arc<DashMap<u64, SystemTime>>,
}

impl std::fmt::Debug for Gcp {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Gcp")
            .field("project_id", &self.project_id)
            .field("monitored_resource_type", &self.monitored_resource_type)
            .field("monitored_resource_labels", &self.monitored_resource_labels)
            .field("metric_names", &self.metric_names)
            .field("container_meta", &self.container_meta)
            .field("first_seen", &self.first_seen)
            .finish()
    }
}
struct LazyMonitoringClient {
    cell: tokio::sync::OnceCell<anyhow::Result<MetricService>>,
}

impl LazyMonitoringClient {
    fn new() -> Self {
        Self {
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get(&self) -> &anyhow::Result<MetricService> {
        self.cell
            .get_or_init(|| async {
                MetricService::builder()
                    .build()
                    .await
                    .inspect_err(|e| log::error!("Failed to create GCP monitoring client: {e:?}"))
                    .context("create monitoring client")
            })
            .await
    }
}

impl Gcp {
    pub fn new(
        project_id: String,
        monitored_resource_type: String,
        monitored_resource_labels: HashMap<String, String>,
        metric_names: HashMap<String, String>,
        container_meta: ContainerMetadata,
        first_seen: Arc<DashMap<u64, SystemTime>>,
    ) -> Self {
        Self {
            client: Arc::new(LazyMonitoringClient::new()),
            project_id,
            monitored_resource_type,
            monitored_resource_labels,
            metric_names,
            container_meta,
            first_seen,
        }
    }

    async fn export_metrics(&self, metrics: Vec<CollectedMetric>) -> Result<(), anyhow::Error> {
        if metrics.is_empty() {
            return Ok(());
        }

        let client = match self.client.get().await {
            Ok(client) => client,
            Err(e) => {
                log::error!("Failed to get monitoring client: {}", e);
                return Err(anyhow::anyhow!("Failed to get monitoring client: {}", e));
            }
        };

        log::info!(
            "Exporting {} metrics to GCP project {}",
            metrics.len(),
            self.project_id
        );

        // Convert our metrics to Google Cloud TimeSeries format
        let time_series = self.get_metric_data(metrics);
        // TODO(fredr): add sys-metrics

        // Send metrics in batches (Google Cloud allows up to 200 time series per request)
        for batch in time_series.chunks(200) {
            if let Err(e) = self.send_time_series_batch(client, batch.to_vec()).await {
                log::error!("Failed to export metrics batch: {}", e);
                // Continue with remaining batches even if one fails
            }
        }

        Ok(())
    }

    fn get_metric_data(&self, collected: Vec<CollectedMetric>) -> Vec<TimeSeries> {
        let end_time = SystemTime::now();
        let ts_end_time: google_cloud_wkt::Timestamp = end_time.try_into().unwrap_or_default();

        let mut data: Vec<TimeSeries> = Vec::with_capacity(collected.len());
        let container_labels = self.container_meta.labels();
        let container_labels_len = container_labels.len();

        for metric in collected {
            let cloud_metric_name = match self.metric_names.get(&metric.name) {
                Some(name) => name,
                None => continue,
            };

            // Pre-allocate exact capacity for labels
            let mut labels: HashMap<String, String> =
                HashMap::with_capacity(container_labels_len + metric.labels.len());
            labels.extend(container_labels.clone());
            labels.extend(metric.labels.clone());

            let (kind, value_type, typed_value, interval) = match metric.value {
                MetricValue::Counter(val) => {
                    let start_time: google_cloud_wkt::Timestamp = self
                        .first_seen
                        .get(&metric.key.get_hash())
                        .map(|entry| *entry)
                        .unwrap_or_else(|| end_time - Duration::from_millis(1)) // this should never happen, but just so we dont have to handle errors
                        .try_into()
                        .unwrap_or_default();

                    (
                        MetricKind::Cumulative,
                        ValueType::Int64,
                        TypedValue::new().set_int64_value(val as i64),
                        TimeInterval::new()
                            .set_start_time(start_time)
                            .set_end_time(ts_end_time),
                    )
                }
                MetricValue::Gauge(val) => (
                    MetricKind::Gauge,
                    ValueType::Double,
                    TypedValue::new().set_double_value(val),
                    TimeInterval::new().set_end_time(ts_end_time),
                ),
            };

            data.push(
                TimeSeries::new()
                    .set_metric_kind(kind)
                    .set_metric(
                        Metric::new()
                            .set_type(format!("custom.googleapis.com/{}", cloud_metric_name))
                            .set_labels(labels),
                    )
                    .set_resource(
                        MonitoredResource::new()
                            .set_type(&self.monitored_resource_type)
                            .set_labels(&self.monitored_resource_labels),
                    )
                    .set_value_type(value_type)
                    .set_points(vec![Point::new()
                        .set_interval(interval)
                        .set_value(typed_value)]),
            );
        }

        data
    }

    async fn send_time_series_batch(
        &self,
        client: &MetricService,
        time_series: Vec<google_cloud_monitoring_v3::model::TimeSeries>,
    ) -> Result<(), anyhow::Error> {
        client
            .create_time_series()
            .set_name(format!("projects/{}", self.project_id))
            .set_time_series(time_series)
            .send()
            .await
            .map_err(anyhow::Error::new)?;

        log::debug!("Successfully exported metrics batch to GCP");
        Ok(())
    }
}

#[async_trait::async_trait]
impl Exporter for Gcp {
    async fn export(&self, metrics: Vec<CollectedMetric>) {
        if let Err(err) = self.export_metrics(metrics).await {
            log::error!("Failed to export metrics to GCP: {}", err);
        }
    }
}
