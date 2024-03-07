use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};
use google_cloud_googleapis::pubsub::v1::PubsubMessage;
use google_cloud_pubsub as gcp;

use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub::gcp::LazyGCPClient;
use crate::pubsub::{self, MessageData, MessageId};

#[derive(Debug)]
pub struct Topic {
    client: Arc<LazyGCPClient>,
    project_id: String,
    cloud_name: CloudName,
    cell: tokio::sync::OnceCell<Result<(gcp::topic::Topic, gcp::publisher::Publisher)>>,
}

impl Topic {
    pub(super) fn new(client: Arc<LazyGCPClient>, cfg: &pb::PubSubTopic) -> Self {
        let Some(pb::pub_sub_topic::ProviderConfig::GcpConfig(gcp_cfg)) =
            cfg.provider_config.as_ref()
        else {
            panic!("missing gcp config for topic")
        };

        Self {
            client,
            project_id: gcp_cfg.project_id.clone(),
            cloud_name: cfg.cloud_name.clone().into(),
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get_topic(&self) -> Result<(&gcp::topic::Topic, &gcp::publisher::Publisher)> {
        let res = self
            .cell
            .get_or_init(|| async {
                match self.client.get().await {
                    Ok(client) => {
                        let fqtn =
                            format!("projects/{}/topics/{}", self.project_id, self.cloud_name);
                        let topic = client.topic(&fqtn);
                        let publisher = topic.new_publisher(None);
                        Ok((topic, publisher))
                    }
                    Err(e) => anyhow::bail!("failed to get gcp client: {}", e),
                }
            })
            .await;
        match res {
            Ok((topic, publisher)) => Ok((topic, publisher)),
            Err(e) => anyhow::bail!("failed to get topic: {}", e),
        }
    }
}

impl pubsub::Topic for Topic {
    fn publish(
        &self,
        msg: MessageData,
    ) -> Pin<Box<dyn Future<Output = Result<MessageId>> + Send + '_>> {
        Box::pin(async move {
            let data = serde_json::to_vec(&msg.body).context("failed to serialize message body")?;

            let (_, publisher) = self.get_topic().await?;
            let awaiter = publisher
                .publish(PubsubMessage {
                    data,
                    attributes: msg.attrs.into_iter().collect(),
                    ordering_key: "".to_string(), // TODO support
                    ..Default::default()
                })
                .await;
            match awaiter.get().await {
                Ok(id) => Ok(id as MessageId),
                Err(e) => Err(e.into()),
            }
        })
    }
}
