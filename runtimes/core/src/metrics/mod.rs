mod counter;
mod exporter;
mod manager;
mod registry;

#[cfg(test)]
mod test;

pub use counter::Counter;
pub use manager::Manager;
pub use registry::{CollectedMetric, MetricInfo, MetricValue, Registry};

pub fn requests_total_counter(service: &str, endpoint: &str) -> Counter {
    crate::metrics::Counter::new("e_requests_total").with_labels([
        ("service", service.to_string()),
        ("endpoint", endpoint.to_string()),
    ])
}
