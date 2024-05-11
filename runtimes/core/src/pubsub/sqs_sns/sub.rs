use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result};
use aws_sdk_sqs::types::MessageSystemAttributeName;
use serde::Deserialize;
use tokio_retry::strategy::ExponentialBackoff;
use tokio_retry::{Action, Retry};

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::sqs_sns::{fetcher, LazyClient};
use crate::pubsub::{self};

#[derive(Debug)]
pub struct Subscription {
    client: Arc<LazyClient>,
    cloud_name: CloudName,
    ack_deadline: Duration,
    fetcher_cfg: fetcher::Config,
    requeue_policy: ExponentialBackoff,
}

impl Subscription {
    pub(super) fn new(
        client: Arc<LazyClient>,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Self {
        let mut requeue_policy = ExponentialBackoff::from_millis(
            meta.retry_policy
                .as_ref()
                .map(|retry| retry.min_backoff / 1_000_000)
                .unwrap_or(1000) as u64,
        )
        .factor(2);

        if let Some(retry) = &meta.retry_policy {
            requeue_policy =
                requeue_policy.max_delay(Duration::from_nanos(retry.max_backoff as u64));
        }

        let fetcher_cfg = fetcher::Config {
            max_concurrency: meta.max_concurrency.unwrap_or(100) as usize,
            max_batch_size: 10, // AWS SQS max batch size
        };

        // Clamp the ack deadline to between [1s, 12h].
        let ack_deadline = Duration::from_nanos(
            meta.ack_deadline
                .clamp(1_000_000_000, 12 * 3600 * 1_000_000_000) as u64,
        );

        Self {
            client,
            cloud_name: cfg.subscription_cloud_name.clone().into(),
            ack_deadline,
            fetcher_cfg,
            requeue_policy,
        }
    }
}

impl pubsub::Subscription for Subscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = Result<()>> + Send + '_>> {
        let client = self.client.clone();
        let cloud_name = self.cloud_name.clone();
        let ack_deadline = self.ack_deadline;
        let requeue_policy = self.requeue_policy.clone();
        let fetcher_cfg = self.fetcher_cfg.clone();

        Box::pin(async move {
            let client = client.get_sqs().await.clone();

            let sqs_fetcher = Arc::new(SqsFetcher {
                handler,
                client,
                queue_url: cloud_name.into(),
                ack_deadline,
                requeue_policy,
            });
            fetcher::process_concurrently(fetcher_cfg.clone(), sqs_fetcher).await;

            Ok(())
        })
    }
}

struct SqsFetcher {
    client: aws_sdk_sqs::Client,
    queue_url: String,
    ack_deadline: Duration,
    requeue_policy: ExponentialBackoff,
    handler: Arc<SubHandler>,
}

impl fetcher::Fetcher for Arc<SqsFetcher> {
    type Item = aws_sdk_sqs::types::Message;
    type Error = anyhow::Error;

    fn fetch(
        self,
        max_items: usize,
    ) -> Pin<
        Box<
            dyn Future<Output = std::result::Result<Vec<Self::Item>, Self::Error>> + Send + 'static,
        >,
    > {
        let client = self.client.clone();
        let queue_url = self.queue_url.clone();
        let ack_deadline = self.ack_deadline;

        Box::pin(async move {
            let result = client
                .receive_message()
                .queue_url(queue_url)
                .attribute_names("ApproximateReceiveCount".into())
                .visibility_timeout(ack_deadline.as_secs() as i32)
                .wait_time_seconds(20) // maximum allowed time
                .max_number_of_messages(max_items as i32)
                .send()
                .await;
            result
                .map(|res| res.messages.unwrap_or_default())
                .context("unable to receive messages")
        })
    }

    fn process(self, item: Self::Item) -> Pin<Box<dyn Future<Output = ()> + Send + 'static>> {
        Box::pin(async move {
            let receipt_handle = item.receipt_handle.clone().expect("missing receipt handle");
            let attempt = parse_attempt(&item);

            let result = match parse_message(item, attempt) {
                Ok(msg) => self
                    .handler
                    .handle_message(msg)
                    .await
                    .map_err(|err| err.into()),
                Err(err) => {
                    log::error!(
                        "encore: internal error: failed to parse message from SQS: {:#?}",
                        err
                    );
                    Err(err)
                }
            };

            match result {
                Ok(()) => {
                    let delete_action = DeleteMessageAction {
                        fetcher: self.clone(),
                        receipt_handle,
                    };

                    // Retry deleting a few times.
                    // If we can't delete the message, it'll be redelivered. Not much we can do.
                    let retry = ExponentialBackoff::from_millis(100).factor(2).take(5);
                    if let Err(err) = Retry::spawn(retry, delete_action).await {
                        log::error!(
                            "encore: internal error: failed to delete aws pub/sub message: {}",
                            err
                        );
                    }
                }
                Err(_) => {
                    // Determine the requeue delay.
                    let requeue_delay = self
                        .requeue_policy
                        .clone()
                        .skip((attempt - 1).max(0) as usize)
                        .next()
                        .unwrap_or(Duration::from_secs(1));

                    let requeue_action = RequeueMessageAction {
                        fetcher: self.clone(),
                        receipt_handle,
                        visibility_timeout: requeue_delay,
                    };

                    // Retry requeuing a few times.
                    let retry = ExponentialBackoff::from_millis(100).factor(2).take(5);
                    if let Err(err) = Retry::spawn(retry, requeue_action).await {
                        log::error!(
                            "encore: internal error: failed to requeue aws pub/sub message: {}",
                            err
                        );
                    }
                }
            }
        })
    }
}

struct RequeueMessageAction {
    fetcher: Arc<SqsFetcher>,
    receipt_handle: String,
    visibility_timeout: Duration,
}

impl Action for RequeueMessageAction {
    type Future = Pin<Box<dyn Future<Output = Result<()>> + Send>>;
    type Item = ();
    type Error = anyhow::Error;

    fn run(&mut self) -> Self::Future {
        let fetcher = self.fetcher.clone();
        let receipt_handle = self.receipt_handle.clone();
        let visibility_timeout = self.visibility_timeout;

        Box::pin(async move {
            fetcher
                .client
                .change_message_visibility()
                .queue_url(fetcher.queue_url.clone())
                .receipt_handle(receipt_handle)
                .visibility_timeout(visibility_timeout.as_secs() as i32)
                .send()
                .await
                .map(|_| ())
                .context("unable to requeue message")
        })
    }
}

struct DeleteMessageAction {
    fetcher: Arc<SqsFetcher>,
    receipt_handle: String,
}

impl Action for DeleteMessageAction {
    type Future = Pin<Box<dyn Future<Output = Result<()>> + Send>>;
    type Item = ();
    type Error = anyhow::Error;

    fn run(&mut self) -> Self::Future {
        let fetcher = self.fetcher.clone();
        let receipt_handle = self.receipt_handle.clone();

        Box::pin(async move {
            fetcher
                .client
                .delete_message()
                .queue_url(fetcher.queue_url.clone())
                .receipt_handle(receipt_handle)
                .send()
                .await
                .map(|_| ())
                .context("unable to delete message")
        })
    }
}

fn parse_attempt(message: &aws_sdk_sqs::types::Message) -> u32 {
    message
        .attributes
        .as_ref()
        .and_then(|attrs| attrs.get(&MessageSystemAttributeName::ApproximateReceiveCount))
        .and_then(|s| s.parse::<u32>().ok())
        .unwrap_or(1)
}

fn parse_message(message: aws_sdk_sqs::types::Message, attempt: u32) -> Result<pubsub::Message> {
    // We currently have to clone the message data because we can't move it out of the
    // ReceivedMessage as we need to call ack/nack afterwards.
    let sns_message: SNSMessageWrapper =
        serde_json::from_str(message.body.as_deref().unwrap_or_default())
            .context("failed to decode SNS message body")?;

    let publish_time = chrono::DateTime::parse_from_rfc3339(&sns_message.timestamp)
        .context("failed to parse publish timestamp")?
        .with_timezone(&chrono::Utc);

    let attrs = sns_message
        .message_attributes
        .into_iter()
        .filter_map(|(k, v)| {
            if v.r#type == "String" {
                Some((k, v.value))
            } else {
                None
            }
        })
        .collect::<HashMap<_, _>>();

    let body = serde_json::from_str(&sns_message.message).ok();
    let raw_body = sns_message.message.as_bytes().to_vec();

    Ok(pubsub::Message {
        id: sns_message.message_id,
        publish_time: Some(publish_time),
        attempt,
        data: pubsub::MessageData {
            attrs,
            body,
            raw_body,
        },
    })
}

/// SNSMessageWrapper matches the JSON that is sent to SQS from an SNS subscription
#[derive(Debug, Deserialize)]
#[allow(dead_code)]
struct SNSMessageWrapper {
    #[serde(rename = "Type")]
    r#type: String,
    #[serde(rename = "MessageId")]
    message_id: String,
    #[serde(rename = "TopicArn")]
    topic_arn: String,
    #[serde(rename = "Message")]
    message: String,
    #[serde(rename = "Timestamp")]
    timestamp: String,
    #[serde(rename = "SignatureVersion")]
    signature_version: String,
    #[serde(rename = "SigningCertURL")]
    signing_cert_url: String,
    #[serde(rename = "UnsubscribeURL")]
    unsubscribe_url: String,
    #[serde(rename = "MessageAttributes")]
    message_attributes: HashMap<String, MessageAttribute>,
}

#[derive(Debug, Deserialize)]
struct MessageAttribute {
    #[serde(rename = "Type")]
    r#type: String,
    #[serde(rename = "Value")]
    value: String,
}
