use std::collections::HashMap;

/// A counter with base labels that can be incremented with additional labels
#[derive(Debug, Clone)]
pub struct Counter {
    metric_name: &'static str,
    base_labels: HashMap<String, String>,
}

impl Counter {
    /// Create a new counter with the given metric name
    pub fn new(metric_name: &'static str) -> Self {
        Self {
            metric_name,
            base_labels: HashMap::new(),
        }
    }

    /// Add multiple labels to the counter's base labels
    pub fn with_labels<K, V>(
        mut self,
        new_labels: impl IntoIterator<Item = (K, V)>,
    ) -> Self
    where
        K: Into<String>,
        V: Into<String>,
    {
        for (key, value) in new_labels {
            self.base_labels.insert(key.into(), value.into());
        }
        self
    }

    /// Increment the counter
    pub fn increment(&self) {
        use metrics::{Key, Label, Metadata};

        metrics::with_recorder(|recorder| {
            let labels: Vec<Label> = self
                .base_labels
                .iter()
                .map(|(k, v)| Label::new(k.clone(), v.clone()))
                .collect();

            let key = Key::from_parts(self.metric_name, labels);
            let metadata =
                Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
            let counter = recorder.register_counter(&key, &metadata);
            counter.increment(1);
        });
    }

    /// Increment the counter with additional labels
    pub fn increment_with<K, V>(&self, extra_labels: impl IntoIterator<Item = (K, V)>)
    where
        K: Into<String>,
        V: Into<String>,
    {
        use metrics::{Key, Label, Metadata};

        metrics::with_recorder(|recorder| {
            let mut labels: Vec<Label> = self
                .base_labels
                .iter()
                .map(|(k, v)| Label::new(k.clone(), v.clone()))
                .collect();

            // Add extra labels
            for (key, value) in extra_labels {
                labels.push(Label::new(key.into(), value.into()));
            }

            let key = Key::from_parts(self.metric_name, labels);
            let metadata =
                Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
            let counter = recorder.register_counter(&key, &metadata);
            counter.increment(1);
        });
    }
}
