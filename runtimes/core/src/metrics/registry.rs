use crate::metrics::counter::CounterOps;
use crate::metrics::gauge::GaugeOps;
use crate::metrics::{counter, gauge};

use super::system::SystemMetricsCollector;
use super::{Counter, Gauge};
use dashmap::DashMap;
use malachite::base::num::basic::traits::One;
use metrics::{Key, Label};
use std::collections::HashSet;
use std::sync::atomic::AtomicU64;
use std::sync::Arc;
use std::time::SystemTime;

/// Storage for a metric with its atomic value and getter closure
struct MetricStorage {
    atomic: Arc<AtomicU64>,
    getter: Box<dyn Fn() -> MetricValue + Send + Sync>,
    registered_at: SystemTime,
}

impl std::fmt::Debug for MetricStorage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("MetricStorage")
            .field("atomic", &self.atomic)
            .field("registered_at", &self.registered_at)
            .finish()
    }
}

#[derive(Debug, Clone)]
pub struct Registry {
    counters: Arc<DashMap<Key, MetricStorage>>,
    gauges: Arc<DashMap<Key, MetricStorage>>,
    system_metrics: Arc<SystemMetricsCollector>,
}

impl Registry {
    pub fn new() -> Self {
        Self {
            counters: Arc::new(DashMap::new()),
            gauges: Arc::new(DashMap::new()),
            system_metrics: Arc::new(SystemMetricsCollector::new()),
        }
    }

    /// Create a counter with the given name and labels
    pub fn get_or_create_counter<'a, T>(
        &self,
        name: &str,
        labels: impl IntoIterator<Item = (&'a str, &'a str)>,
    ) -> Counter<T>
    where
        Arc<AtomicU64>: CounterOps<T>,
        T: One + Send + Sync + 'static,
    {
        let labels_vec: Vec<Label> = labels
            .into_iter()
            .map(|(k, v)| Label::new(k.to_string(), v.to_string()))
            .collect();
        let key = Key::from_parts(name.to_string(), labels_vec);

        let entry = self.counters.entry(key).or_insert_with(|| {
            let atomic = Arc::new(AtomicU64::new(0));
            let counter = Counter::new(Arc::clone(&atomic));
            let getter = Box::new(move || counter.get());
            MetricStorage {
                atomic,
                getter,
                registered_at: SystemTime::now(),
            }
        });

        Counter::new(Arc::clone(&entry.atomic))
    }

    /// Create a gauge with the given name and labels
    pub fn get_or_create_gauge<'a, T>(
        &self,
        name: &str,
        labels: impl IntoIterator<Item = (&'a str, &'a str)>,
    ) -> Gauge<T>
    where
        Arc<AtomicU64>: GaugeOps<T>,
        T: Send + Sync + 'static,
    {
        let labels_vec: Vec<Label> = labels
            .into_iter()
            .map(|(k, v)| Label::new(k.to_string(), v.to_string()))
            .collect();
        let key = Key::from_parts(name.to_string(), labels_vec);

        let entry = self.gauges.entry(key).or_insert_with(|| {
            let atomic = Arc::new(AtomicU64::new(0));
            let gauge = Gauge::new(Arc::clone(&atomic));
            let getter = Box::new(move || gauge.get());
            MetricStorage {
                atomic,
                getter,
                registered_at: SystemTime::now(),
            }
        });

        Gauge::new(Arc::clone(&entry.atomic))
    }

    /// Create a counter schema builder for defining static and dynamic labels
    pub fn counter_schema<T>(&self, name: &str) -> CounterSchemaBuilder<T>
    where
        Arc<AtomicU64>: CounterOps<T>,
        T: One + Send + Sync + 'static,
    {
        CounterSchemaBuilder::new(name.to_string(), Arc::new(self.clone()))
    }

    /// Create a gauge schema builder for defining static and dynamic labels
    pub fn gauge_schema<T>(&self, name: &str) -> GaugeSchemaBuilder<T>
    where
        Arc<AtomicU64>: GaugeOps<T>,
        T: Send + Sync + 'static,
    {
        GaugeSchemaBuilder::new(name.to_string(), Arc::new(self.clone()))
    }

    pub fn collect(&self) -> Vec<CollectedMetric> {
        let mut collected_metrics = Vec::new();

        self.system_metrics.update(self);

        // Collect counters
        for entry in self.counters.iter() {
            let key = entry.key();
            let store = entry.value();

            let value = (store.getter)();

            collected_metrics.push(CollectedMetric {
                value,
                key: key.clone(),
                registered_at: store.registered_at,
            });
        }

        // Collect gauges
        for entry in self.gauges.iter() {
            let key = entry.key();
            let store = entry.value();

            let value = (store.getter)();

            collected_metrics.push(CollectedMetric {
                value,
                key: key.clone(),
                registered_at: store.registered_at,
            });
        }

        collected_metrics
    }
}

impl Default for Registry {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone, PartialEq)]
pub enum MetricValue {
    // Counter variants
    CounterU64(u64),
    CounterI64(i64),

    // Gauge variants
    GaugeU64(u64),
    GaugeI64(i64),
    GaugeF64(f64),
}

#[derive(Debug, Clone)]
pub struct CollectedMetric {
    pub key: Key,
    pub value: MetricValue,
    pub registered_at: SystemTime,
}

/// Builder for creating counter schemas with static labels and required dynamic keys
pub struct CounterSchemaBuilder<T> {
    name: String,
    static_labels: Vec<(String, String)>,
    required_dynamic_keys: HashSet<String>,
    registry: Arc<Registry>,
    _phantom: std::marker::PhantomData<T>,
}

impl<T> CounterSchemaBuilder<T>
where
    Arc<AtomicU64>: CounterOps<T>,
    T: One + Send + Sync + 'static,
{
    pub(crate) fn new(name: String, registry: Arc<Registry>) -> Self {
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
    pub fn build(self) -> counter::Schema<T> {
        counter::Schema::new(
            self.name,
            self.static_labels,
            self.required_dynamic_keys,
            self.registry,
        )
    }
}

/// Builder for creating gauge schemas with static labels and required dynamic keys
pub struct GaugeSchemaBuilder<T> {
    name: String,
    static_labels: Vec<(String, String)>,
    required_dynamic_keys: HashSet<String>,
    registry: Arc<Registry>,
    _phantom: std::marker::PhantomData<T>,
}

impl<T> GaugeSchemaBuilder<T>
where
    Arc<AtomicU64>: GaugeOps<T>,
    T: Send + Sync + 'static,
{
    pub(crate) fn new(name: String, registry: Arc<Registry>) -> Self {
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

    /// Specify required dynamic label keys that must be provided at set/add/sub time
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

    /// Build the gauge schema
    pub fn build(self) -> gauge::Schema<T> {
        gauge::Schema::new(
            self.name,
            self.static_labels,
            self.required_dynamic_keys,
            self.registry,
        )
    }
}
