use crate::metrics;
use std::collections::{HashMap, HashSet};
use std::marker::PhantomData;
use std::sync::atomic::AtomicU64;
use std::sync::Arc;

pub trait GaugeOps<T> {
    fn set(&self, value: T);
    fn get(&self) -> crate::metrics::MetricValue;
}

/// A typed gauge that can be set, incremented, or decremented
/// T must be compatible with GaugeOps for type-safe operations
pub struct Gauge<T> {
    atomic: Arc<AtomicU64>,
    _phantom: PhantomData<T>,
}

impl<T> Gauge<T>
where
    Arc<AtomicU64>: GaugeOps<T>,
{
    /// Create a new gauge with the given atomic storage
    /// This is typically called by Registry, not directly by users
    pub(crate) fn new(atomic: Arc<AtomicU64>) -> Self {
        Self {
            atomic,
            _phantom: PhantomData,
        }
    }

    /// Set the gauge to the specified value
    pub fn set(&self, value: T) {
        GaugeOps::set(&self.atomic, value);
    }

    /// Get the current value of the gauge
    pub fn get(&self) -> metrics::MetricValue {
        GaugeOps::get(&self.atomic)
    }
}

/// A gauge schema that defines static labels and required dynamic label keys
/// Validates dynamic labels at set/add/sub time and creates separate time series
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
    Arc<AtomicU64>: GaugeOps<T>,
    T: Send + Sync + 'static,
{
    /// Create a new gauge schema
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

    /// Set the gauge value directly without dynamic labels
    pub fn set(&self, value: T) {
        if !self.required_dynamic_keys.is_empty() {
            log::warn!(
                "setting gauge '{}' without required dynamic labels, required keys: {:?}",
                self.name,
                self.required_dynamic_keys
            );
        }

        self.get_or_create_gauge(HashMap::new()).set(value);
    }

    // Set the dynamic label values and return a completed Gauge
    pub fn with<L, K, V>(&self, dynamic_labels: L) -> Gauge<T>
    where
        L: IntoIterator<Item = (K, V)>,
        K: AsRef<str>,
        V: AsRef<str>,
    {
        // Convert dynamic_labels to HashMap first
        let dynamic_labels_map: HashMap<String, String> = dynamic_labels
            .into_iter()
            .map(|(k, v)| (k.as_ref().to_string(), v.as_ref().to_string()))
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

        self.get_or_create_gauge(dynamic_labels_map)
    }

    /// Get or create a gauge for the given dynamic labels
    fn get_or_create_gauge(&self, dynamic_labels: HashMap<String, String>) -> Gauge<T> {
        // Create merged labels (static + dynamic)
        let mut merged_labels = self.static_labels.clone();
        for (key, value) in dynamic_labels {
            merged_labels.push((key, value));
        }

        self.registry.get_or_create_gauge(
            &self.name,
            merged_labels.iter().map(|(k, v)| (k.as_str(), v.as_str())),
        )
    }
}
