use malachite::base::num::basic::traits::One;

use crate::metrics;
use std::collections::{HashMap, HashSet};
use std::marker::PhantomData;
use std::sync::atomic::AtomicU64;
use std::sync::Arc;

pub trait CounterOps<T> {
    fn increment(&self, value: T);
    fn get(&self) -> crate::metrics::MetricValue;
}

/// A typed counter that can be incremented
/// T must be compatible with CounterOps for type-safe operations
pub struct Counter<T> {
    atomic: Arc<AtomicU64>,
    _phantom: PhantomData<T>,
}

impl<T> Counter<T>
where
    Arc<AtomicU64>: CounterOps<T>,
    T: One,
{
    /// Create a new counter with the given atomic storage
    /// This is typically called by Registry, not directly by users
    pub(crate) fn new(atomic: Arc<AtomicU64>) -> Self {
        Self {
            atomic,
            _phantom: PhantomData,
        }
    }

    /// Increment the counter by 1
    pub fn increment(&self) {
        CounterOps::increment(&self.atomic, T::ONE);
    }

    /// Get the current value of the counter
    pub fn get(&self) -> metrics::MetricValue {
        CounterOps::get(&self.atomic)
    }
}

/// A counter schema that defines static labels and required dynamic label keys
/// Validates dynamic labels at increment time and creates separate time series
/// for each unique combination of static + dynamic labels
#[derive(Clone, Debug)]
pub struct Schema<T> {
    name: String,
    static_labels: Vec<(String, String)>,
    required_dynamic_keys: HashSet<String>,
    registry: Arc<metrics::Registry>,
    _phantom: PhantomData<T>,
}

impl<T> Schema<T>
where
    Arc<AtomicU64>: CounterOps<T>,
    T: One + Send + Sync + 'static,
{
    /// Create a new counter schema
    pub(crate) fn new(
        name: String,
        static_labels: Vec<(String, String)>,
        required_dynamic_keys: HashSet<String>,
        registry: Arc<metrics::Registry>,
    ) -> Self {
        Self {
            name,
            static_labels,
            required_dynamic_keys,
            registry,
            _phantom: PhantomData,
        }
    }

    pub fn with<L, K, V>(&self, dynamic_labels: L) -> Counter<T>
    where
        L: IntoIterator<Item = (K, V)>,
        K: Into<String>,
        V: Into<String>,
    {
        // Convert dynamic_labels to HashMap first
        let dynamic_labels_map: HashMap<String, String> = dynamic_labels
            .into_iter()
            .map(|(k, v)| (k.into(), v.into()))
            .collect();

        // Validate required keys are present
        let missing: Vec<String> = self
            .required_dynamic_keys
            .iter()
            .filter(|key| !dynamic_labels_map.contains_key(*key))
            .cloned()
            .collect();

        if !missing.is_empty() {
            log::warn!(
                "missing required dynamic labels for metric '{}': {:?}, required keys: {:?}",
                self.name,
                missing,
                self.required_dynamic_keys
            );
        }

        self.get_or_create_counter(dynamic_labels_map)
    }

    /// Increment the counter with the given dynamic labels
    pub fn increment(&self)
    where
        T: One,
    {
        if !self.required_dynamic_keys.is_empty() {
            log::warn!(
                "incrementing counter '{}' without required dynamic labels, required keys: {:?}",
                self.name,
                self.required_dynamic_keys
            );
        }

        self.get_or_create_counter(HashMap::new()).increment();
    }

    /// Get or create a counter for the given dynamic labels
    fn get_or_create_counter(&self, dynamic_labels: HashMap<String, String>) -> Counter<T> {
        // Create merged labels (static + dynamic)
        let mut merged_labels = self.static_labels.clone();
        for (key, value) in dynamic_labels {
            merged_labels.push((key, value));
        }

        self.registry.get_or_create_counter(
            &self.name,
            merged_labels.iter().map(|(k, v)| (k.as_str(), v.as_str())),
        )
    }
}

/// Builder for creating counter schemas with static labels and required dynamic keys
pub struct CounterSchemaBuilder<T> {
    name: String,
    static_labels: Vec<(String, String)>,
    required_dynamic_keys: HashSet<String>,
    registry: Arc<metrics::Registry>,
    _phantom: std::marker::PhantomData<T>,
}

impl<T> CounterSchemaBuilder<T>
where
    Arc<AtomicU64>: CounterOps<T>,
    T: One + Send + Sync + 'static,
{
    pub(crate) fn new(name: String, registry: Arc<metrics::Registry>) -> Self {
        Self {
            name,
            static_labels: Vec::new(),
            required_dynamic_keys: HashSet::new(),
            registry,
            _phantom: std::marker::PhantomData,
        }
    }

    /// Add static labels that are set once when the schema is created
    pub fn static_labels<I, K, V>(mut self, labels: I) -> Self
    where
        I: IntoIterator<Item = (K, V)>,
        K: AsRef<str>,
        V: AsRef<str>,
    {
        for (key, value) in labels {
            self.static_labels
                .push((key.as_ref().to_string(), value.as_ref().to_string()));
        }
        self
    }

    /// Add a single static label
    pub fn static_label(mut self, key: &str, value: &str) -> Self {
        self.static_labels
            .push((key.to_string(), value.to_string()));
        self
    }

    /// Specify required dynamic label keys that must be provided at increment time
    pub fn require_dynamic_keys<I, K>(mut self, keys: I) -> Self
    where
        I: IntoIterator<Item = K>,
        K: AsRef<str>,
    {
        for key in keys {
            self.required_dynamic_keys.insert(key.as_ref().to_string());
        }
        self
    }

    /// Add a single required dynamic key
    pub fn require_dynamic_key(mut self, key: &str) -> Self {
        self.required_dynamic_keys.insert(key.to_string());
        self
    }

    /// Build the counter schema
    pub fn build(self) -> Schema<T> {
        Schema::new(
            self.name,
            self.static_labels,
            self.required_dynamic_keys,
            self.registry,
        )
    }
}
