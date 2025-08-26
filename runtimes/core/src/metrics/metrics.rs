use std::sync::atomic;

pub struct Counter {}
pub struct Gauge {}

struct MetricInfo {
    name: String,
    svc_num: u16,
}
struct Timeseries<T> {
    info: MetricInfo,
    id: u64,
    value: Vec<T>,
    valid: Vec<atomic::AtomicBool>,
}
