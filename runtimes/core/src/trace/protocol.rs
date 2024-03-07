//! Implements the trace protocol.

use std::sync::atomic::{AtomicU64, Ordering};

use crate::api;
use crate::model::{Request, TraceEventId};
use crate::trace::eventbuf::EventBuffer;
use crate::trace::log::TraceEvent;
use crate::{model, EncoreName};

/// Represents a type of trace event.
#[derive(Debug, Clone, Copy)]
#[repr(u8)]
pub enum EventType {
    RequestSpanStart = 0x01,
    RequestSpanEnd = 0x02,
    AuthSpanStart = 0x03,
    AuthSpanEnd = 0x04,
    PubsubMessageSpanStart = 0x05,
    PubsubMessageSpanEnd = 0x06,
    DBTransactionStart = 0x07,
    DBTransactionEnd = 0x08,
    DBQueryStart = 0x09,
    DBQueryEnd = 0x0A,
    RPCCallStart = 0x0B,
    RPCCallEnd = 0x0C,
    HTTPCallStart = 0x0D,
    HTTPCallEnd = 0x0E,
    LogMessage = 0x0F,
    PubsubPublishStart = 0x10,
    PubsubPublishEnd = 0x11,
    ServiceInitStart = 0x12,
    ServiceInitEnd = 0x13,
    CacheCallStart = 0x14,
    CacheCallEnd = 0x15,
    BodyStream = 0x16,
}

// A global event id counter.
static EVENT_ID: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Clone)]
pub struct Tracer {
    tx: Option<tokio::sync::mpsc::UnboundedSender<TraceEvent>>,
}

pub static TRACE_VERSION: u16 = 14;

impl Tracer {
    pub(super) fn new(tx: tokio::sync::mpsc::UnboundedSender<TraceEvent>) -> Self {
        Self { tx: Some(tx) }
    }

    pub fn noop() -> Self {
        Self { tx: None }
    }
}

impl Tracer {
    #[inline]
    pub fn request_span_start(&self, req: &model::Request) {
        let mut eb = SpanStartEventData {
            parent: Parent::from(req),
            caller_event_id: req.caller_event_id,
            ext_correlation_id: req.ext_correlation_id.as_deref(),
            extra_space: 100,
        }
        .to_eb();

        let event_type = match &req.data {
            model::RequestData::RPC(rpc) => {
                eb.str(&rpc.endpoint.name.service());
                eb.str(&rpc.endpoint.name.endpoint());
                eb.str(rpc.method.as_str());
                eb.str(&rpc.path);

                // Encode path params. We only encode the values since the keys are known in metadata.
                {
                    let path_params = rpc.parsed_payload.as_ref().and_then(|p| p.path.as_ref());
                    if let Some(path_params) = path_params {
                        eb.uvarint(path_params.len() as u64);
                        for (_, v) in path_params {
                            match &v {
                                serde_json::Value::String(s) => eb.str(s.as_str()),
                                other => eb.str(other.to_string().as_str()),
                            }
                        }
                    } else {
                        eb.uvarint(0u64);
                    }
                }

                // Encode request headers. If a header has multiple values it is encoded multiple times.
                eb.headers(&rpc.req_headers);

                let payload = rpc
                    .parsed_payload
                    .as_ref()
                    .and_then(|p| serde_json::to_vec_pretty(p).ok());
                eb.opt_byte_string(payload.as_deref());

                eb.opt_str(req.ext_correlation_id.as_deref()); // yes, this is repeated for some reason
                eb.opt_str(rpc.auth_user_id.as_deref());

                EventType::RequestSpanStart
            }

            model::RequestData::Auth(auth) => {
                let name = &auth.auth_handler;
                eb.str(&name.service());
                eb.str(&name.endpoint());

                // TODO: non-raw payload.
                eb.byte_string(&[]);

                EventType::AuthSpanStart
            }
            model::RequestData::PubSub(msg_data) => {
                eb.str(&msg_data.service);
                eb.str(&msg_data.topic);
                eb.str(&msg_data.subscription);
                eb.str(&msg_data.message_id);
                eb.uvarint(msg_data.attempt as u64);
                eb.time(&msg_data.published);
                eb.byte_string(&msg_data.payload);

                EventType::PubsubMessageSpanStart
            }
        };

        _ = self.send(event_type, req.span, eb);
    }

    #[inline]
    pub fn request_span_end(&self, resp: &model::Response) {
        // If the request has no span, we don't need to do anything.
        let req = resp.request.as_ref();

        let mut eb = SpanEndEventData {
            parent: Parent::from(req),
            duration: resp.duration,
            err: match &resp.data {
                model::ResponseData::RPC(rpc) => rpc.error.as_ref(),
                model::ResponseData::Auth(res) => res.as_ref().err(),
                model::ResponseData::PubSub(res) => res.as_ref().err(),
            },
            extra_space: 100,
        }
        .to_eb();

        match &req.data {
            model::RequestData::RPC(req_data) => {
                eb.str(&req_data.endpoint.name.service());
                eb.str(&req_data.endpoint.name.endpoint());
            }
            model::RequestData::Auth(auth_data) => {
                let name = &auth_data.auth_handler;
                eb.str(&name.service());
                eb.str(&name.endpoint());
            }
            model::RequestData::PubSub(msg_data) => {
                eb.str(&msg_data.service);
                eb.str(&msg_data.topic);
                eb.str(&msg_data.subscription);
            }
        }

        let event_type = match &resp.data {
            model::ResponseData::RPC(resp_data) => {
                eb.uvarint(resp_data.status_code);
                eb.headers(&resp_data.resp_headers);

                if let Some(payload) = &resp_data.resp_payload {
                    let payload = serde_json::to_vec_pretty(payload).unwrap_or_default();
                    eb.byte_string(&payload);
                } else {
                    eb.byte_string(&[]);
                }

                EventType::RequestSpanEnd
            }
            model::ResponseData::Auth(auth_result) => {
                match auth_result {
                    Ok(auth_success) => {
                        eb.str(auth_success.user_id.as_str());
                        let user_data = serde_json::to_string(&auth_success.user_data)
                            .unwrap_or_else(|_| String::new());
                        eb.str(&user_data);
                    }
                    Err(_) => {
                        eb.str(""); // auth uid
                        eb.str(""); // response payload
                    }
                }

                EventType::AuthSpanEnd
            }

            model::ResponseData::PubSub(_) => EventType::PubsubMessageSpanEnd,
        };

        _ = self.send(event_type, req.span, eb);
    }
}

impl Tracer {
    #[inline]
    pub fn rpc_call_start(&self, call: &model::APICall) -> Option<TraceEventId> {
        let Some(source) = call.source else {
            return None;
        };

        let (service, endpoint) = (call.target.service(), call.target.endpoint());
        let mut eb = BasicEventData {
            correlation_event_id: None,
            extra_space: 4 + 4 + service.len() + endpoint.len(),
        }
        .to_eb();

        eb.str(service);
        eb.str(endpoint);
        eb.nyi_stack_pcs();

        Some(self.send(EventType::RPCCallStart, source.span, eb))
    }

    #[inline]
    pub fn rpc_call_end(
        &self,
        call: &model::APICall,
        start_event_id: TraceEventId,
        err: Option<&api::Error>,
    ) {
        let Some(source) = call.source else {
            return;
        };

        let (service, endpoint) = (call.target.service(), call.target.endpoint());
        let mut eb = BasicEventData {
            correlation_event_id: Some(start_event_id),
            extra_space: 4 + 4 + service.len() + endpoint.len(),
        }
        .to_eb();

        eb.api_err_with_legacy_stack(err);

        _ = self.send(EventType::RPCCallEnd, source.span, eb);
    }
}

pub struct PublishStartData<'a> {
    pub source: &'a Request,
    pub topic: &'a EncoreName,
    pub payload: &'a [u8],
}

pub struct PublishEndData<'a> {
    pub start_id: TraceEventId,
    pub source: &'a Request,
    pub result: &'a anyhow::Result<String>,
}

impl Tracer {
    #[inline]
    pub fn pubsub_publish_start(&self, data: PublishStartData) -> TraceEventId {
        let mut eb = BasicEventData {
            correlation_event_id: None,
            extra_space: 4 + 4 + 8 + data.topic.len() + data.payload.len(),
        }
        .to_eb();

        eb.str(&data.topic);
        eb.byte_string(data.payload);
        eb.nyi_stack_pcs();

        self.send(EventType::PubsubPublishStart, data.source.span, eb)
    }

    #[inline]
    pub fn pubsub_publish_end(&self, data: PublishEndData) {
        let mut eb = BasicEventData {
            correlation_event_id: Some(data.start_id),
            extra_space: 4 + 4 + 8,
        }
        .to_eb();

        eb.str(data.result.as_deref().unwrap_or(""));
        eb.err_with_legacy_stack(data.result.as_ref().err());

        _ = self.send(EventType::PubsubPublishEnd, data.source.span, eb);
    }
}

pub struct DBQueryStartData<'a> {
    pub source: &'a Request,
    pub query: &'a str,
}

pub struct DBQueryEndData<'a, E> {
    pub start_id: TraceEventId,
    pub source: &'a Request,
    pub error: Option<&'a E>,
}

impl Tracer {
    #[inline]
    pub fn db_query_start(&self, data: DBQueryStartData) -> TraceEventId {
        let mut eb = BasicEventData {
            correlation_event_id: None,
            extra_space: 4 + 4 + data.query.len() + 32,
        }
        .to_eb();

        eb.str(&data.query);
        eb.nyi_stack_pcs();

        self.send(EventType::DBQueryStart, data.source.span, eb)
    }

    #[inline]
    pub fn db_query_end<E>(&self, data: DBQueryEndData<E>)
    where
        E: std::fmt::Display,
    {
        let mut eb = BasicEventData {
            correlation_event_id: Some(data.start_id),
            extra_space: 4 + 4 + 8,
        }
        .to_eb();

        eb.err_with_legacy_stack(data.error);

        _ = self.send(EventType::DBQueryEnd, data.source.span, eb);
    }
}

impl Tracer {
    #[inline]
    fn send(&self, typ: EventType, span: model::SpanKey, eb: EventBuffer) -> model::TraceEventId {
        // Make sure the event id is never 0, as it's used to indicate "no event" in the protocol.
        let mut id = EVENT_ID.fetch_add(1, Ordering::SeqCst);
        if id == 0 {
            id = EVENT_ID.fetch_add(1, Ordering::SeqCst);
        }
        let id = model::TraceEventId(id);

        // If we have a sender, send the event. Otherwise this is a no-op tracer.
        if let Some(tx) = &self.tx {
            _ = tx.send(TraceEvent {
                typ,
                span,
                id,
                data: eb.freeze(),
                ts: tokio::time::Instant::now(),
            });
        }

        id
    }
}

impl EventBuffer {
    fn parent(&mut self, parent: Option<&Parent>) {
        self.reserve(16 + 8);

        if let Some(parent) = parent {
            match parent {
                Parent::Trace(trace) => {
                    self.bytes(&trace.0);
                    self.bytes(&[0; 8]);
                }
                Parent::Span(span) => {
                    self.bytes(&span.0 .0);
                    self.bytes(&span.1 .0);
                }
            }
        } else {
            self.bytes(&[0; 16]);
            self.bytes(&[0; 8]);
        }
    }

    /// Writes a span key to the buffer.
    /// Writes 0 bytes if key is None.
    fn span_key(&mut self, key: Option<model::SpanKey>) {
        self.reserve(16 + 8);
        match key {
            Some(key) => {
                self.bytes(&key.0 .0);
                self.bytes(&key.1 .0);
            }
            None => {
                self.bytes(&[0; 16]);
                self.bytes(&[0; 8]);
            }
        }
    }

    fn event_id(&mut self, event_id: Option<model::TraceEventId>) {
        self.uvarint(match event_id {
            Some(event_id) => event_id.0,
            None => 0,
        });
    }

    fn opt_str(&mut self, s: Option<&str>) {
        let str = s.unwrap_or("");
        self.str(str);
    }

    fn opt_byte_string(&mut self, s: Option<&[u8]>) {
        let bytes = s.unwrap_or(&[]);
        self.byte_string(bytes);
    }

    fn headers(&mut self, headers: &axum::http::HeaderMap) {
        self.uvarint(headers.len() as u64);
        for (k, v) in headers.iter() {
            self.str(k.as_str());
            self.str(v.to_str().unwrap_or(""));
        }
    }
}

#[derive(Debug, Clone)]
enum Parent {
    Trace(model::TraceId),
    Span(model::SpanKey),
}

impl Parent {
    fn from(req: &model::Request) -> Option<Self> {
        if let Some(span) = req.parent_span {
            Some(Parent::Span(span))
        } else {
            req.parent_trace.map(|t| Parent::Trace(t))
        }
    }
}

struct SpanStartEventData<'a> {
    parent: Option<Parent>,
    caller_event_id: Option<model::TraceEventId>,
    ext_correlation_id: Option<&'a str>,

    /// Additional extra space to allocate in the buffer.
    extra_space: usize,
}

impl SpanStartEventData<'_> {
    pub fn to_eb(self) -> EventBuffer {
        let correlation_len = self.ext_correlation_id.map(|s| s.len()).unwrap_or(0);
        let mut eb =
            EventBuffer::with_capacity(4 + 16 + 8 + 4 + correlation_len + 2 + self.extra_space);

        eb.uvarint(0u64); // TODO: GOID
        eb.parent(self.parent.as_ref());
        eb.uvarint(0u64); // TODO: def loc
        eb.event_id(self.caller_event_id);
        eb.opt_str(self.ext_correlation_id);

        eb
    }
}

struct SpanEndEventData<'a> {
    parent: Option<Parent>,
    duration: std::time::Duration,
    err: Option<&'a api::Error>,

    /// Additional extra space to allocate in the buffer.
    extra_space: usize,
}

impl SpanEndEventData<'_> {
    pub fn to_eb(self) -> EventBuffer {
        let mut eb = EventBuffer::with_capacity(8 + 12 + 8 + self.extra_space);

        eb.duration(self.duration);
        eb.api_err_with_legacy_stack(self.err);
        eb.nyi_formatted_stack();
        eb.parent(self.parent.as_ref());

        eb
    }
}

struct BasicEventData {
    correlation_event_id: Option<model::TraceEventId>,

    /// Additional extra space to allocate in the buffer.
    extra_space: usize,
}

impl BasicEventData {
    pub fn to_eb(self) -> EventBuffer {
        let mut eb = EventBuffer::with_capacity(4 + 4 + self.extra_space);

        eb.uvarint(0u64); // TODO: def loc
        eb.uvarint(0u64); // TODO: GOID
        eb.event_id(self.correlation_event_id);

        eb
    }
}
