use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};
use tokio_nsq::{
    NSQChannel, NSQConsumerConfig, NSQConsumerConfigSources, NSQMessage, NSQRequeueDelay, NSQTopic,
};

use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::nsq::topic::EncodedMessage;
use crate::pubsub::Subscription;

#[derive(Debug)]
pub struct NsqSubscription {
    addr: String,
    topic_name: CloudName,
    sub_name: CloudName,
}

impl NsqSubscription {
    pub(super) fn new(addr: String, cfg: &pb::PubSubSubscription) -> Self {
        let topic_name = cfg.topic_cloud_name.clone().into();
        let sub_name = cfg.subscription_cloud_name.clone().into();
        NsqSubscription {
            addr,
            topic_name,
            sub_name,
        }
    }
}

impl Subscription for NsqSubscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = Result<()>> + Send + '_>> {
        let topic = NSQTopic::new(self.topic_name.to_string()).unwrap();
        let channel = NSQChannel::new(self.sub_name.to_string()).unwrap();

        let mut consumer = NSQConsumerConfig::new(topic, channel)
            .set_max_in_flight(15)
            .set_sources(NSQConsumerConfigSources::Daemons(vec![self.addr.clone()]))
            .build();

        Box::pin(async move {
            loop {
                let Some(msg) = consumer.consume_filtered().await else {
                    continue;
                };
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
            // TODO customize requeue strategy
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
            body: encoded.body,
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
