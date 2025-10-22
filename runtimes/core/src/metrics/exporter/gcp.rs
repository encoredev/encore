use crate::metadata::ContainerMetaClient;
use crate::metrics::exporter::Exporter;
use crate::metrics::{CollectedMetric, MetricValue};
use anyhow::Context;
use google_cloud_api::model::metric_descriptor::{MetricKind, ValueType};
use google_cloud_api::model::{Metric, MonitoredResource};
use google_cloud_monitoring_v3::client::MetricService;
use google_cloud_monitoring_v3::model::{Point, TimeInterval, TimeSeries, TypedValue};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::SystemTime;
use tokio::sync::OnceCell;

type LabelPairs = Vec<(String, String)>;

#[derive(Debug)]
pub struct Gcp {
    client: Arc<LazyMonitoringClient>,
    project_id: String,
    monitored_resource_type: String,
    monitored_resource_labels: HashMap<String, String>,
    metric_names: HashMap<String, String>,
    container_meta_client: Arc<ContainerMetaClient>,
    container_labels: Arc<OnceCell<Arc<LabelPairs>>>,
}

#[derive(Debug)]
struct LazyMonitoringClient {
    cell: OnceCell<anyhow::Result<MetricService>>,
}

impl LazyMonitoringClient {
    fn new() -> Self {
        Self {
            cell: OnceCell::new(),
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
        container_meta_client: ContainerMetaClient,
    ) -> Self {
        Self {
            client: Arc::new(LazyMonitoringClient::new()),
            project_id,
            monitored_resource_type,
            monitored_resource_labels,
            metric_names,
            container_meta_client: Arc::new(container_meta_client),
            container_labels: Arc::new(OnceCell::new()),
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

        log::trace!(
            "Exporting {} metrics to GCP project {}",
            metrics.len(),
            self.project_id
        );

        let time_series = self.get_metric_data(metrics).await;

        // Send metrics in batches (Google Cloud allows up to 200 time series per request)
        for batch in time_series.chunks(200) {
            if let Err(e) = self.send_time_series_batch(client, batch.to_vec()).await {
                log::error!("Failed to export metrics batch: {}", e);
            }
        }

        Ok(())
    }

    async fn container_labels(&self) -> Arc<Vec<(String, String)>> {
        self.container_labels
            .get_or_try_init(|| async {
                let container_metadata = self.container_meta_client.collect().await?;
                anyhow::Ok(Arc::new(container_metadata.labels()))
            })
            .await
            .map(Arc::clone)
            .unwrap_or_else(|e| {
                log::warn!("failed fetching container metadata: {e}, using fallback");
                Arc::new(self.container_meta_client.fallback().labels())
            })
    }

    async fn get_metric_data(&self, collected: Vec<CollectedMetric>) -> Vec<TimeSeries> {
        let end_time = SystemTime::now();
        let ts_end_time: google_cloud_wkt::Timestamp = end_time.try_into().unwrap_or_default();

        let mut data: Vec<TimeSeries> = Vec::with_capacity(collected.len());

        let container_labels = self.container_labels().await;
        let container_labels_len = container_labels.len();

        let instance_id = &self
            .container_meta_client
            .collect()
            .await
            .unwrap_or(self.container_meta_client.fallback())
            .instance_id;

        for metric in collected {
            let cloud_metric_name = match self.metric_names.get(metric.key.name()) {
                Some(name) => name,
                None => {
                    log::warn!(
                        "Skipping metric '{}' - no cloud metric name configured",
                        metric.key.name()
                    );
                    continue;
                }
            };

            let mut labels =
                HashMap::with_capacity(container_labels_len + metric.key.labels().len());
            labels.extend(container_labels.iter().cloned());
            labels.extend(
                metric
                    .key
                    .labels()
                    .map(|label| (label.key().to_string(), label.value().to_string())),
            );

            let (kind, value_type, typed_value, interval) = match metric.value {
                MetricValue::CounterU64(val) => {
                    let start_time: google_cloud_wkt::Timestamp =
                        metric.registered_at.try_into().unwrap_or_default();

                    (
                        MetricKind::Cumulative,
                        ValueType::Int64,
                        TypedValue::new().set_int64_value(val as i64),
                        TimeInterval::new()
                            .set_start_time(start_time)
                            .set_end_time(ts_end_time),
                    )
                }
                MetricValue::CounterI64(val) => {
                    let start_time: google_cloud_wkt::Timestamp =
                        metric.registered_at.try_into().unwrap_or_default();

                    (
                        MetricKind::Cumulative,
                        ValueType::Int64,
                        TypedValue::new().set_int64_value(val),
                        TimeInterval::new()
                            .set_start_time(start_time)
                            .set_end_time(ts_end_time),
                    )
                }
                MetricValue::GaugeF64(val) => (
                    MetricKind::Gauge,
                    ValueType::Double,
                    TypedValue::new().set_double_value(val),
                    TimeInterval::new().set_end_time(ts_end_time),
                ),
                MetricValue::GaugeU64(val) => (
                    MetricKind::Gauge,
                    ValueType::Int64,
                    TypedValue::new().set_int64_value(val as i64),
                    TimeInterval::new().set_end_time(ts_end_time),
                ),
                MetricValue::GaugeI64(val) => (
                    MetricKind::Gauge,
                    ValueType::Int64,
                    TypedValue::new().set_int64_value(val),
                    TimeInterval::new().set_end_time(ts_end_time),
                ),
            };

            // Add container instance ID to node_id if present
            let mut monitored_resource_labels = self.monitored_resource_labels.clone();
            if let Some(node_id) = monitored_resource_labels.get("node_id") {
                monitored_resource_labels.insert(
                    "node_id".to_string(),
                    format!("{}-{}", node_id, instance_id),
                );
            }

            let mut mr = MonitoredResource::new().set_type(&self.monitored_resource_type);
            mr.labels = monitored_resource_labels;

            data.push(
                TimeSeries::new()
                    .set_metric_kind(kind)
                    .set_metric(
                        Metric::new()
                            .set_type(format!("custom.googleapis.com/{}", cloud_metric_name))
                            .set_labels(labels),
                    )
                    .set_resource(mr)
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
