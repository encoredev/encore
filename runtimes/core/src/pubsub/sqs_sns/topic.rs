use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};

use crate::encore::runtime::v1 as pb;
use crate::encore::runtime::v1::pub_sub_topic::DeliveryGuarantee;
use crate::names::CloudName;
use crate::pubsub::sqs_sns::LazyClient;
use crate::pubsub::{self, MessageData, MessageId};

#[derive(Debug)]
pub struct Topic {
    client: Arc<LazyClient>,
    cloud_name: CloudName,
    delivery_guarantee: DeliveryGuarantee,
    publisher_id: xid::Id,
}

impl Topic {
    pub(super) fn new(
        client: Arc<LazyClient>,
        cfg: &pb::PubSubTopic,
        publisher_id: xid::Id,
    ) -> Self {
        Self {
            client,
            cloud_name: cfg.cloud_name.clone().into(),
            delivery_guarantee: cfg.delivery_guarantee(),
            publisher_id,
        }
    }
}

impl pubsub::Topic for Topic {
    fn publish(
        &self,
        msg: MessageData,
        ordering_key: Option<String>,
    ) -> Pin<Box<dyn Future<Output = Result<MessageId>> + Send + '_>> {
        Box::pin(async move {
            // The raw body is JSON, so it's valid UTF8.
            let data =
                String::from_utf8(msg.raw_body).context("failed to serialize message body")?;

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
            let mut params = client
                .publish()
                .topic_arn(self.cloud_name.to_string())
                .message(data);

            if let Some(ordering_key) = ordering_key {
                params = params.message_group_id(ordering_key);
                params = params.message_deduplication_id(format!("msg_{}", xid::new()));
            } else if self.delivery_guarantee == DeliveryGuarantee::ExactlyOnce {
                params = params.message_group_id(format!("inst_{}", self.publisher_id));
                params = params.message_deduplication_id(format!("msg_{}", xid::new()));
            }

            let result = params.set_message_attributes(Some(attrs)).send().await;

            match result {
                Ok(id) => Ok(id.message_id.unwrap_or("".to_string()) as MessageId),
                Err(e) => Err(e.into()),
            }
        })
    }
}
