use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};
use azservicebus::prelude::{ServiceBusMessage, ServiceBusSender, ServiceBusSenderOptions};
use fe2o3_amqp_types::messaging::ApplicationProperties;
use fe2o3_amqp_types::primitives::{OrderedMap, SimpleValue};

use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub::azure::LazyAzureClient;
use crate::pubsub::{self, MessageData, MessageId};

#[derive(Debug)]
pub struct Topic {
    client: Arc<LazyAzureClient>,
    cloud_name: CloudName,
    sender: tokio::sync::OnceCell<anyhow::Result<Arc<tokio::sync::Mutex<ServiceBusSender>>>>,
}

impl Topic {
    pub(super) fn new(client: Arc<LazyAzureClient>, cfg: &pb::PubSubTopic) -> Self {
        Self {
            client,
            cloud_name: cfg.cloud_name.clone().into(),
            sender: tokio::sync::OnceCell::new(),
        }
    }

    async fn get_sender(
        &self,
    ) -> &anyhow::Result<Arc<tokio::sync::Mutex<ServiceBusSender>>> {
        self.sender
            .get_or_init(|| async {
                match self.client.get().await {
                    Ok(arc_client) => {
                        let mut client = arc_client.lock().await;
                        let sender = client
                            .create_sender(
                                self.cloud_name.to_string(),
                                ServiceBusSenderOptions::default(),
                            )
                            .await
                            .context("failed to create Azure Service Bus sender")?;
                        Ok(Arc::new(tokio::sync::Mutex::new(sender)))
                    }
                    Err(e) => anyhow::bail!("failed to get Azure client: {}", e),
                }
            })
            .await
    }
}

impl pubsub::Topic for Topic {
    fn publish(
        &self,
        msg: MessageData,
        ordering_key: Option<String>,
    ) -> Pin<Box<dyn Future<Output = Result<MessageId>> + Send + '_>> {
        Box::pin(async move {
            let arc_sender = match self.get_sender().await {
                Ok(s) => s.clone(),
                Err(e) => anyhow::bail!("failed to get Azure Service Bus sender: {}", e),
            };

            // Destructure early to avoid partial-move errors.
            let MessageData { raw_body, attrs } = msg;

            let message_id = xid::new().to_string();
            let mut message = ServiceBusMessage::new(raw_body);

            // Set a unique message ID for deduplication.
            message
                .set_message_id(message_id.clone())
                .map_err(|e| anyhow::anyhow!("failed to set message ID: {:?}", e))?;

            // Set the ordering key as the session ID for ordered delivery.
            if let Some(key) = ordering_key {
                message
                    .set_session_id(Some(key))
                    .map_err(|e| anyhow::anyhow!("failed to set session ID: {:?}", e))?;
            }

            // Copy message attributes into Azure AMQP application properties.
            if !attrs.is_empty() {
                let app_props = message
                    .application_properties_mut()
                    .get_or_insert_with(|| ApplicationProperties(OrderedMap::new()));
                for (k, v) in attrs {
                    app_props.0.insert(k, SimpleValue::String(v));
                }
            }

            let mut sender = arc_sender.lock().await;
            sender
                .send_message(message)
                .await
                .context("failed to publish message to Azure Service Bus")?;

            Ok(message_id)
        })
    }
}
