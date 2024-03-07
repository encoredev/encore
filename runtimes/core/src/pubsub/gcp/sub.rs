use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::{Context, Result};
use google_cloud_pubsub as gcp;
use tokio_util::sync::CancellationToken;

use crate::encore::runtime::v1 as pb;
use crate::pubsub::gcp::LazyGCPClient;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::{self, MessageId};

#[derive(Debug)]
pub struct Subscription {
    client: Arc<LazyGCPClient>,
    project_id: String,
    sub_name: String,
    cell: tokio::sync::OnceCell<Result<gcp::subscription::Subscription>>,
}

impl Subscription {
    pub(super) fn new(client: Arc<LazyGCPClient>, cfg: &pb::PubSubSubscription) -> Self {
        let Some(pb::pub_sub_subscription::ProviderConfig::GcpConfig(gcp_cfg)) =
            cfg.provider_config.as_ref()
        else {
            panic!("missing gcp config for subscription")
        };

        Self {
            client,
            project_id: gcp_cfg.project_id.clone().into(),
            sub_name: cfg.subscription_cloud_name.clone().into(),
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get_sub(&self) -> Result<&gcp::subscription::Subscription> {
        let res = self
            .cell
            .get_or_init(|| async {
                match self.client.get().await {
                    Ok(client) => {
                        let fqdn = format!(
                            "projects/{}/subscriptions/{}",
                            self.project_id, self.sub_name
                        );
                        Ok(client.subscription(&fqdn))
                    }
                    Err(e) => anyhow::bail!("failed to get gcp client: {}", e),
                }
            })
            .await;
        match res {
            Ok(sub) => Ok(sub),
            Err(e) => anyhow::bail!("failed to get topic: {}", e),
        }
    }
}

impl pubsub::Subscription for Subscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = Result<()>> + Send + '_>> {
        Box::pin(async move {
            // TODO configure these
            let cfg: Option<gcp::subscription::ReceiveConfig> = None;

            let sub = self.get_sub().await?;
            let cancel = CancellationToken::new();
            sub.receive(
                move |message, cancel| {
                    let handler = handler.clone();
                    handle_message(handler, message, cancel)
                },
                cancel,
                cfg,
            )
            .await
            .context("receive subscription")?;
            Ok(())
        })
    }
}

async fn handle_message(
    handler: Arc<SubHandler>,
    mut message: gcp::subscriber::ReceivedMessage,
    _cancel: CancellationToken,
) {
    // We currently have to clone the message data because we can't move it out of the
    // ReceivedMessage as we need to call ack/nack afterwards.
    let Ok(body) = serde_json::from_slice(&message.message.data) else {
        _ = message.nack();
        log::error!("failed to decode pubsub message body");
        return;
    };

    let attempt = message.delivery_attempt().unwrap_or(1) as u32;
    let publish_time = message
        .message
        .publish_time
        .as_ref()
        .and_then(|ts| chrono::DateTime::from_timestamp(ts.seconds, ts.nanos as u32));

    let raw_body = message.message.data.drain(..).collect();

    let msg = pubsub::Message {
        id: message.message.message_id.clone() as MessageId,
        publish_time,
        attempt,
        data: pubsub::MessageData {
            attrs: message.message.attributes.clone().into_iter().collect(),
            body,
            raw_body,
        },
    };

    // Process the message asynchronously.
    match handler.handle_message(msg).await {
        Ok(()) => {
            // Acknowledge the message.
            if let Err(err) = message.ack().await {
                log::error!("failed to ack message: {:?}", err);
            }
        }
        Err(err) => {
            log::info!("message handler failed, nacking message: {:?}", err);
            if let Err(err) = message.nack().await {
                log::error!("failed to nack message: {:?}", err);
            }
        }
    }
}
