use anyhow::anyhow;
use async_trait::async_trait;
use google_cloud_secretmanager_v1::client::SecretManagerService;

use super::provider::{Provider, ProviderError};

#[derive(Debug)]
pub struct GcpProvider {
    client: SecretManagerService,
    project_id: String,
}

impl GcpProvider {
    pub async fn new(project_id: String) -> Result<Self, ProviderError> {
        let client = SecretManagerService::builder()
            .build()
            .await
            .map_err(|e| ProviderError::Init(anyhow!(e)))?;
        Ok(Self { client, project_id })
    }

    /// Build the fully-qualified secret-version resource name. The `id` may be
    /// either a short secret name or an already-qualified resource path; in the
    /// latter case the version is appended only if not already present.
    fn resource_name(&self, id: &str, version: &str) -> String {
        let version = if version.is_empty() {
            "latest"
        } else {
            version
        };
        if id.starts_with("projects/") {
            if id.contains("/versions/") {
                id.to_string()
            } else {
                format!("{id}/versions/{version}")
            }
        } else {
            format!(
                "projects/{}/secrets/{}/versions/{}",
                self.project_id, id, version
            )
        }
    }
}

#[async_trait]
impl Provider for GcpProvider {
    async fn load(&self, id: &str, version: &str) -> Result<Vec<u8>, ProviderError> {
        let name = self.resource_name(id, version);
        let resp = self
            .client
            .access_secret_version()
            .set_name(name.clone())
            .send()
            .await
            .map_err(|e| ProviderError::Fetch(anyhow!("access {name}: {e}")))?;
        let payload = resp
            .payload
            .ok_or_else(|| ProviderError::Fetch(anyhow!("{name}: missing payload")))?;
        Ok(payload.data.to_vec())
    }
}
