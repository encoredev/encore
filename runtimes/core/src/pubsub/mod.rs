use std::collections::HashMap;
use std::fmt::Debug;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

pub use manager::{Manager, SubscriptionObj, TopicObj};
pub use push_registry::PushHandlerRegistry;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::pubsub::manager::SubHandler;
use crate::{api, model};

mod gcp;
mod manager;
mod noop;
mod nsq;
mod push_registry;

pub type MessageId = String;

pub struct MessageData {
    pub attrs: HashMap<String, String>,
    pub body: Option<serde_json::Value>,
    pub raw_body: Vec<u8>,
}

pub struct Message {
    pub id: MessageId,
    pub publish_time: Option<chrono::DateTime<chrono::Utc>>,
    pub attempt: u32, // starts at 1
    pub data: MessageData,
}

trait Cluster: Debug + Send + Sync {
    fn topic(&self, cfg: &pb::PubSubTopic) -> Arc<dyn Topic + 'static>;
    fn subscription(
        &self,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Arc<dyn Subscription + 'static>;
}

trait Topic: Debug + Send + Sync {
    fn publish(
        &self,
        msg: MessageData,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<MessageId>> + Send + '_>>;
}

trait Subscription: Debug + Send + Sync {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send + '_>>;

    fn push_handler(&self) -> Option<(String, Arc<dyn PushRequestHandler>)> {
        None
    }
}

trait PushRequestHandler: Debug + Sync + Send + 'static {
    fn handle_push(
        &self,
        req: axum::extract::Request,
    ) -> Pin<Box<dyn Future<Output = axum::response::Response<axum::body::Body>> + Send + 'static>>;
}

pub trait SubscriptionHandler: Debug + Send + Sync {
    fn handle_message(
        &self,
        msg: Arc<model::Request>,
    ) -> Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send + '_>>;
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct SubName {
    pub topic: EncoreName,
    pub subscription: EncoreName,
}
