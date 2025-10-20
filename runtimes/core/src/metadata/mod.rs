use std::collections::HashMap;

use crate::{
    encore::runtime::v1::{environment::Cloud, Environment},
    metadata::aws::AwsMetadataClient,
};
use anyhow::Context;
use tokio::sync::OnceCell;

mod aws;
mod gce;

#[derive(Debug)]
pub struct ContainerMetaClient {
    cell: OnceCell<ContainerMetadata>,
    env: Environment,
    http_client: reqwest::Client,
    fallback: ContainerMetadata,
}

impl ContainerMetaClient {
    pub fn new(env: Environment, http_client: reqwest::Client) -> Self {
        Self {
            cell: OnceCell::new(),
            fallback: ContainerMetadata {
                env_name: env.env_name.clone(),
                ..Default::default()
            },
            env,
            http_client,
        }
    }

    pub async fn collect(&self) -> anyhow::Result<&ContainerMetadata> {
        self.cell
            .get_or_try_init(|| async {
                ContainerMetadata::collect(&self.env, &self.http_client).await
            })
            .await
    }

    pub fn fallback(&self) -> &ContainerMetadata {
        &self.fallback
    }
}

#[derive(Debug, Clone, Default)]
pub struct ContainerMetadata {
    pub service_id: String,
    pub revision_id: String,
    pub instance_id: String,
    pub env_name: String,
}

impl ContainerMetadata {
    pub fn labels(&self) -> Vec<(String, String)> {
        vec![
            ("service_id".to_string(), self.service_id.clone()),
            ("revision_id".to_string(), self.revision_id.clone()),
            ("instance_id".to_string(), self.instance_id.clone()),
            ("env_name".to_string(), self.env_name.clone()),
        ]
    }

    pub async fn collect(env: &Environment, http_client: &reqwest::Client) -> anyhow::Result<Self> {
        match env.cloud() {
            Cloud::Gcp | Cloud::Encore => Self::collect_gcp(env, http_client).await,
            Cloud::Aws => Self::collect_aws(env, http_client).await,
            Cloud::Azure | Cloud::Unspecified | Cloud::Local => anyhow::bail!(
                "can't collect container meta in {}",
                env.cloud().as_str_name()
            ),
        }
    }

    async fn collect_aws(env: &Environment, http_client: &reqwest::Client) -> anyhow::Result<Self> {
        // Encore supports running on both ECS Fargate and EKS.
        // For Fargate, we can get the metadata from the ECS metadata service.
        // For EKS there doesn't appear to be a standard way to get the metadata, so skip it in that case.
        let metadata_uri = std::env::var("ECS_CONTAINER_METADATA_URI_V4")
            .map_err(|_| anyhow::anyhow!("unable to get ecs container metadata uri"))?;

        let client = AwsMetadataClient::new(http_client.clone(), metadata_uri);
        let task_meta = client.fetch_task_meta().await?;

        let instance_id = task_meta
            .task_arn
            .get(task_meta.task_arn.len().saturating_sub(8)..)
            .unwrap_or(&task_meta.task_arn)
            .to_string();

        Ok(Self {
            service_id: task_meta.service_name,
            revision_id: task_meta.revision,
            instance_id,
            env_name: env.env_name.clone(),
        })
    }

    async fn collect_gcp(env: &Environment, http_client: &reqwest::Client) -> anyhow::Result<Self> {
        let service = std::env::var("K_SERVICE").map_err(|_| {
            anyhow::anyhow!("unable to get service ID: env variable 'K_SERVICE' unset")
        })?;

        let revision = std::env::var("K_REVISION").map_err(|_| {
            anyhow::anyhow!("unable to get revision ID: env variable 'K_REVISION' unset")
        })?;

        let revision = revision
            .strip_prefix(&format!("{}-", service))
            .unwrap_or(&revision)
            .to_string();

        let instance_id = match std::env::var("K_POD") {
            Ok(pod_id) => {
                // If we have a K8s POD name, take the last part of it which is the random pod ID
                // On GKE, the InstanceID appears to be the Node, so if the multiple replicas are running
                // on the same InstanceID then we'd have a collision. This is unlikely, but possible -
                // hence why we use the pod ID instead.
                pod_id
                    .rsplit('-')
                    .next()
                    .ok_or_else(|| anyhow::anyhow!("invalid instance ID '{}'", pod_id))?
                    .to_string()
            }
            Err(_) => {
                // If we don't have a K8s POD name, we're running on Cloud Run and can get the instance ID from the metadata server
                let metadata_client = gce::GceMetadataClient::new(http_client.clone());

                metadata_client
                    .instance_id()
                    .await
                    .context("failed to get instance ID from GCE metadata server")?
            }
        };

        Ok(Self {
            service_id: service,
            revision_id: revision,
            instance_id,
            env_name: env.env_name.clone(),
        })
    }
}

/// Process environment variable substitution in labels
/// Replaces $ENV:VARIABLE_NAME with the actual environment variable value
pub fn process_env_substitution(labels: &mut HashMap<String, String>) {
    for (_, value) in labels.iter_mut() {
        if value.starts_with("$ENV:") {
            let env_var = &value[5..];
            if let Ok(env_value) = std::env::var(env_var) {
                *value = env_value;
            }
        }
    }
}
