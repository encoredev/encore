use crate::metrics::counter::{CounterOps, CounterSchemaBuilder};
use crate::metrics::gauge::{GaugeOps, GaugeSchemaBuilder};

use super::system::SystemMetricsCollector;
use super::{Counter, Gauge};
use dashmap::DashMap;
use malachite::base::num::basic::traits::One;
use metrics::{Key, Label};
use std::sync::atomic::AtomicU64;
use std::sync::Arc;
use std::time::SystemTime;

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
