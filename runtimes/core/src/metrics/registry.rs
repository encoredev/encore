use super::metrics::{CollectedMetric, MetricInfo};
use metrics::{Key, KeyName, Metadata, Recorder, Unit};
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::SystemTime;
use std::sync::atomic::{AtomicU64, Ordering};

#[derive(Debug, Default, Clone)]
pub struct Registry {
    counters: Arc<Mutex<HashMap<String, Arc<AtomicU64>>>>,
}

impl Registry {
    pub fn new() -> Self {
        Self {
            counters: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    pub fn collect(&self) -> Vec<CollectedMetric> {
        let counters = self.counters.lock().unwrap();
        counters
            .iter()
            .map(|(key_str, atomic_value)| {
                // Parse the key to extract name and labels
                let (name, labels) = Self::parse_metric_key(key_str);
                let value = atomic_value.load(Ordering::Relaxed);
                CollectedMetric {
                    info: MetricInfo {
                        name,
                        svc_num: 0,
                    },
                    labels,
                    value,
                    timestamp: SystemTime::now(),
                }
            })
            .collect()
    }

    fn parse_metric_key(key_str: &str) -> (String, Vec<(String, String)>) {
        // For now, assume the key format is "metric_name{label1=value1,label2=value2}"
        // This is a simplified parser - in production you'd want a more robust one
        if let Some(brace_pos) = key_str.find('{') {
            let name = key_str[..brace_pos].to_string();
            let labels_str = &key_str[brace_pos + 1..key_str.len() - 1]; // Remove { and }

            let labels = if labels_str.is_empty() {
                Vec::new()
            } else {
                labels_str
                    .split(',')
                    .filter_map(|pair| {
                        let mut parts = pair.split('=');
                        if let (Some(key), Some(value)) = (parts.next(), parts.next()) {
                            Some((key.trim().to_string(), value.trim().to_string()))
                        } else {
                            None
                        }
                    })
                    .collect()
            };
            (name, labels)
        } else {
            (key_str.to_string(), Vec::new())
        }
    }

    fn key_to_string(key: &Key) -> String {
        let name = key.name().to_string();
        let labels: Vec<String> = key
            .labels()
            .map(|label| format!("{}={}", label.key(), label.value()))
            .collect();

        if labels.is_empty() {
            name
        } else {
            format!("{}{{{}}}" , name, labels.join(","))
        }
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
        let key_string = Self::key_to_string(key);

        // Get or create an atomic counter for this key
        let atomic_counter = {
            let mut counters = self.counters.lock().unwrap();
            counters
                .entry(key_string)
                .or_insert_with(|| Arc::new(AtomicU64::new(0)))
                .clone()
        };

        // Return a metrics counter that wraps our atomic counter
        metrics::Counter::from_arc(atomic_counter)
    }

    fn register_gauge(&self, _key: &Key, _metadata: &Metadata<'_>) -> metrics::Gauge {
        metrics::Gauge::noop()
    }

    fn register_histogram(&self, _key: &Key, _metadata: &Metadata<'_>) -> metrics::Histogram {
        metrics::Histogram::noop()
    }
}
