use dashmap::DashMap;
use metrics::{Key, KeyName, Metadata, Recorder, Unit};
use metrics_util::registry::{AtomicStorage, Registry as MetricsRegistry};
use std::sync::atomic::Ordering;
use std::sync::Arc;
use std::time::SystemTime;

#[derive(Clone)]
pub struct Registry {
    inner: Arc<MetricsRegistry<Key, AtomicStorage>>,
    first_seen: Arc<DashMap<u64, SystemTime>>,
}

impl Registry {
    pub fn new() -> Self {
        Self {
            inner: Arc::new(MetricsRegistry::atomic()),
            first_seen: Arc::new(DashMap::new()),
        }
    }

    pub fn first_seen(&self) -> Arc<DashMap<u64, SystemTime>> {
        Arc::clone(&self.first_seen)
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

            let count_value = counter.load(Ordering::Relaxed);
            collected_metrics.push(CollectedMetric {
                name,
                labels,
                value: MetricValue::Counter(count_value),
                timestamp,
                key: key.clone(),
            });
        });

        // Collect gauges
        self.inner.visit_gauges(|key, gauge| {
            let name = key.name().to_string();
            let labels: Vec<(String, String)> = key
                .labels()
                .map(|label| (label.key().to_string(), label.value().to_string()))
                .collect();

            let f64_value = f64::from_bits(gauge.load(Ordering::Relaxed));
            collected_metrics.push(CollectedMetric {
                name,
                labels,
                value: MetricValue::Gauge(f64_value),
                timestamp,
                key: key.clone(),
            });
        });

        // TODO(fredr): Collect histograms

        collected_metrics
    }

    /// Record the first registration time for a timeseries ID (only if not already registered)
    fn record_first_seen(&self, key: &Key) {
        self.first_seen
            .entry(key.get_hash())
            .or_insert_with(SystemTime::now);
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
        self.record_first_seen(key);
        let counter_handle = self.inner.get_or_create_counter(key, |c| c.clone());
        metrics::Counter::from_arc(counter_handle)
    }

    fn register_gauge(&self, key: &Key, _metadata: &Metadata<'_>) -> metrics::Gauge {
        self.record_first_seen(key);
        let gauge_handle = self.inner.get_or_create_gauge(key, |g| g.clone());
        metrics::Gauge::from_arc(gauge_handle)
    }

    fn register_histogram(&self, key: &Key, _metadata: &Metadata<'_>) -> metrics::Histogram {
        self.record_first_seen(key);
        let histogram_handle = self.inner.get_or_create_histogram(key, |h| h.clone());
        metrics::Histogram::from_arc(histogram_handle)
    }
}

impl Default for Registry {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone)]
pub struct MetricInfo {
    pub name: String,
}

impl MetricInfo {
    pub fn new(name: String) -> Self {
        Self { name }
    }
}

#[derive(Debug, Clone)]
pub enum MetricValue {
    Counter(u64),
    Gauge(f64),
}

#[derive(Debug, Clone)]
pub struct CollectedMetric {
    pub name: String,
    pub labels: Vec<(String, String)>,
    pub value: MetricValue,
    pub timestamp: std::time::SystemTime,
    pub key: Key,
}
