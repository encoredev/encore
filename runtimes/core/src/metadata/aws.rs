use std::time::Duration;

use anyhow::Context;

#[derive(serde::Deserialize)]
pub struct AwsTaskMeta {
    #[serde(rename = "ServiceName")]
    pub service_name: String,
    #[serde(rename = "Revision")]
    pub revision: String,
    #[serde(rename = "TaskARN")]
    pub task_arn: String,
}

#[derive(Debug)]
pub struct AwsMetadataClient {
    http_client: reqwest::Client,
    metadata_uri: String,
}

impl AwsMetadataClient {
    pub fn new(http_client: reqwest::Client, metadata_uri: String) -> Self {
        AwsMetadataClient {
            http_client,
            metadata_uri,
        }
    }

    pub async fn fetch_task_meta(&self) -> anyhow::Result<AwsTaskMeta> {
        let req = self
            .http_client
            .get(format!("{}/task", self.metadata_uri))
            .timeout(Duration::from_secs(30))
            .build()
            .context("create metadata request")?;

        let resp = self
            .http_client
            .execute(req)
            .await
            .context("send metadata request")?;

        let task_meta = resp
            .json::<AwsTaskMeta>()
            .await
            .context("deserialize task metadata")?;

        Ok(task_meta)
    }
}
