use napi::{Env, Ref};
use napi_derive::napi;
use std::collections::HashMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};

#[derive(Debug)]
#[napi(string_enum)]
pub enum MetricType {
    Counter,
    GaugeInt,
    GaugeFloat,
}

#[derive(Debug, Clone)]
pub struct MetricMetadata {
    pub slot: usize,
    pub name: String,
    pub labels: HashMap<String, String>,
    pub metric_type: MetricType,
}

/// MetricsRegistry manages the SharedArrayBuffer and slot allocation
/// for custom application metrics.
#[napi]
pub struct MetricsRegistry {
    buffer_ref: Ref<()>,
    metadata: Arc<Mutex<Vec<MetricMetadata>>>,
    next_slot: Arc<AtomicUsize>,
}

#[napi]
impl MetricsRegistry {
    #[napi(constructor)]
    pub fn new(env: Env, buffer: napi::JsObject) -> napi::Result<Self> {
        let buffer_ref = env.create_reference(buffer)?;

        Ok(Self {
            buffer_ref,
            metadata: Arc::new(Mutex::new(Vec::new())),
            next_slot: Arc::new(AtomicUsize::new(0)),
        })
    }

    /// Allocate a new slot for a unique metric name + label combination
    #[napi]
    pub fn allocate_slot(
        &self,
        name: String,
        labels: HashMap<String, String>,
        metric_type: MetricType,
    ) -> u32 {
        let slot = self.next_slot.fetch_add(1, Ordering::SeqCst);

        let mut metadata = self.metadata.lock().unwrap();
        metadata.push(MetricMetadata {
            slot,
            name,
            labels,
            metric_type,
        });

        slot as u32
    }

    /// Collect all metrics by reading values from the SharedArrayBuffer
    #[napi]
    pub fn collect(&self, _env: Env) -> napi::Result<Vec<CollectedMetric>> {
        // For now, return collected metadata without reading from buffer
        // This can be enhanced to actually read from SharedArrayBuffer once we
        // figure out the correct NAPI types
        let metadata = self.metadata.lock().unwrap();
        let mut collected = Vec::with_capacity(metadata.len());

        for meta in metadata.iter() {
            // For now, return zero values - will be updated when buffer reading is implemented
            let (counter, gauge_int, gauge_float) = match meta.metric_type {
                MetricType::Counter => (Some("0".to_string()), None, None),
                MetricType::GaugeInt => (None, Some("0".to_string()), None),
                MetricType::GaugeFloat => (None, None, Some(0.0)),
            };

            collected.push(CollectedMetric {
                name: meta.name.clone(),
                labels: meta.labels.clone(),
                counter,
                gauge_int,
                gauge_float,
            });
        }

        Ok(collected)
    }

    /// Get the number of allocated slots
    #[napi]
    pub fn slot_count(&self) -> u32 {
        self.next_slot.load(Ordering::SeqCst) as u32
    }
}

#[napi(object)]
pub struct CollectedMetric {
    pub name: String,
    pub labels: HashMap<String, String>,
    pub counter: Option<String>,
    pub gauge_int: Option<String>,
    pub gauge_float: Option<f64>,
}

/// A counter schema that can create counter instances with specific labels
#[napi]
pub struct CounterSchema {
    name: String,
    static_labels: HashMap<String, String>,
    metadata: Arc<Mutex<Vec<MetricMetadata>>>,
    next_slot: Arc<AtomicUsize>,
}

#[napi]
impl CounterSchema {
    #[napi(constructor)]
    pub fn new(name: String, registry: &MetricsRegistry) -> Self {
        Self {
            name,
            static_labels: HashMap::new(),
            metadata: Arc::clone(&registry.metadata),
            next_slot: Arc::clone(&registry.next_slot),
        }
    }

    /// Add a static label to this schema
    #[napi]
    pub fn static_label(&mut self, key: String, value: String) {
        self.static_labels.insert(key, value);
    }

    /// Allocate a slot for a specific set of dynamic labels
    #[napi]
    pub fn allocate_slot_for_labels(&self, dynamic_labels: HashMap<String, String>) -> u32 {
        // Merge static and dynamic labels
        let mut all_labels = self.static_labels.clone();
        all_labels.extend(dynamic_labels);

        let slot = self.next_slot.fetch_add(1, Ordering::SeqCst);

        let mut metadata = self.metadata.lock().unwrap();
        metadata.push(MetricMetadata {
            slot,
            name: self.name.clone(),
            labels: all_labels,
            metric_type: MetricType::Counter,
        });

        slot as u32
    }
}

/// A gauge schema that can create gauge instances with specific labels
#[napi]
pub struct GaugeSchema {
    name: String,
    static_labels: HashMap<String, String>,
    gauge_type: MetricType,
    metadata: Arc<Mutex<Vec<MetricMetadata>>>,
    next_slot: Arc<AtomicUsize>,
}

#[napi]
impl GaugeSchema {
    #[napi(constructor)]
    pub fn new(name: String, gauge_type: MetricType, registry: &MetricsRegistry) -> Self {
        Self {
            name,
            static_labels: HashMap::new(),
            gauge_type,
            metadata: Arc::clone(&registry.metadata),
            next_slot: Arc::clone(&registry.next_slot),
        }
    }

    /// Add a static label to this schema
    #[napi]
    pub fn static_label(&mut self, key: String, value: String) {
        self.static_labels.insert(key, value);
    }

    /// Allocate a slot for a specific set of dynamic labels
    #[napi]
    pub fn allocate_slot_for_labels(&self, dynamic_labels: HashMap<String, String>) -> u32 {
        // Merge static and dynamic labels
        let mut all_labels = self.static_labels.clone();
        all_labels.extend(dynamic_labels);

        let slot = self.next_slot.fetch_add(1, Ordering::SeqCst);

        let metric_type = match self.gauge_type {
            MetricType::GaugeInt => MetricType::GaugeInt,
            MetricType::GaugeFloat => MetricType::GaugeFloat,
            _ => MetricType::GaugeInt, // Default fallback
        };

        let mut metadata = self.metadata.lock().unwrap();
        metadata.push(MetricMetadata {
            slot,
            name: self.name.clone(),
            labels: all_labels,
            metric_type,
        });

        slot as u32
    }
}
