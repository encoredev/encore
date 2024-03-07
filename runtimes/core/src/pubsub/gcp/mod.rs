use std::sync::Arc;

use anyhow::Context;
use google_cloud_pubsub as gcp;

use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::gcp::sub::Subscription;
use crate::pubsub::gcp::topic::Topic;

mod jwk;
mod push_sub;
mod sub;
mod topic;
#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyGCPClient>,
}

impl Cluster {
    pub fn new() -> Self {
        let client = Arc::new(LazyGCPClient::new());
        Self { client }
    }
}

impl pubsub::Cluster for Cluster {
    fn topic(&self, cfg: &pb::PubSubTopic) -> Arc<dyn pubsub::Topic + 'static> {
        Arc::new(Topic::new(self.client.clone(), cfg))
    }

    fn subscription(
        &self,
        cfg: &pb::PubSubSubscription,
    ) -> Arc<dyn pubsub::Subscription + 'static> {
        // If this is a push-based subscription, return that implementation.
        if let Some(pb::pub_sub_subscription::ProviderConfig::GcpConfig(gcp_cfg)) =
            cfg.provider_config.as_ref()
        {
            if gcp_cfg.push_service_account.is_some() {
                return Arc::new(push_sub::PushSubscription::new(cfg));
            }
        }

        Arc::new(Subscription::new(self.client.clone(), &cfg))
    }
}

#[derive(Debug)]
struct LazyGCPClient {
    cell: tokio::sync::OnceCell<anyhow::Result<gcp::client::Client>>,
}

impl LazyGCPClient {
    fn new() -> Self {
        Self {
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get(&self) -> &anyhow::Result<gcp::client::Client> {
        self.cell
            .get_or_init(|| async {
                let config = gcp::client::ClientConfig::default()
                    .with_auth()
                    .await
                    .context("get client config")?;
                gcp::client::Client::new(config).await.context("get client")
            })
            .await
    }
}
