use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::atomic::AtomicUsize;
use std::sync::{Arc, OnceLock, RwLock};

use anyhow::Context;
use chrono::Utc;
use futures::future::Shared;
use futures::FutureExt;
use tokio_util::sync::CancellationToken;

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
    publisher_id: xid::Id,

    topics: Arc<RwLock<HashMap<EncoreName, Arc<TopicInner>>>>,
    subs: Arc<RwLock<HashMap<SubName, Arc<SubscriptionObj>>>>,
    push_registry: PushHandlerRegistry,
    cancel: CancellationToken,
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
    ordering_attr: Option<String>,
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
        let ordering_attr = self.ordering_attr.clone();
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

            let ordering_key: Option<String> = if let Some(attr) = ordering_attr {
                Some(
                    msg.attrs
                        .get(&attr)
                        .with_context(|| format!("ordering attribute {attr} not found"))?
                        .clone(),
                )
            } else {
                None
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
                // If this is a traced platform request, propagate the sampled flag so that
                // subscribers always trace platform-initiated messages.
                // We check both is_platform_request and traced so that scheduled cron jobs
                // that were sampled out don't force-trace their downstream subscribers.
                if source.is_platform_request && source.traced {
                    msg.attrs
                        .insert(ATTR_FORCE_TRACE.to_string(), "true".to_string());
                }
                let start_id = tracer.pubsub_publish_start(protocol::PublishStartData {
                    source,
                    topic: &name,
                    payload: &msg.raw_body,
                });
                let result = inner.publish(msg, ordering_key).await;
                tracer.pubsub_publish_end(protocol::PublishEndData {
                    start_id,
                    source,
                    result: &result,
                });
                result
            } else {
                inner.publish(msg, ordering_key).await
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
    cancel: CancellationToken,

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
                in_flight: Arc::new(InFlightTracker::new()),
            })
        });
        h.add_handler(handler);

        self.subscribe_fut
            .get_or_init(|| {
                self.inner
                    .subscribe(h.clone(), self.cancel.child_token())
                    .shared()
            })
            .clone()
            .await
    }
}

#[derive(Debug)]
pub struct SubHandler {
    obj: Arc<SubscriptionObj>,
    handlers: RwLock<Vec<Arc<dyn SubscriptionHandler>>>,
    counter: AtomicUsize,
    in_flight: Arc<InFlightTracker>,
}

/// Tracks in-flight message handlers so we can wait for them to drain.
#[derive(Debug)]
struct InFlightTracker {
    count: AtomicUsize,
    notify: tokio::sync::Notify,
}

impl InFlightTracker {
    fn new() -> Self {
        Self {
            count: AtomicUsize::new(0),
            notify: tokio::sync::Notify::new(),
        }
    }

    fn acquire(&self) {
        self.count
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
    }

    fn release(&self) {
        if self
            .count
            .fetch_sub(1, std::sync::atomic::Ordering::Release)
            == 1
        {
            self.notify.notify_waiters();
        }
    }

    async fn drain(&self) {
        while self.count.load(std::sync::atomic::Ordering::Acquire) > 0 {
            self.notify.notified().await;
        }
    }
}

/// Guard that releases the in-flight tracker on drop.
struct InFlightGuard(Arc<InFlightTracker>);

impl Drop for InFlightGuard {
    fn drop(&mut self) {
        self.0.release();
    }
}

type PubSubHandlerFuture = Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send>>;

/// Guard that spawns the pubsub handler into a background task on cancellation,
/// ensuring `request_span_end` is always emitted. On the normal path this is a no-op.
struct PubSubCancellationGuard {
    fut: Option<PubSubHandlerFuture>,
    info: Option<PubSubCancellationGuardInfo>,
    _in_flight: InFlightGuard,
}

struct PubSubCancellationGuardInfo {
    tracer: Tracer,
    request: Arc<model::Request>,
    start: tokio::time::Instant,
}

impl PubSubCancellationGuard {
    async fn run(&mut self) -> Result<(), api::Error> {
        let result = match self.fut.as_mut() {
            Some(fut) => std::future::poll_fn(|cx| fut.as_mut().poll(cx)).await,
            None => Err(api::Error::internal(anyhow::anyhow!(
                "handler already completed"
            ))),
        };
        self.fut = None;
        self.info = None; // disarm
        result
    }
}

impl Drop for PubSubCancellationGuard {
    fn drop(&mut self) {
        let Some(info) = self.info.take() else {
            return;
        };
        if let Some(fut) = self.fut.take() {
            // Take the in-flight guard so it moves into the spawned task,
            // keeping the counter incremented until the handler finishes.
            let in_flight = std::mem::replace(
                &mut self._in_flight,
                InFlightGuard(Arc::new(InFlightTracker::new())),
            );
            tokio::spawn(async move {
                let _in_flight = in_flight;
                let result = fut.await;
                let duration = tokio::time::Instant::now().duration_since(info.start);
                let resp = model::Response {
                    request: info.request,
                    duration,
                    data: ResponseData::PubSub(result),
                };
                info.tracer.request_span_end(&resp, false);
            });
        }
    }
}

const ATTR_PARENT_TRACE_ID: &str = "encore_parent_trace_id";
const ATTR_EXT_CORRELATION_ID: &str = "encore_ext_correlation_id";
const ATTR_FORCE_TRACE: &str = "encore_force_trace";

impl SubHandler {
    fn add_handler(&self, h: Arc<dyn SubscriptionHandler>) {
        self.handlers.write().unwrap().push(h);
    }

    pub(super) fn handle_message(
        &self,
        msg: Message,
    ) -> Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send + 'static>> {
        let obj = self.obj.clone();
        let next_handler = self.next_handler();
        self.in_flight.acquire();
        let in_flight_guard = InFlightGuard(self.in_flight.clone());
        Box::pin(async move {
            let span = SpanKey(TraceId::generate(), SpanId::generate());

            let parent_trace_id: Option<TraceId> = msg
                .data
                .attrs
                .get(ATTR_PARENT_TRACE_ID)
                .and_then(|s| TraceId::parse_encore(s).ok());
            let ext_correlation_id = msg.data.attrs.get(ATTR_EXT_CORRELATION_ID);

            // If force trace is set, always trace. Otherwise, make an independent sampling decision.
            let traced = msg
                .data
                .attrs
                .get(ATTR_FORCE_TRACE)
                .is_some_and(|s| s == "true")
                || obj
                    .tracer
                    .should_sample_pubsub(&obj.service, &obj.topic, &obj.subscription);

            let mut de = serde_json::Deserializer::from_slice(&msg.data.raw_body);
            let parsed_payload = obj.schema.deserialize(
                &mut de,
                jsonschema::DecodeConfig {
                    coerce_strings: false,
                    arrays_as_repeated_fields: false,
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
                    service: obj.service.clone(),
                    topic: obj.topic.clone(),
                    subscription: obj.subscription.clone(),
                    message_id: msg.id.to_string(),
                    published: msg.publish_time.unwrap_or_else(Utc::now),
                    attempt: msg.attempt,
                    payload: msg.data.raw_body.clone(),
                    parsed_payload,
                }),
                traced,
            });

            let logger = crate::log::root();
            logger.info(Some(&req), "starting request", None);

            obj.tracer.request_span_start(&req, false);

            // Build the handler future and wrap it in a HandlerCall so the
            // cancellation guard can spawn it into a background task if
            // this future is cancelled.
            let handler_fut: Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send>> =
                if let Some(parse_error) = parse_error {
                    Box::pin(std::future::ready(Err(parse_error)))
                } else {
                    next_handler.handle_message(req.clone())
                };

            let mut guard = PubSubCancellationGuard {
                fut: Some(handler_fut),
                info: Some(PubSubCancellationGuardInfo {
                    tracer: obj.tracer.clone(),
                    request: req.clone(),
                    start,
                }),
                _in_flight: in_flight_guard,
            };

            let result = guard.run().await;

            let duration = tokio::time::Instant::now().duration_since(start);

            logger.info(Some(&req), "request completed", None);

            let resp = model::Response {
                request: req,
                duration,
                data: ResponseData::PubSub(result.clone()),
            };
            obj.tracer.request_span_end(&resp, false);
            result
        })
    }

    pub(super) fn topic(&self) -> &EncoreName {
        &self.obj.topic
    }

    pub(super) fn subscription(&self) -> &EncoreName {
        &self.obj.subscription
    }

    /// Waits for all in-flight message handlers to complete.
    pub(super) async fn drain_in_flight(&self) {
        self.in_flight.drain().await;
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
            publisher_id: xid::new(),
            tracer,
            topic_cfg,
            sub_cfg,
            topics: Arc::default(),
            subs: Arc::default(),
            push_registry: PushHandlerRegistry::new(),
            cancel: CancellationToken::new(),
        })
    }

    /// Returns the cancellation token for all subscriptions.
    pub fn cancel_token(&self) -> CancellationToken {
        self.cancel.clone()
    }

    /// Waits for all in-flight message handlers to complete.
    pub async fn drain(&self) {
        let subs = self.subs.read().expect("subs lock poisoned").clone();
        for sub in subs.values() {
            if let Some(handler) = sub.handler.get() {
                handler.in_flight.drain().await;
            }
        }
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
                let imp = cfg.cluster.topic(&cfg.cfg, self.publisher_id);
                TopicInner {
                    name: name.clone(),
                    imp,
                    tracer: self.tracer.clone(),
                    attr_fields: cfg.attr_fields.clone(),
                    ordering_attr: cfg.cfg.ordering_attr.clone(),
                }
            } else {
                TopicInner {
                    name: name.clone(),
                    imp: Arc::new(noop::NoopTopic),
                    tracer: self.tracer.clone(),
                    attr_fields: Arc::new(vec![]),
                    ordering_attr: None,
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
                    cancel: self.cancel.child_token(),
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

                    cancel: self.cancel.child_token(),
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
        | Typ::Option(_)
        | Typ::Union(_)
        | Typ::Literal(_)
        | Typ::TypeParameter(_)
        | Typ::Config(_) => Ok(vec![]),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use futures::FutureExt;
    use std::sync::Arc;

    fn count(tracker: &InFlightTracker) -> usize {
        tracker.count.load(std::sync::atomic::Ordering::Acquire)
    }

    #[test]
    fn drain_completes_immediately_when_empty() {
        let tracker = InFlightTracker::new();
        assert!(tracker.drain().now_or_never().is_some());
    }

    #[test]
    fn drain_is_pending_while_guard_held() {
        let tracker = Arc::new(InFlightTracker::new());
        tracker.acquire();
        let guard = InFlightGuard(tracker.clone());

        assert!(tracker.drain().now_or_never().is_none());

        drop(guard);
        assert!(tracker.drain().now_or_never().is_some());
    }

    #[test]
    fn drain_is_pending_until_all_guards_dropped() {
        let tracker = Arc::new(InFlightTracker::new());
        tracker.acquire();
        tracker.acquire();
        let guard1 = InFlightGuard(tracker.clone());
        let guard2 = InFlightGuard(tracker.clone());

        assert!(tracker.drain().now_or_never().is_none());

        drop(guard1);
        assert!(tracker.drain().now_or_never().is_none());

        drop(guard2);
        assert!(tracker.drain().now_or_never().is_some());
    }

    #[test]
    fn guard_decrements_count_on_drop() {
        let tracker = Arc::new(InFlightTracker::new());
        tracker.acquire();
        tracker.acquire();
        assert_eq!(count(&tracker), 2);

        let guard = InFlightGuard(tracker.clone());
        tracker.release();
        assert_eq!(count(&tracker), 1);

        drop(guard);
        assert_eq!(count(&tracker), 0);
    }

    #[tokio::test]
    async fn guard_decrements_on_panic() {
        let tracker = Arc::new(InFlightTracker::new());
        tracker.acquire();

        let tracker_clone = tracker.clone();
        let result = tokio::spawn(async move {
            let _guard = InFlightGuard(tracker_clone);
            panic!("handler panicked");
        })
        .await;

        assert!(result.is_err());
        assert_eq!(count(&tracker), 0);
        assert!(tracker.drain().now_or_never().is_some());
    }
}
