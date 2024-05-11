use std::sync::Arc;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::sqs_sns::sub::Subscription;
use crate::pubsub::sqs_sns::topic::Topic;

mod fetcher;
mod sub;
mod topic;

#[derive(Debug)]
pub struct Cluster {
    /// publisher_id is a unique ID for this Encore app instance, used as the Message Group ID
    /// for topics which don't specify a grouping field. This is based on [AWS's recommendation]
    /// that each producer should have a unique message group ID to send all it's messages.
    ///
    /// [AWS's recommendation]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues-understanding-logic.html
    _publisher_id: xid::Id,

    client: Arc<LazyClient>,
}

impl Cluster {
    pub fn new() -> Self {
        let publisher_id = xid::new();
        let client = Arc::new(LazyClient::new());
        Self {
            _publisher_id: publisher_id,
            client,
        }
    }
}

impl pubsub::Cluster for Cluster {
    fn topic(&self, cfg: &pb::PubSubTopic) -> Arc<dyn pubsub::Topic + 'static> {
        Arc::new(Topic::new(self.client.clone(), cfg))
    }

    fn subscription(
        &self,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Arc<dyn pubsub::Subscription + 'static> {
        Arc::new(Subscription::new(self.client.clone(), cfg, meta))
    }
}

#[derive(Debug)]
struct LazyClient {
    sns_cell: tokio::sync::OnceCell<aws_sdk_sns::Client>,
    sqs_cell: tokio::sync::OnceCell<aws_sdk_sqs::Client>,
}

impl LazyClient {
    fn new() -> Self {
        Self {
            sns_cell: tokio::sync::OnceCell::new(),
            sqs_cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn config(&self) -> aws_config::SdkConfig {
        let provider = aws_config::meta::region::RegionProviderChain::default_provider();
        aws_config::defaults(aws_config::BehaviorVersion::latest())
            .region(provider)
            .load()
            .await
    }

    async fn get_sns(&self) -> &aws_sdk_sns::Client {
        self.sns_cell
            .get_or_init(|| async {
                let cfg = self.config().await;
                aws_sdk_sns::Client::new(&cfg)
            })
            .await
    }

    async fn get_sqs(&self) -> &aws_sdk_sqs::Client {
        self.sqs_cell
            .get_or_init(|| async {
                let cfg = self.config().await;
                aws_sdk_sqs::Client::new(&cfg)
            })
            .await
    }
}
