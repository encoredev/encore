use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use tokio::sync::{mpsc, oneshot};
use tokio_nsq::{NSQEvent, NSQProducerConfig, NSQTopic};

use crate::encore::runtime::v1 as pb;
use crate::pubsub::{MessageData, MessageId, Topic};

struct PublishRequest {
    msg: MessageData,
    resp: oneshot::Sender<Result<MessageId>>,
}

#[derive(Debug)]
pub struct NsqTopic {
    tx: mpsc::Sender<PublishRequest>,
}

impl NsqTopic {
    pub(super) fn new(addr: String, cfg: &pb::PubSubTopic) -> Self {
        let cloud_name = cfg.cloud_name.clone();
        let (tx, mut rx) = mpsc::channel::<PublishRequest>(32);
        tokio::spawn(async move {
            let topic = NSQTopic::new(&cloud_name).unwrap();
            let mut producer = NSQProducerConfig::new(addr).build();

            // Wait for the producer to send a Ready event.
            loop {
                match producer.consume().await {
                    Some(NSQEvent::Healthy()) => break,
                    _ => {}
                }
            }

            loop {
                // Wait for either a publish request or an acknowledgement from the producer.
                tokio::select! {
                    req = rx.recv() => {
                        let Some(req) = req else {
                            break;
                        };

                        // Serialize the message body.
                        let encoded = EncodedMessage::new_for_data(req.msg);
                        let bytes = serde_json::to_vec(&encoded).expect("unable to serialize request");

                        // TODO handle error
                        producer
                            .publish(&topic, bytes)
                            .await
                            .expect("failed to publish message");

                        // Ignore error.
                        _ = req.resp.send(Ok(encoded.id));
                    }
                    _ = producer.consume() => {}
                }
            }
        });

        NsqTopic { tx }
    }
}

impl Topic for NsqTopic {
    fn publish(
        &self,
        msg: MessageData,
    ) -> Pin<Box<dyn Future<Output = Result<MessageId>> + Send + '_>> {
        let tx = self.tx.clone();
        Box::pin(async move {
            let (resp_tx, resp_rx) = oneshot::channel::<Result<MessageId>>();
            let req = PublishRequest { msg, resp: resp_tx };
            tx.send(req).await.context("failed to send message")?;

            let resp = resp_rx.await.context("failed to receive response")?;
            Ok(resp?)
        })
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub(super) struct EncodedMessage {
    pub id: MessageId,
    pub body: Option<serde_json::Value>,
    pub attrs: HashMap<String, String>,
}

impl EncodedMessage {
    pub fn new_for_data(msg: MessageData) -> Self {
        Self {
            id: xid::new().to_string(),
            body: msg.body,
            attrs: msg.attrs,
        }
    }
}
