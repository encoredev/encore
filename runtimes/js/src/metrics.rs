use convert_case::{Case, Casing};
use encore_runtime_core::metrics::{CollectedMetric, MetricValue, MetricsCollector};
use metrics::{Key, Label};
use napi::{Env, NapiRaw};
use napi_derive::napi;
use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::time::SystemTime;

#[derive(Debug)]
#[napi(string_enum)]
pub enum MetricType {
    Counter,
    Gauge,
}

#[derive(Debug, Clone)]
pub struct MetricMetadata {
    pub slot: usize,
    pub key: Key,
    pub metric_type: MetricType,
    pub registered_at: SystemTime,
}

/// Internal state of the metrics registry, wrapped in Arc for sharing
pub(crate) struct MetricsRegistryInner {
    buffer_ptr: BufferPtr,
    next_slot: AtomicUsize,
    slot_map: Mutex<HashMap<Key, MetricMetadata>>,
}

/// Global singleton for the metrics registry, shared across all worker threads
/// TODO: use get_or_try_init when stable
static METRICS_REGISTRY: OnceLock<napi::Result<Arc<MetricsRegistryInner>>> = OnceLock::new();

/// MetricsRegistry manages the SharedArrayBuffer and slot allocation
/// for custom application metrics.
/// This is a lightweight wrapper around Arc<MetricsRegistryInner> for NAPI.
#[napi]
pub struct MetricsRegistry {
    pub(crate) inner: Arc<MetricsRegistryInner>,
}

/// Thread-safe wrapper around a SharedArrayBuffer-backed TypedArray pointer.
///
/// # Safety
///
/// This struct wraps a raw pointer obtained from a BigUint64Array view of a JavaScript
/// SharedArrayBuffer and must uphold critical safety invariants:
///
/// ## Lifetime Invariants
/// 1. **JavaScript Ownership**: The underlying SharedArrayBuffer is owned by JavaScript
///    and its lifetime is managed by V8's garbage collector.
/// 2. **JavaScript Lifetime Management**: The SharedArrayBuffer is stored in a JavaScript
///    module-level global variable (`globalBuffer`), which prevents garbage
///    collection for the entire application lifetime.
/// 3. **Pointer Stability**: SharedArrayBuffer backing stores do not move in memory,
///    so the pointer remains valid as long as the buffer exists. The TypedArray view's
///    data pointer is stable and points directly to the backing store.
///
/// ## Thread Safety
/// - The pointer is `Send + Sync` because:
///   - JavaScript uses `Atomics` operations for all writes to the SharedArrayBuffer
///   - Rust uses `AtomicU64::load()` with SeqCst ordering for all reads
///   - The backing SharedArrayBuffer is explicitly designed for concurrent access
///   - Multiple TypedArray views of the same SharedArrayBuffer share the same backing store
///
/// ## Usage Contract
/// - This pointer must NEVER outlive the `MetricsRegistry` that created it
/// - All access must be through atomic operations
/// - Bounds checking must be performed before dereferencing
/// - The pointer was obtained via `napi_get_typedarray_info`, which works correctly
///   with SharedArrayBuffer-backed TypedArrays (unlike `napi_get_arraybuffer_info`)
struct BufferPtr {
    ptr: *mut u64,
    len: usize,
}

unsafe impl Send for BufferPtr {}
unsafe impl Sync for BufferPtr {}

#[napi]
impl MetricsRegistry {
    #[napi(constructor)]
    pub fn new(env: Env, typed_array: napi::JsObject) -> napi::Result<Self> {
        // Extract the raw pointer from the TypedArray (BigUint64Array)
        // This works with SharedArrayBuffer-backed TypedArrays
        let (ptr, len) = unsafe {
            let mut arraybuffer: napi::sys::napi_value = std::ptr::null_mut();
            let mut byte_offset: usize = 0;
            let mut length: usize = 0;
            let mut data_ptr: *mut std::ffi::c_void = std::ptr::null_mut();

            let raw_env = env.raw();
            let raw_value = typed_array.raw();

            // Get TypedArray info (works with SharedArrayBuffer backing)
            let status = napi::sys::napi_get_typedarray_info(
                raw_env,
                raw_value,
                std::ptr::null_mut(), // Don't need type
                &mut length,
                &mut data_ptr,
                &mut arraybuffer,
                &mut byte_offset,
            );

            if status != napi::sys::Status::napi_ok {
                return Err(napi::Error::new(
                    napi::Status::from(status),
                    "Failed to get TypedArray info",
                ));
            }

            let ptr = data_ptr as *mut u64;
            // Length is already in elements (u64), not bytes

            (ptr, length)
        };

        Ok(Self {
            inner: Arc::new(MetricsRegistryInner {
                buffer_ptr: BufferPtr { ptr, len },
                next_slot: AtomicUsize::new(0),
                slot_map: Mutex::new(HashMap::new()),
            }),
        })
    }

    /// Allocate (or get) a slot for a unique metric
    #[napi]
    pub fn allocate_slot(
        &self,
        name: String,
        labels: Vec<(String, String)>,
        service_name: Option<String>,
        metric_type: MetricType,
    ) -> u32 {
        let mut label_vec: Vec<Label> = labels
            .into_iter()
            .map(|(k, v)| Label::new(k.to_case(Case::Snake), v))
            .collect();

        if let Some(ref svc) = service_name {
            label_vec.push(Label::new("service", svc.clone()));
        }

        let key = Key::from_parts(name, label_vec);
        let mut slot_map = self.inner.slot_map.lock().expect("mutex poisoned");

        if let Some(existing) = slot_map.get(&key) {
            return existing.slot as u32;
        }

        // Allocate new slot and insert metadata
        let slot = self.inner.next_slot.fetch_add(1, Ordering::SeqCst);
        slot_map.insert(
            key.clone(),
            MetricMetadata {
                slot,
                key,
                metric_type,
                registered_at: SystemTime::now(),
            },
        );

        slot as u32
    }

    /// Get the number of allocated slots
    #[napi]
    pub fn slot_count(&self) -> u32 {
        self.inner.next_slot.load(Ordering::SeqCst) as u32
    }

    /// Initialize the global metrics registry (should be called once by main thread)
    /// Returns the existing registry if already initialized
    pub(crate) fn get_or_init_global(
        env: Env,
        typed_array: napi::JsObject,
    ) -> napi::Result<Arc<MetricsRegistryInner>> {
        METRICS_REGISTRY
            .get_or_init(|| Self::new(env, typed_array).map(|registry| Arc::clone(&registry.inner)))
            .clone()
    }

    /// Get the global metrics registry if it has been initialized
    pub fn get_global() -> Option<Self> {
        METRICS_REGISTRY.get().and_then(|result| {
            result.as_ref().ok().map(|inner| Self {
                inner: Arc::clone(inner),
            })
        })
    }
}

/// Collector that bridges JS metrics to the core runtime's metrics system
pub struct JsMetricsCollector {
    registry: Arc<MetricsRegistryInner>,
}

impl JsMetricsCollector {
    pub fn new(registry: &MetricsRegistry) -> Self {
        Self {
            registry: Arc::clone(&registry.inner),
        }
    }

    /// Read a u64 value from the SharedArrayBuffer at the given slot
    fn read_slot(&self, slot: usize) -> u64 {
        if slot >= self.registry.buffer_ptr.len {
            return 0;
        }
        // SAFETY: The pointer is valid for the lifetime of the SharedArrayBuffer,
        // and JavaScript uses Atomics to write to it. We use atomic loads for thread safety.
        unsafe {
            let ptr = self.registry.buffer_ptr.ptr.add(slot);
            AtomicU64::from_ptr(ptr).load(Ordering::SeqCst)
        }
    }
}

impl MetricsCollector for JsMetricsCollector {
    fn collect(&self) -> Vec<CollectedMetric> {
        let slot_map = self.registry.slot_map.lock().expect("mutext poisoned");
        let mut collected = Vec::with_capacity(slot_map.len());

        for meta in slot_map.values() {
            let raw_value = self.read_slot(meta.slot);

            let value = match meta.metric_type {
                MetricType::Counter => MetricValue::CounterU64(raw_value),
                MetricType::Gauge => MetricValue::GaugeF64(f64::from_bits(raw_value)),
            };

            collected.push(CollectedMetric {
                key: meta.key.clone(),
                value,
                registered_at: meta.registered_at,
            });
        }

        collected
    }
}
