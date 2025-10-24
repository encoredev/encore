use encore_runtime_core::metrics::{CollectedMetric as CoreCollectedMetric, MetricValue, MetricsCollector};
use metrics::{Key, Label};
use napi::{Env, NapiRaw};
use napi_derive::napi;
use std::collections::HashMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::SystemTime;

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
    pub registered_at: SystemTime,
}

/// MetricsRegistry manages the SharedArrayBuffer and slot allocation
/// for custom application metrics.
#[napi]
pub struct MetricsRegistry {
    buffer_ptr: Arc<BufferPtr>,
    metadata: Arc<Mutex<Vec<MetricMetadata>>>,
    next_slot: Arc<AtomicUsize>,
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
///    module-level global variable (`globalBuffer` in mod.ts), which prevents garbage
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
            buffer_ptr: Arc::new(BufferPtr { ptr, len }),
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
            registered_at: SystemTime::now(),
        });

        slot as u32
    }

    /// Get the number of allocated slots
    #[napi]
    pub fn slot_count(&self) -> u32 {
        self.next_slot.load(Ordering::SeqCst) as u32
    }
}

/// Collector that bridges JS metrics to the core runtime's metrics system
pub struct JsMetricsCollector {
    metadata: Arc<Mutex<Vec<MetricMetadata>>>,
    buffer_ptr: Arc<BufferPtr>,
}

impl JsMetricsCollector {
    pub fn new(registry: &MetricsRegistry) -> Self {
        Self {
            metadata: Arc::clone(&registry.metadata),
            buffer_ptr: Arc::clone(&registry.buffer_ptr),
        }
    }

    /// Read a u64 value from the SharedArrayBuffer at the given slot
    fn read_slot(&self, slot: usize) -> u64 {
        if slot >= self.buffer_ptr.len {
            return 0;
        }
        // SAFETY: The pointer is valid for the lifetime of the SharedArrayBuffer,
        // and JavaScript uses Atomics to write to it. We use atomic loads for thread safety.
        unsafe {
            let ptr = self.buffer_ptr.ptr.add(slot);
            std::sync::atomic::AtomicU64::from_ptr(ptr).load(std::sync::atomic::Ordering::SeqCst)
        }
    }
}

impl MetricsCollector for JsMetricsCollector {
    fn collect(&self) -> Vec<CoreCollectedMetric> {
        let metadata = self.metadata.lock().unwrap();
        let mut collected = Vec::with_capacity(metadata.len());

        for meta in metadata.iter() {
            // Read the actual value from the SharedArrayBuffer
            let raw_value = self.read_slot(meta.slot);

            let value = match meta.metric_type {
                MetricType::Counter => MetricValue::CounterU64(raw_value),
                MetricType::GaugeInt => {
                    // For signed gauges, reinterpret the bits as i64
                    MetricValue::GaugeI64(raw_value as i64)
                }
                MetricType::GaugeFloat => {
                    // For float gauges, reinterpret the bits as f64
                    MetricValue::GaugeF64(f64::from_bits(raw_value))
                }
            };

            // Convert labels HashMap to Vec<Label>
            let labels: Vec<Label> = meta
                .labels
                .iter()
                .map(|(k, v)| Label::new(k.clone(), v.clone()))
                .collect();

            let key = Key::from_parts(meta.name.clone(), labels);

            collected.push(CoreCollectedMetric {
                key,
                value,
                registered_at: meta.registered_at,
            });
        }

        collected
    }
}
