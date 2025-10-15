mod gcp;
pub use gcp::Gcp;

#[async_trait::async_trait]
pub trait Exporter: Send + Sync + std::fmt::Debug {
    async fn export(&self, metrics: Vec<crate::metrics::CollectedMetric>);
}
