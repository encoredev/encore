use std::fmt::Debug;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result};
use tokio_nsq::{
    NSQChannel, NSQConsumerConfig, NSQConsumerConfigSources, NSQMessage, NSQRequeueDelay, NSQTopic,
};

use crate::api::APIResult;
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::nsq::topic::EncodedMessage;
use crate::pubsub::Subscription;

pub struct NsqSubscription {
    addr: String,
    config: NSQConsumerConfig,
    max_retries: i64,
}

impl Debug for NsqSubscription {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("NsqSubscription")
            .field("addr", &self.addr)
            .finish()
    }
}

impl NsqSubscription {
    pub(super) fn new(
        addr: String,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Self {
        let topic = NSQTopic::new(cfg.topic_cloud_name.clone()).unwrap();
        let channel = NSQChannel::new(cfg.subscription_cloud_name.clone()).unwrap();

        let mut config = NSQConsumerConfig::new(topic, channel)
            .set_sources(NSQConsumerConfigSources::Daemons(vec![addr.clone()]))
            .set_max_in_flight(meta.max_concurrency.map_or(100, |v| v as u32));

        // For local development, default to 2 retries if we don't have a retry policy.
        // We don't want to retry forever but zero retries might cause surprises when suddenly
        // things start retrying in other environments.
        let mut max_retries = 2;

        if let Some(retry) = &meta.retry_policy {
            let min_backoff = Duration::from_nanos(retry.min_backoff.max(0) as u64);
            config = config.set_base_requeue_interval(clamp(
                min_backoff,
                Duration::from_secs(0),
                Duration::from_secs(60 * 60),
            ));

            let max_backoff = Duration::from_nanos(retry.max_backoff.max(0) as u64);
            config = config.set_max_requeue_interval(clamp(
                max_backoff,
                Duration::from_secs(0),
                Duration::from_secs(60 * 60),
            ));
            max_retries = retry.max_retries;
        }

        NsqSubscription {
            addr,
            config,
            max_retries,
        }
    }
}

impl Subscription for NsqSubscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = APIResult<()>> + Send + 'static>> {
        let mut consumer = self.config.clone().build();
        let max_retries = self.max_retries;

        Box::pin(async move {
            loop {
                let Some(msg) = consumer.consume_filtered().await else {
                    continue;
                };

                // If the attempt exceeds the max retries, drop it.
                // Attempt starts at 1 for the first delivery, which means
                // the retry count is (attempt-1).
                let retry = msg.attempt as i64 - 1;
                if retry > max_retries {
                    msg.finish().await;
                    continue;
                }

                // Process the message asynchronously.
                let h = handler.clone();
                tokio::spawn(async move { process_message(msg, h).await });
            }
        })
    }
}

async fn process_message(mut msg: NSQMessage, handler: Arc<SubHandler>) {
    let body: Vec<u8> = msg.body.drain(..).collect();
    let result = handle_message(body, msg.timestamp, msg.attempt, handler).await;
    match result {
        Ok(()) => msg.finish().await,
        Err(err) => {
            log::info!("message handler failed, requeueing message: {:?}", err);
            msg.requeue(NSQRequeueDelay::DefaultDelay).await;
        }
    }
}

async fn handle_message(
    body: Vec<u8>,
    timestamp: u64,
    attempt: u16,
    handler: Arc<SubHandler>,
) -> Result<()> {
    let encoded =
        serde_json::from_slice::<EncodedMessage>(&body).context("failed to decode message")?;

    let publish_time = nano_timestamp(timestamp);
    let raw_body = serde_json::to_vec_pretty(&encoded.body).unwrap_or_default();
    let pubsub_msg = pubsub::Message {
        id: encoded.id,
        publish_time,
        attempt: attempt as u32,
        data: pubsub::MessageData {
            attrs: encoded.attrs,
            raw_body,
        },
    };

    handler
        .handle_message(pubsub_msg)
        .await
        .context("message handler failed")
}

fn nano_timestamp(mut nsec: u64) -> Option<chrono::DateTime<chrono::Utc>> {
    // From Go's time.Unix.
    let mut sec: i64 = 0;
    if nsec >= 1_000_000_000 {
        let n = nsec / 1_000_000_000;
        sec += n as i64;
        nsec -= n * 1_000_000_000;
    }
    chrono::DateTime::from_timestamp(sec, nsec as u32)
}

fn clamp<T: PartialOrd>(val: T, min: T, max: T) -> T {
    if val < min {
        min
    } else if val > max {
        max
    } else {
        val
    }
}
