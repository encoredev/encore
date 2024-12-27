use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::atomic::AtomicUsize;
use std::sync::{Arc, OnceLock, RwLock};

use anyhow::Context;
use chrono::Utc;
use futures::future::Shared;
use futures::FutureExt;

use crate::api::jsonschema::{self, JSONSchema};
use crate::api::{APIResult, PValues};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::parser::schema::v1 as schema;
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
    topic_cfg: HashMap<EncoreName, TopicConfig>,
    sub_cfg: HashMap<SubName, SubConfig>,

    topics: Arc<RwLock<HashMap<EncoreName, Arc<TopicInner>>>>,
    subs: Arc<RwLock<HashMap<SubName, Arc<SubscriptionObj>>>>,
    push_registry: PushHandlerRegistry,
}

#[derive(Debug)]
pub struct TopicObj {
    inner: Arc<TopicInner>,
}

#[derive(Debug)]
struct TopicInner {
    name: EncoreName,
    tracer: Tracer,
    imp: Arc<dyn Topic>,
    attr_fields: Arc<Vec<String>>,
}

impl TopicObj {
    pub fn publish(
        &self,
        payload: PValues,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = anyhow::Result<MessageId>> + 'static {
        self.inner.publish(payload, source)
    }
}

impl TopicInner {
    pub fn publish(
        &self,
        payload: PValues,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = anyhow::Result<MessageId>> + 'static {
        let tracer = self.tracer.clone();
        let inner = self.imp.clone();
        let name = self.name.clone();
        let attr_fields = self.attr_fields.clone();

        async move {
            let raw_body = serde_json::to_vec_pretty(&payload)
                .context("unable to serialize message payload")?;
            let mut msg = MessageData {
                attrs: HashMap::new(),
                raw_body,
            };

            for name in attr_fields.iter() {
                if let Some(val) = payload.get(name) {
                    msg.attrs.insert(name.clone(), val.to_string());
                }
            }

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

    handler: OnceLock<Arc<SubHandler>>,
    subscribe_fut: OnceLock<Shared<SubscribeFut>>,
}

type SubscribeFut = Pin<Box<dyn Future<Output = APIResult<()>> + Send>>;

impl SubscriptionObj {
    pub async fn subscribe(
        self: &Arc<Self>,
        handler: Arc<dyn SubscriptionHandler>,
    ) -> APIResult<()> {
        let h = self.handler.get_or_init(|| {
            Arc::new(SubHandler {
                obj: self.clone(),
                handlers: RwLock::new(Vec::new()),
                counter: AtomicUsize::new(0),
            })
        });
        h.add_handler(handler);

        self.subscribe_fut
            .get_or_init(|| self.inner.subscribe(h.clone()).shared())
            .clone()
            .await
    }
}

#[derive(Debug)]
pub struct SubHandler {
    obj: Arc<SubscriptionObj>,
    handlers: RwLock<Vec<Arc<dyn SubscriptionHandler>>>,
    counter: AtomicUsize,
}

const ATTR_PARENT_TRACE_ID: &str = "encore_parent_trace_id";
const ATTR_EXT_CORRELATION_ID: &str = "encore_ext_correlation_id";

impl SubHandler {
    fn add_handler(&self, h: Arc<dyn SubscriptionHandler>) {
        self.handlers.write().unwrap().push(h);
    }

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
                    let handler = self.next_handler();
                    handler.handle_message(req.clone()).await
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

    fn next_handler(&self) -> Arc<dyn SubscriptionHandler> {
        let handlers = self.handlers.read().unwrap();
        let n = handlers.len();
        // If we have a single handler, skip the increment and modulo steps.
        if n == 1 {
            return handlers[0].clone();
        }

        // Atomically increment the counter, and then get the next handler.
        let idx = self
            .counter
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        handlers[idx % n].clone()
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
        let inner = self.topic_impl(name)?;
        Some(TopicObj { inner })
    }

    fn topic_impl(&self, name: EncoreName) -> Option<Arc<TopicInner>> {
        if let Some(topic) = self.topics.read().unwrap().get(&name) {
            return Some(topic.clone());
        }

        let topic = Arc::new({
            if let Some(cfg) = self.topic_cfg.get(&name) {
                let imp = cfg.cluster.topic(&cfg.cfg);
                TopicInner {
                    name: name.clone(),
                    imp,
                    tracer: self.tracer.clone(),
                    attr_fields: cfg.attr_fields.clone(),
                }
            } else {
                TopicInner {
                    name: name.clone(),
                    imp: Arc::new(noop::NoopTopic),
                    tracer: self.tracer.clone(),
                    attr_fields: Arc::new(vec![]),
                }
            }
        });

        self.topics.write().unwrap().insert(name, topic.clone());
        Some(topic)
    }

    pub fn subscription(&self, name: SubName) -> Option<Arc<SubscriptionObj>> {
        if let Some(sub) = self.subs.read().unwrap().get(&name) {
            return Some(sub.clone());
        }

        let sub = {
            if let Some(cfg) = self.sub_cfg.get(&name) {
                let inner = cfg.cluster.subscription(&cfg.cfg, &cfg.meta);

                // If we have a push handler, register it.
                if let Some((sub_id, push_handler)) = inner.push_handler() {
                    self.push_registry.register(sub_id, push_handler);
                }

                Arc::new(SubscriptionObj {
                    inner,
                    tracer: self.tracer.clone(),
                    service: cfg.meta.service_name.clone().into(),
                    topic: name.topic.clone(),
                    subscription: name.subscription.clone(),
                    schema: cfg.schema.clone(),
                    handler: OnceLock::new(),
                    subscribe_fut: Default::default(),
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

                    handler: OnceLock::new(),
                    subscribe_fut: Default::default(),
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

#[derive(Debug)]
struct TopicConfig {
    cluster: Arc<dyn Cluster>,
    cfg: pb::PubSubTopic,

    /// Names of fields in the payload that should be copied into
    /// the PubSub message attributes.
    attr_fields: Arc<Vec<String>>,
}

#[derive(Debug)]
struct SubConfig {
    cluster: Arc<dyn Cluster>,
    cfg: pb::PubSubSubscription,
    meta: meta::pub_sub_topic::Subscription,
    schema: JSONSchema,
}

fn make_cfg_maps(
    clusters: Vec<pb::PubSubCluster>,
    md: &meta::Data,
) -> anyhow::Result<(
    HashMap<EncoreName, TopicConfig>,
    HashMap<SubName, SubConfig>,
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

            let attr_fields = message_attr_fields(&md.decls, schema_type, 0)?;
            let schema_idx = schema_builder
                .register_type(schema_type)
                .with_context(|| format!("invalid schema for topic {}", topic.name))?;

            topic_map.insert(topic.name.clone(), Arc::new(attr_fields));
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
            let Some(attr_fields) = meta_topics.get(&topic_cfg.encore_name) else {
                anyhow::bail!("topic {} not found in metadata", topic_cfg.encore_name);
            };
            topic_map.insert(
                topic_cfg.encore_name.clone().into(),
                TopicConfig {
                    cluster: cluster.clone(),
                    cfg: topic_cfg,
                    attr_fields: attr_fields.clone(),
                },
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
                SubConfig {
                    cluster: cluster.clone(),
                    cfg: sub_cfg,
                    meta: meta_sub.to_owned(),
                    schema,
                },
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

fn message_attr_fields(
    decls: &[schema::Decl],
    typ: &schema::Type,
    depth: u16,
) -> anyhow::Result<Vec<String>> {
    if depth > 100 {
        anyhow::bail!("pubsub message attribute computation: recursion depth exceeded");
    }
    let Some(typ) = &typ.typ else {
        return Ok(vec![]);
    };

    use schema::r#type::Typ;
    match typ {
        Typ::Named(named) => {
            let Some(decl) = decls.get(named.id as usize) else {
                anyhow::bail!("missing decl for named type");
            };
            let Some(decl_typ) = decl.r#type.as_ref() else {
                anyhow::bail!("missing type for named decl");
            };
            message_attr_fields(decls, decl_typ, depth + 1)
        }
        Typ::Struct(st) => {
            let names: Vec<String> = st
                .fields
                .iter()
                .filter_map(|f| {
                    f.tags.iter().find_map(|tag| {
                        if tag.key == "pubsub-attr" {
                            Some(tag.name.clone())
                        } else {
                            None
                        }
                    })
                })
                .collect();
            Ok(names)
        }

        Typ::Map(_)
        | Typ::List(_)
        | Typ::Builtin(_)
        | Typ::Pointer(_)
        | Typ::Union(_)
        | Typ::Literal(_)
        | Typ::TypeParameter(_)
        | Typ::Config(_) => Ok(vec![]),
    }
}
