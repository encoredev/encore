use std::time::Duration;

use anyhow::Context;

const IMDS_ENDPOINT: &str =
    "http://169.254.169.254/metadata/instance?api-version=2021-02-01";
const REQUEST_TIMEOUT: Duration = Duration::from_secs(5);

#[derive(Debug, serde::Deserialize)]
pub struct AzureInstanceMeta {
    pub compute: AzureComputeMeta,
}

#[derive(Debug, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AzureComputeMeta {
    pub location: String,
    pub subscription_id: String,
    pub resource_group_name: String,
    pub name: String,
    pub vm_id: String,
}

#[derive(Debug)]
pub struct AzureMetadataClient {
    http_client: reqwest::Client,
}

impl AzureMetadataClient {
    pub fn new(http_client: reqwest::Client) -> Self {
        Self { http_client }
    }

    pub async fn fetch_instance_meta(&self) -> anyhow::Result<AzureInstanceMeta> {
        let req = self
            .http_client
            .get(IMDS_ENDPOINT)
            .header("Metadata", "true")
            .timeout(REQUEST_TIMEOUT)
            .build()
            .context("create Azure IMDS request")?;

        let resp = self
            .http_client
            .execute(req)
            .await
            .context("send Azure IMDS request")?;

        let meta = resp
            .json::<AzureInstanceMeta>()
            .await
            .context("deserialize Azure IMDS response")?;

        Ok(meta)
    }
}
