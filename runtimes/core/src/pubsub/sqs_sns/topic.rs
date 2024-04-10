use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};

use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub::sqs_sns::LazyClient;
use crate::pubsub::{self, MessageData, MessageId};

#[derive(Debug)]
pub struct Topic {
    client: Arc<LazyClient>,
    cloud_name: CloudName,
}

impl Topic {
    pub(super) fn new(client: Arc<LazyClient>, cfg: &pb::PubSubTopic) -> Self {
        Self {
            client,
            cloud_name: cfg.cloud_name.clone().into(),
        }
    }
}

impl pubsub::Topic for Topic {
    fn publish(
        &self,
        msg: MessageData,
    ) -> Pin<Box<dyn Future<Output = Result<MessageId>> + Send + '_>> {
        Box::pin(async move {
            let data =
                serde_json::to_string(&msg.body).context("failed to serialize message body")?;

            let attrs: Result<HashMap<String, aws_sdk_sns::types::MessageAttributeValue>, _> = msg
                .attrs
                .into_iter()
                .map(|(k, v)| {
                    aws_sdk_sns::types::MessageAttributeValue::builder()
                        .data_type("String".to_string())
                        .string_value(v)
                        .build()
                        .map(|val| (k, val))
                })
                .collect();
            let attrs = attrs.context("failed to build message attributes")?;

            let client = self.client.get_sns().await;
            let result = client
                .publish()
                .topic_arn(self.cloud_name.to_string())
                .set_message_attributes(Some(attrs))
                .message(data)
                .send()
                .await;

            match result {
                Ok(id) => Ok(id.message_id.unwrap_or("".to_string()) as MessageId),
                Err(e) => Err(e.into()),
            }
        })
    }
}
