use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, RwLock};

use anyhow::Context;
use chrono::Utc;

use crate::api::jsonschema::{self, JSONSchema};
use crate::api::PValues;
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::log::LogFromRust;
use crate::model::{PubSubRequestData, RequestData, ResponseData, SpanId, SpanKey, TraceId};
use crate::names::EncoreName;
use crate::pubsub::noop::NoopCluster;
use crate::pubsub::{
    gcp, noop, nsq, sqs_sns, Cluster, Message, MessageData, MessageId, SubName, Subscription,
    SubscriptionHandler, Topic,
};
use crate::trace::{protocol, Tracer};
use crate::{api, model};

use super::push_registry::PushHandlerRegistry;

pub struct Manager {
    tracer: Tracer,
    topic_cfg: HashMap<EncoreName, (Arc<dyn Cluster>, pb::PubSubTopic, JSONSchema)>,
    sub_cfg: HashMap<
        SubName,
        (
            Arc<dyn Cluster>,
            pb::PubSubSubscription,
            meta::pub_sub_topic::Subscription,
            JSONSchema,
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
    pub fn publish(
        &self,
        payload: PValues,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = anyhow::Result<MessageId>> + 'static {
        let tracer = self.tracer.clone();
        let inner = self.inner.clone();
        let name = self.name.clone();

        async move {
            let payload = serde_json::to_vec_pretty(&payload)
                .context("unable to serialize message payload")?;
            let mut msg = MessageData {
                attrs: HashMap::new(),
                raw_body: payload,
            };

            if let Some(source) = source.as_deref() {
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

                let start_id = tracer.pubsub_publish_start(protocol::PublishStartData {
                    source,
                    topic: &name,
                    payload: &msg.raw_body,
                });
                let result = inner.publish(msg).await;
                tracer.pubsub_publish_end(protocol::PublishEndData {
                    start_id,
                    source,
                    result: &result,
                });
                result
            } else {
                inner.publish(msg).await
            }
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
    schema: JSONSchema,
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

const ATTR_PARENT_TRACE_ID: &str = "encore_parent_trace_id";
const ATTR_EXT_CORRELATION_ID: &str = "encore_ext_correlation_id";

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
                .and_then(|s| TraceId::parse_encore(s).ok());
            let ext_correlation_id = msg.data.attrs.get(ATTR_EXT_CORRELATION_ID);

            let mut de = serde_json::Deserializer::from_slice(&msg.data.raw_body);
            let parsed_payload = self.obj.schema.deserialize(
                &mut de,
                jsonschema::DecodeConfig {
                    coerce_strings: false,
                },
            );
            let (parsed_payload, parse_error) = match parsed_payload {
                Ok(parsed_payload) => (Some(parsed_payload), None),
                Err(e) => (
                    None,
                    Some(api::Error::invalid_argument(
                        "unable to parse message payload",
                        e,
                    )),
                ),
            };

            let start = tokio::time::Instant::now();
            let start_time = std::time::SystemTime::now();
            let req = Arc::new(model::Request {
                span,
                parent_trace: parent_trace_id,
                parent_span: None,
                caller_event_id: None,
                ext_correlation_id: ext_correlation_id.cloned(),
                is_platform_request: false,
                internal_caller: None,
                start,
                start_time,
                data: RequestData::PubSub(PubSubRequestData {
                    service: self.obj.service.clone(),
                    topic: self.obj.topic.clone(),
                    subscription: self.obj.subscription.clone(),
                    message_id: msg.id.to_string(),
                    published: msg.publish_time.unwrap_or_else(Utc::now),
                    attempt: msg.attempt,
                    payload: msg.data.raw_body.clone(),
                    parsed_payload,
                }),
            });

            let logger = crate::log::root();
            logger.info(Some(&req), "starting request", None);

            self.obj.tracer.request_span_start(&req);

            let result = {
                // If we have a parse error, use that as the result immediately.
                if let Some(parse_error) = parse_error {
                    Err(parse_error)
                } else {
                    self.inner.handle_message(req.clone()).await
                }
            };

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
    pub fn new(
        tracer: Tracer,
        clusters: Vec<pb::PubSubCluster>,
        md: &meta::Data,
    ) -> anyhow::Result<Self> {
        let (topic_cfg, sub_cfg) = make_cfg_maps(clusters, md)?;

        Ok(Self {
            tracer,
            topic_cfg,
            sub_cfg,
            topics: Arc::default(),
            subs: Arc::default(),
            push_registry: PushHandlerRegistry::new(),
        })
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
            if let Some((cluster, topic_cfg, _schema)) = self.topic_cfg.get(&name) {
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
            if let Some((cluster, sub_cfg, meta_sub, schema)) = self.sub_cfg.get(&name) {
                let inner = cluster.subscription(sub_cfg, meta_sub);

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
                    schema: schema.clone(),
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

                    // We don't have a schema since it's an unknown subscription.
                    // Use a null schema.
                    schema: JSONSchema::null(),
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

#[allow(clippy::type_complexity)]
fn make_cfg_maps(
    clusters: Vec<pb::PubSubCluster>,
    md: &meta::Data,
) -> anyhow::Result<(
    HashMap<EncoreName, (Arc<dyn Cluster>, pb::PubSubTopic, JSONSchema)>,
    HashMap<
        SubName,
        (
            Arc<dyn Cluster>,
            pb::PubSubSubscription,
            meta::pub_sub_topic::Subscription,
            JSONSchema,
        ),
    >,
)> {
    let mut topic_map = HashMap::new();
    let mut sub_map = HashMap::new();

    let mut schema_builder = jsonschema::Builder::new(md);
    let (meta_topics, meta_subs) = {
        let mut topic_map = HashMap::new();
        let mut sub_map = HashMap::new();

        for topic in &md.pubsub_topics {
            let Some(schema_type) = &topic.message_type else {
                anyhow::bail!("topic {} has no message type", topic.name);
            };

            let schema_idx = schema_builder
                .register_type(schema_type)
                .with_context(|| format!("invalid schema for topic {}", topic.name))?;

            topic_map.insert(topic.name.clone(), (topic, schema_idx));
            for sub in &topic.subscriptions {
                let name = SubName {
                    topic: topic.name.clone().into(),
                    subscription: sub.name.clone().into(),
                };
                sub_map.insert(name, (sub, schema_idx));
            }
        }
        (topic_map, sub_map)
    };

    let schemas = schema_builder.build();
    for cluster_cfg in clusters {
        let cluster = new_cluster(&cluster_cfg);

        for topic_cfg in cluster_cfg.topics {
            let Some(&(_topic, idx)) = meta_topics.get(&topic_cfg.encore_name) else {
                anyhow::bail!("topic {} not found in metadata", topic_cfg.encore_name);
            };

            let schema = schemas.schema(idx);
            topic_map.insert(
                topic_cfg.encore_name.clone().into(),
                (cluster.clone(), topic_cfg, schema),
            );
        }

        for sub_cfg in cluster_cfg.subscriptions {
            let topic_name = sub_cfg.topic_encore_name.clone().into();
            let sub_name = sub_cfg.subscription_encore_name.clone().into();
            let name = SubName {
                topic: topic_name,
                subscription: sub_name,
            };
            let Some(&(meta_sub, idx)) = meta_subs.get(&name) else {
                continue;
            };

            let schema = schemas.schema(idx);
            sub_map.insert(
                name,
                (cluster.clone(), sub_cfg, meta_sub.to_owned(), schema),
            );
        }
    }

    Ok((topic_map, sub_map))
}

fn new_cluster(cluster: &pb::PubSubCluster) -> Arc<dyn Cluster> {
    let Some(provider) = &cluster.provider else {
        log::error!("missing PubSub cluster provider: {}", cluster.rid);
        return Arc::new(NoopCluster);
    };

    match provider {
        pb::pub_sub_cluster::Provider::Gcp(_) => return Arc::new(gcp::Cluster::new()),
        pb::pub_sub_cluster::Provider::Nsq(cfg) => {
            return Arc::new(nsq::Cluster::new(cfg.hosts[0].clone()));
        }
        pb::pub_sub_cluster::Provider::Aws(_) => return Arc::new(sqs_sns::Cluster::new()),
        pb::pub_sub_cluster::Provider::Encore(_) => {
            log::error!("Encore Cloud Pub/Sub not yet supported: {}", cluster.rid);
        }
        pb::pub_sub_cluster::Provider::Azure(_) => {
            log::error!("Azure Pub/Sub not yet supported: {}", cluster.rid);
        }
    }

    Arc::new(NoopCluster)
}
