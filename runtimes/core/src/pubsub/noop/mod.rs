use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::Result;

use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::manager::SubHandler;

#[derive(Debug)]
pub struct NoopCluster;

#[derive(Debug)]
pub struct NoopTopic;
#[derive(Debug)]
pub struct NoopSubscription;

impl pubsub::Cluster for NoopCluster {
    fn topic(&self, _cfg: &pb::PubSubTopic) -> Arc<dyn pubsub::Topic> {
        Arc::new(NoopTopic)
    }

    fn subscription(&self, _cfg: &pb::PubSubSubscription) -> Arc<dyn pubsub::Subscription> {
        Arc::new(NoopSubscription)
    }
}

impl pubsub::Topic for NoopTopic {
    fn publish(
        &self,
        _: pubsub::MessageData,
    ) -> Pin<Box<dyn Future<Output = Result<pubsub::MessageId>> + Send + '_>> {
        Box::pin(async {
            anyhow::bail!("topic not configured");
        })
    }
}

impl pubsub::Subscription for NoopSubscription {
    fn subscribe(
        &self,
        _: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = Result<()>> + Send + '_>> {
        Box::pin(futures::future::pending())
    }
}
