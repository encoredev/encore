use metrics::counter;

pub trait MetricLabels {
    const METRIC_NAME: &'static str;
    fn to_key_labels(self) -> Vec<(&'static str, String)>;
}

pub struct Counter<T: MetricLabels> {
    _phantom: std::marker::PhantomData<T>,
    cached_labels: Box<[(&'static str, String)]>,
}

impl<T: MetricLabels> Counter<T> {
    pub fn new(cfg: T) -> Self {
        let cached_labels = cfg.to_key_labels().into_boxed_slice();
        Self {
            _phantom: std::marker::PhantomData,
            cached_labels,
        }
    }

    pub fn increment(&self) {
        self.increment_by(1);
    }

    pub fn increment_by(&self, amount: u64) {
        counter!(T::METRIC_NAME, self.cached_labels.as_ref()).increment(amount);
    }
}

#[derive(Debug)]
pub struct RequestTotal {
    pub endpoint: String,
    pub code: String,
}

impl MetricLabels for RequestTotal {
    const METRIC_NAME: &'static str = "e_requests_total";

    fn to_key_labels(self) -> Vec<(&'static str, String)> {
        vec![("endpoint", self.endpoint), ("code", self.code)]
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
