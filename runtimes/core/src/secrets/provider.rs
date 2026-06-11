use async_trait::async_trait;

/// Provider resolves a single secret from an external backend (GCP Secret
/// Manager, Vault, etc.). Implementations must be cheap to call concurrently.
#[async_trait]
pub trait Provider: Send + Sync + std::fmt::Debug {
    /// Load fetches the bytes for the referenced secret. The `version` may be
    /// empty, in which case the provider's default is used.
    async fn load(&self, id: &str, version: &str) -> Result<Vec<u8>, ProviderError>;
}

#[derive(Debug, thiserror::Error)]
pub enum ProviderError {
    #[error("provider client init failed: {0}")]
    Init(#[source] anyhow::Error),

    #[error("provider fetch failed: {0}")]
    Fetch(#[source] anyhow::Error),
}
