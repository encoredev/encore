mod atomic;
mod exporter;
mod manager;
mod registry;
mod system;

pub mod counter;
pub mod gauge;

#[cfg(test)]
mod test;

use std::sync::Arc;

pub use counter::{Counter, CounterOps};
pub use gauge::{Gauge, GaugeOps};
pub use manager::Manager;
pub use registry::{CollectedMetric, MetricValue, MetricsCollector, Registry};
pub use system::SystemMetricsCollector;

/// Create a requests counter schema
pub fn requests_total_counter(
    registry: &Arc<Registry>,
    service: &str,
    endpoint: &str,
) -> counter::Schema<u64> {
    registry
        .counter_schema::<u64>("e_requests_total")
        .static_labels([("service", service), ("endpoint", endpoint)])
        .require_dynamic_key("code")
        .build()
}

/// Create a memory usage gauge schema
pub fn memory_usage_gauge_schema(registry: &Arc<Registry>) -> gauge::Schema<u64> {
    registry
        .gauge_schema::<u64>("e_sys_memory_used_bytes")
        .build()
}
