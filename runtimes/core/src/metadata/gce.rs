//! GCE metadata client implementation based on golangs "cloud.google.com/go/compute/metadata"

use std::time::Duration;
use tokio::sync::OnceCell;

#[derive(Debug, thiserror::Error)]
pub enum GceMetadataError {
    #[error("HTTP request failed: {0}")]
    HttpRequest(#[from] reqwest::Error),
    #[error("GCE metadata not defined (404)")]
    NotDefined,
    #[error("GCE metadata server temporarily unavailable (503)")]
    ServiceUnavailable,
    #[error("GCE metadata server returned error status: {status}")]
    HttpStatus { status: reqwest::StatusCode },
    #[error("Failed to read response body: {0}")]
    ResponseBody(reqwest::Error),
}

// Default metadata server IP as documented by Google
const METADATA_IP: &str = "169.254.169.254";
// Env to override metadata server host, if not set the METADATA_IP will be used
const METADATA_HOST_ENV: &str = "GCE_METADATA_HOST";
const METADATA_FLAVOR_HEADER: &str = "Metadata-Flavor";
const GOOGLE_HEADER_VALUE: &str = "Google";
const USER_AGENT: &str = "encore-runtime/0.1.0";
// Timeout and retry configuration
const REQUEST_TIMEOUT: Duration = Duration::from_secs(5);
const MAX_RETRIES: usize = 3;

// Global cache for instance ID to ensure it's shared across all calls
static INSTANCE_ID_CACHE: OnceCell<String> = OnceCell::const_new();

#[derive(Debug)]
pub struct GceMetadataClient {
    http_client: reqwest::Client,
}

impl GceMetadataClient {
    pub fn new(http_client: reqwest::Client) -> Self {
        Self { http_client }
    }

    /// Build metadata URL, checking GCE_METADATA_HOST environment variable first
    fn build_metadata_url(path: &str) -> String {
        let host = std::env::var(METADATA_HOST_ENV).unwrap_or_else(|_| METADATA_IP.to_string());
        let path = path.trim_start_matches('/'); // Remove leading slashes like Go does
        format!("http://{}/computeMetadata/v1/{}", host, path)
    }

    /// Get the instance ID from GCE metadata server, with global caching
    /// This ensures only one HTTP request is made even with concurrent calls
    pub async fn instance_id(&self) -> Result<String, GceMetadataError> {
        let instance_id = INSTANCE_ID_CACHE
            .get_or_try_init(|| async { self.fetch_metadata("instance/id").await })
            .await?;

        Ok(instance_id.clone())
    }

    /// Fetch metadata from the GCE metadata server
    pub async fn fetch_metadata(&self, path: &str) -> Result<String, GceMetadataError> {
        let url = Self::build_metadata_url(path);
        log::debug!("Fetching GCE metadata from: {}", url);

        for attempt in 1..=MAX_RETRIES {
            let result = self.try_fetch(&url).await;

            match &result {
                Ok(_) => {
                    log::debug!("Successfully fetched GCE metadata from {}", path);
                    return result;
                }
                Err(e) if attempt == MAX_RETRIES => {
                    log::error!(
                        "Failed to fetch GCE metadata after {} attempts: {}",
                        MAX_RETRIES,
                        e
                    );
                    return result;
                }
                Err(e) => {
                    log::warn!("Attempt {} failed: {}, retrying...", attempt, e);
                    tokio::time::sleep(Duration::from_millis(100 * attempt as u64)).await;
                }
            }
        }

        unreachable!("unexpected failure fetching metadata")
    }

    async fn try_fetch(&self, url: &str) -> Result<String, GceMetadataError> {
        let response = self
            .http_client
            .get(url)
            .header(METADATA_FLAVOR_HEADER, GOOGLE_HEADER_VALUE)
            .header(http::header::USER_AGENT, USER_AGENT)
            .timeout(REQUEST_TIMEOUT)
            .send()
            .await?;

        let status = response.status();
        if status.is_success() {
            let text = response
                .text()
                .await
                .map_err(GceMetadataError::ResponseBody)?;
            Ok(text.trim().to_string())
        } else if status == reqwest::StatusCode::NOT_FOUND {
            Err(GceMetadataError::NotDefined)
        } else if status == reqwest::StatusCode::SERVICE_UNAVAILABLE {
            Err(GceMetadataError::ServiceUnavailable)
        } else {
            Err(GceMetadataError::HttpStatus { status })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[tokio::test]
    async fn test_gce_metadata_client_creation() {
        let http_client = reqwest::Client::builder()
            .timeout(Duration::from_secs(1))
            .build()
            .unwrap();

        let client = GceMetadataClient::new(http_client);

        // Test that the client can be created without panic
        assert!(format!("{:?}", client).contains("GceMetadataClient"));
    }

    #[tokio::test]
    async fn test_singleton_cache_behavior() {
        // Verify the cache is initially empty
        assert!(INSTANCE_ID_CACHE.get().is_none());

        // Test that we can set a value (simulating successful fetch)
        let test_id = "test-instance-123".to_string();
        let result = INSTANCE_ID_CACHE.set(test_id.clone());
        assert!(result.is_ok());

        // Verify the cached value can be retrieved
        assert_eq!(INSTANCE_ID_CACHE.get(), Some(&test_id));
    }

    #[test]
    fn test_metadata_url_construction() {
        // Test default URL construction (no environment variable)
        std::env::remove_var(METADATA_HOST_ENV);
        let url = GceMetadataClient::build_metadata_url("instance/id");
        assert_eq!(url, "http://169.254.169.254/computeMetadata/v1/instance/id");

        // Test with leading slash removal
        let url = GceMetadataClient::build_metadata_url("/instance/id");
        assert_eq!(url, "http://169.254.169.254/computeMetadata/v1/instance/id");
    }

    #[test]
    fn test_metadata_host_env_var() {
        // Test custom host via environment variable
        std::env::set_var(METADATA_HOST_ENV, "custom.metadata.host");
        let url = GceMetadataClient::build_metadata_url("instance/id");
        assert_eq!(
            url,
            "http://custom.metadata.host/computeMetadata/v1/instance/id"
        );

        // Clean up
        std::env::remove_var(METADATA_HOST_ENV);
    }
}
