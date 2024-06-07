use std::sync::Arc;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::nsq::sub::NsqSubscription;
use crate::pubsub::nsq::topic::NsqTopic;

mod sub;
mod topic;

#[derive(Debug)]
pub struct Cluster {
    /// Address to the NSQ server.
    address: String,
}

impl Cluster {
    pub fn new(address: String) -> Self {
        Self { address }
    }
}

impl pubsub::Cluster for Cluster {
    fn topic(&self, cfg: &pb::PubSubTopic) -> Arc<dyn pubsub::Topic + 'static> {
        Arc::new(NsqTopic::new(self.address.clone(), cfg))
    }

    fn subscription(
        &self,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Arc<dyn pubsub::Subscription + 'static> {
        Arc::new(NsqSubscription::new(self.address.clone(), cfg, meta))
    }
}
