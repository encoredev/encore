use metrics::counter;
use serde::{Deserialize, Serialize};

pub trait MetricLabels: Clone {
    /// Convert labels to key-value pairs for metrics
    fn to_key_labels(&self) -> Vec<(String, String)>;

    /// Get the metric name for this label type
    fn metric_name() -> &'static str;
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct RequestTotalLabels {
    pub endpoint: String,
    pub code: String,
}

impl MetricLabels for RequestTotalLabels {
    fn to_key_labels(&self) -> Vec<(String, String)> {
        vec![
            ("endpoint".to_string(), self.endpoint.clone()),
            ("code".to_string(), self.code.clone()),
        ]
    }

    fn metric_name() -> &'static str {
        "e_requests_total"
    }
}

#[derive(Debug, Clone)]
pub struct Counter<L: MetricLabels> {
    labels: L,
}

impl<L: MetricLabels> Counter<L> {
    pub fn new(labels: L) -> Self {
        Self { labels }
    }

    pub fn with(&self, labels: L) -> Self {
        Self { labels }
    }

    pub fn increment(&self) {
        self.increment_by(1u64);
    }

    pub fn increment_by<V: Into<u64>>(&self, amount: V) {
        let amount_u64 = amount.into();
        let label_pairs = self.labels.to_key_labels();
        counter!(L::metric_name(), &label_pairs).increment(amount_u64);
    }
}

pub struct Gauge {}

#[derive(Debug, Clone)]
pub struct MetricInfo {
    pub name: String,
    pub svc_num: u16,
}

#[derive(Debug, Clone)]
pub struct CollectedMetric {
    pub info: MetricInfo,
    pub labels: Vec<(String, String)>,
    pub value: u64,
    pub timestamp: std::time::SystemTime,
}
