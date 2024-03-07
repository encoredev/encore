use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, RwLock};

use chrono::Utc;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::log::LogFromRust;
use crate::model::{PubSubRequestData, RequestData, ResponseData, SpanId, SpanKey, TraceId};
use crate::names::EncoreName;
use crate::pubsub::noop::NoopCluster;
use crate::pubsub::{
    gcp, noop, nsq, Cluster, Message, MessageData, MessageId, SubName, Subscription,
    SubscriptionHandler, Topic,
};
use crate::trace::{protocol, Tracer};
use crate::{api, model};

use super::push_registry::PushHandlerRegistry;

pub struct Manager {
    tracer: Tracer,
    topic_cfg: HashMap<EncoreName, (Arc<dyn Cluster>, pb::PubSubTopic)>,
    sub_cfg: HashMap<
        SubName,
        (
            Arc<dyn Cluster>,
            pb::PubSubSubscription,
            meta::pub_sub_topic::Subscription,
        ),
    >,

    topics: Arc<RwLock<HashMap<EncoreName, Arc<dyn Topic>>>>,
    subs: Arc<RwLock<HashMap<SubName, Arc<SubscriptionObj>>>>,
    push_registry: PushHandlerRegistry,
}

#[derive(Debug)]
pub struct TopicObj {
    name: EncoreName,
    tracer: Tracer,
    inner: Arc<dyn Topic>,
}

impl TopicObj {
    pub async fn publish(
        &self,
        mut msg: MessageData,
        source: Option<&model::Request>,
    ) -> anyhow::Result<MessageId> {
        if let Some(source) = source {
            msg.attrs.insert(
                ATTR_PARENT_TRACE_ID.to_string(),
                source.span.0.serialize_encore(),
            );
            if let Some(ext_correlation_id) = &source.ext_correlation_id {
                msg.attrs.insert(
                    ATTR_EXT_CORRELATION_ID.to_string(),
                    ext_correlation_id.clone(),
                );
            }

            let payload = serde_json::to_vec_pretty(&msg.body).unwrap_or_default();
            let start_id = self
                .tracer
                .pubsub_publish_start(protocol::PublishStartData {
                    source,
                    topic: &self.name,
                    payload: &payload,
                });
            let result = self.inner.publish(msg).await;
            self.tracer.pubsub_publish_end(protocol::PublishEndData {
                start_id,
                source,
                result: &result,
            });
            result
        } else {
            self.inner.publish(msg).await
        }
    }
}

#[derive(Debug)]
pub struct SubscriptionObj {
    inner: Arc<dyn Subscription>,
    tracer: Tracer,
    service: EncoreName,
    topic: EncoreName,
    subscription: EncoreName,
}

impl SubscriptionObj {
    pub async fn subscribe(
        self: &Arc<Self>,
        handler: Arc<dyn SubscriptionHandler>,
    ) -> anyhow::Result<()> {
        let handler = Arc::new(SubHandler {
            obj: self.clone(),
            inner: handler,
        });
        self.inner.subscribe(handler).await
    }
}

#[derive(Debug)]
pub struct SubHandler {
    obj: Arc<SubscriptionObj>,
    inner: Arc<dyn SubscriptionHandler>,
}

const ATTR_PARENT_TRACE_ID: &'static str = "encore_parent_trace_id";
const ATTR_EXT_CORRELATION_ID: &'static str = "encore_ext_correlation_id";

impl SubHandler {
    pub(super) fn handle_message(
        &self,
        msg: Message,
    ) -> Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send + '_>> {
        Box::pin(async move {
            let span = SpanKey(TraceId::generate(), SpanId::generate());

            let parent_trace_id: Option<TraceId> = msg
                .data
                .attrs
                .get(ATTR_PARENT_TRACE_ID)
                .and_then(|s| TraceId::parse_encore(&s).ok());
            let ext_correlation_id = msg.data.attrs.get(ATTR_EXT_CORRELATION_ID);

            let start = tokio::time::Instant::now();
            let req = Arc::new(model::Request {
                span,
                parent_trace: parent_trace_id,
                parent_span: None,
                caller_event_id: None,
                ext_correlation_id: ext_correlation_id.cloned(),
                is_platform_request: false,
                internal_caller: None,
                start,
                data: RequestData::PubSub(PubSubRequestData {
                    service: self.obj.service.clone(),
                    topic: self.obj.topic.clone(),
                    subscription: self.obj.subscription.clone(),
                    message_id: msg.id.to_string(),
                    published: msg.publish_time.unwrap_or_else(|| Utc::now()),
                    attempt: msg.attempt,
                    payload: msg.data.raw_body.clone(),
                    parsed_payload: msg.data.body.clone(),
                }),
            });

            let logger = crate::log::root();
            logger.info(Some(&req), "starting request", None);

            self.obj.tracer.request_span_start(&req);
            let result = self.inner.handle_message(req.clone()).await;

            logger.info(Some(&req), "request completed", None);

            let resp = model::Response {
                request: req,
                duration: tokio::time::Instant::now().duration_since(start),
                data: ResponseData::PubSub(result.clone()),
            };
            self.obj.tracer.request_span_end(&resp);
            result
        })
    }
}

impl Manager {
    pub fn new(tracer: Tracer, clusters: Vec<pb::PubSubCluster>, md: &meta::Data) -> Self {
        let (topic_cfg, sub_cfg) = make_cfg_maps(clusters, md);

        Self {
            tracer,
            topic_cfg,
            sub_cfg,
            topics: Arc::default(),
            subs: Arc::default(),
            push_registry: PushHandlerRegistry::new(),
        }
    }

    pub fn topic(&self, name: EncoreName) -> Option<TopicObj> {
        let inner = self.topic_impl(name.clone())?;
        Some(TopicObj {
            name,
            inner,
            tracer: self.tracer.clone(),
        })
    }

    fn topic_impl(&self, name: EncoreName) -> Option<Arc<dyn Topic>> {
        if let Some(topic) = self.topics.read().unwrap().get(&name) {
            return Some(topic.clone());
        }

        let topic = {
            if let Some((cluster, topic_cfg)) = self.topic_cfg.get(&name) {
                cluster.topic(topic_cfg)
            } else {
                Arc::new(noop::NoopTopic)
            }
        };

        self.topics.write().unwrap().insert(name, topic.clone());
        Some(topic)
    }

    pub fn subscription(&self, name: SubName) -> Option<Arc<SubscriptionObj>> {
        if let Some(sub) = self.subs.read().unwrap().get(&name) {
            return Some(sub.clone());
        }

        let sub = {
            if let Some((cluster, sub_cfg, meta_sub)) = self.sub_cfg.get(&name) {
                let inner = cluster.subscription(sub_cfg);

                // If we have a push handler, register it.
                if let Some((sub_id, push_handler)) = inner.push_handler() {
                    self.push_registry.register(sub_id, push_handler);
                }

                Arc::new(SubscriptionObj {
                    inner,
                    tracer: self.tracer.clone(),
                    service: meta_sub.service_name.clone().into(),
                    topic: name.topic.clone(),
                    subscription: name.subscription.clone(),
                })
            } else {
                let inner = Arc::new(noop::NoopSubscription);
                Arc::new(SubscriptionObj {
                    inner,
                    tracer: self.tracer.clone(),
                    topic: name.topic.clone(),
                    subscription: name.subscription.clone(),

                    // We don't have a service if there is no sub config, but that's fine
                    // since it's only used by tracing, and a no-op subscription won't
                    // generate any traces.
                    service: "".into(),
                })
            }
        };

        self.subs.write().unwrap().insert(name, sub.clone());

        Some(sub)
    }

    pub fn push_registry(&self) -> PushHandlerRegistry {
        self.push_registry.clone()
    }
}

fn make_cfg_maps(
    clusters: Vec<pb::PubSubCluster>,
    md: &meta::Data,
) -> (
    HashMap<EncoreName, (Arc<dyn Cluster>, pb::PubSubTopic)>,
    HashMap<
        SubName,
        (
            Arc<dyn Cluster>,
            pb::PubSubSubscription,
            meta::pub_sub_topic::Subscription,
        ),
    >,
) {
    let mut topic_map = HashMap::new();
    let mut sub_map = HashMap::new();

    let meta_subs = {
        let mut map = HashMap::new();
        for topic in &md.pubsub_topics {
            for sub in &topic.subscriptions {
                let name = SubName {
                    topic: topic.name.clone().into(),
                    subscription: sub.name.clone().into(),
                };
                map.insert(name, sub);
            }
        }
        map
    };

    for cluster_cfg in clusters {
        let cluster = new_cluster(&cluster_cfg);

        for topic_cfg in cluster_cfg.topics {
            topic_map.insert(
                topic_cfg.encore_name.clone().into(),
                (cluster.clone(), topic_cfg),
            );
        }

        for sub_cfg in cluster_cfg.subscriptions {
            let topic_name = sub_cfg.topic_encore_name.clone().into();
            let sub_name = sub_cfg.subscription_encore_name.clone().into();
            let name = SubName {
                topic: topic_name,
                subscription: sub_name,
            };
            let Some(&meta_sub) = meta_subs.get(&name) else {
                continue;
            };

            sub_map.insert(name, (cluster.clone(), sub_cfg, meta_sub.to_owned()));
        }
    }

    (topic_map, sub_map)
}

fn new_cluster(cluster: &pb::PubSubCluster) -> Arc<dyn Cluster> {
    let Some(provider) = &cluster.provider else {
        log::error!("missing PubSub cluster provider: {}", cluster.rid);
        return Arc::new(NoopCluster);
    };

    match provider {
        pb::pub_sub_cluster::Provider::Gcp(_) => return Arc::new(gcp::Cluster::new()),
        pb::pub_sub_cluster::Provider::Nsq(cfg) => {
            return Arc::new(nsq::Cluster::new(cfg.hosts[0].clone()))
        }
        pb::pub_sub_cluster::Provider::Encore(_) => {
            log::error!("Encore Cloud Pub/Sub not yet supported: {}", cluster.rid);
        }
        pb::pub_sub_cluster::Provider::Aws(_) => {
            log::error!("AWS Pub/Sub not yet supported: {}", cluster.rid);
        }
        pb::pub_sub_cluster::Provider::Azure(_) => {
            log::error!("Azure Pub/Sub not yet supported: {}", cluster.rid);
        }
    }

    Arc::new(NoopCluster)
}
