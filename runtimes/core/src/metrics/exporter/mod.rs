mod aws;
mod datadog;
mod gcp;
pub use aws::Aws;
pub use datadog::Datadog;
pub use gcp::Gcp;

#[async_trait::async_trait]
pub trait Exporter: Send + Sync {
    async fn export(&self, metrics: Vec<crate::metrics::CollectedMetric>);
}
