use super::metrics::{CollectedMetric, MetricInfo};
use metrics::{Key, KeyName, Metadata, Recorder, Unit};
use metrics_util::registry::{AtomicStorage, Registry as MetricsRegistry};
use std::sync::atomic::Ordering;
use std::sync::Arc;
use std::time::SystemTime;

#[derive(Clone)]
pub struct Registry {
    inner: Arc<MetricsRegistry<Key, AtomicStorage>>,
}

impl Registry {
    pub fn new() -> Self {
        Self {
            inner: Arc::new(MetricsRegistry::atomic()),
        }
    }

    pub fn collect(&self) -> Vec<CollectedMetric> {
        let mut collected_metrics = Vec::new();
        let timestamp = SystemTime::now();

        // Collect counters
        self.inner.visit_counters(|key, counter| {
            let name = key.name().to_string();
            let labels: Vec<(String, String)> = key
                .labels()
                .map(|label| (label.key().to_string(), label.value().to_string()))
                .collect();
            let value = counter.load(Ordering::Relaxed);

            collected_metrics.push(CollectedMetric {
                info: MetricInfo { name, svc_num: 0 },
                labels,
                value,
                timestamp,
            });
        });

        // Collect gauges
        self.inner.visit_gauges(|key, gauge| {
            let name = key.name().to_string();
            let labels: Vec<(String, String)> = key
                .labels()
                .map(|label| (label.key().to_string(), label.value().to_string()))
                .collect();
            // Gauge values are stored as f64 bits in u64, convert back for display
            let f64_value = f64::from_bits(gauge.load(Ordering::Relaxed));
            let value = f64_value.to_bits(); // Keep as bits for now to maintain interface

            collected_metrics.push(CollectedMetric {
                info: MetricInfo { name, svc_num: 0 },
                labels,
                value,
                timestamp,
            });
        });

        // Collect histograms
        self.inner.visit_histograms(|key, histogram| {
            let name = key.name().to_string();
            let labels: Vec<(String, String)> = key
                .labels()
                .map(|label| (label.key().to_string(), label.value().to_string()))
                .collect();

            // TODO: Implement proper histogram data extraction
            // For now, just report 0 as a placeholder
            let value = 0u64;

            collected_metrics.push(CollectedMetric {
                info: MetricInfo { name, svc_num: 0 },
                labels,
                value,
                timestamp,
            });
        });

        collected_metrics
    }
}

// Implement the metrics Recorder trait to capture metrics
impl Recorder for Registry {
    fn describe_counter(
        &self,
        _key: KeyName,
        _unit: Option<Unit>,
        _description: metrics::SharedString,
    ) {
    }

    fn describe_gauge(
        &self,
        _key: KeyName,
        _unit: Option<Unit>,
        _description: metrics::SharedString,
    ) {
    }

    fn describe_histogram(
        &self,
        _key: KeyName,
        _unit: Option<Unit>,
        _description: metrics::SharedString,
    ) {
    }

    fn register_counter(&self, key: &Key, _metadata: &Metadata<'_>) -> metrics::Counter {
        let counter_handle = self.inner.get_or_create_counter(key, |c| c.clone());
        metrics::Counter::from_arc(counter_handle)
    }

    fn register_gauge(&self, key: &Key, _metadata: &Metadata<'_>) -> metrics::Gauge {
        let gauge_handle = self.inner.get_or_create_gauge(key, |g| g.clone());
        metrics::Gauge::from_arc(gauge_handle)
    }

    fn register_histogram(&self, key: &Key, _metadata: &Metadata<'_>) -> metrics::Histogram {
        let histogram_handle = self.inner.get_or_create_histogram(key, |h| h.clone());
        metrics::Histogram::from_arc(histogram_handle)
    }
}

impl Default for Registry {
    fn default() -> Self {
        Self::new()
    }
}
