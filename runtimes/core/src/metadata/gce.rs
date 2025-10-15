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

        for attempt in 1..=MAX_RETRIES {
            let result = self.try_fetch(&url).await;

            match &result {
                Ok(_) => {
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
