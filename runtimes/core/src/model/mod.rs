use chrono::Utc;
use indexmap::IndexMap;
use std::fmt::Debug;
use std::str::FromStr;
use std::sync::Arc;
use std::time::SystemTime;

use rand::RngCore;
use tokio::time::Instant;

use crate::api::reqauth::caller::Caller;
use crate::api::schema::JSONPayload;
use crate::api::{auth, Endpoint};
use crate::{api, EncoreName, EndpointName};

#[derive(Clone, Copy, Debug, Hash, Eq, PartialEq)]
pub struct TraceId(pub [u8; 16]);

#[derive(Clone, Copy, Debug, Hash, Eq, PartialEq)]
pub struct SpanId(pub [u8; 8]);

/// Uniquely identifies a span.
#[derive(Clone, Copy, Debug, Hash, Eq, PartialEq)]
pub struct SpanKey(pub TraceId, pub SpanId);

/// Uniquely identifies an event within a trace.
#[derive(Clone, Copy, Debug, Hash, Eq, PartialEq)]
#[must_use]
pub struct TraceEventId(pub u64);

impl FromStr for TraceEventId {
    type Err = std::num::ParseIntError;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let id = u64::from_str_radix(s, 32)?;
        Ok(TraceEventId(id))
    }
}

impl TraceEventId {
    pub fn serialize(&self) -> String {
        radix_fmt::radix(self.0, 32).to_string()
    }
}

impl TraceId {
    pub fn generate() -> Self {
        let mut trace_id = [0u8; 16];
        rand::thread_rng().fill_bytes(&mut trace_id);
        TraceId(trace_id)
    }

    pub fn serialize_encore(&self) -> String {
        crate::base32::encode(crate::base32::Alphabet::Encore, &self.0)
    }

    pub fn serialize_std(&self) -> String {
        hex::encode(&self.0)
    }

    pub fn parse_encore(s: &str) -> Result<Self, InvalidBase32> {
        let Some(bytes) = crate::base32::decode(crate::base32::Alphabet::Encore, s) else {
            return Err(InvalidBase32);
        };
        let trace_id: [u8; 16] = bytes.try_into().map_err(|_| InvalidBase32)?;
        Ok(TraceId(trace_id))
    }

    pub fn parse_std(s: &str) -> Result<Self, hex::FromHexError> {
        let bytes = hex::decode(s)?;
        let trace_id: [u8; 16] = bytes
            .try_into()
            .map_err(|_| hex::FromHexError::InvalidStringLength)?;
        Ok(TraceId(trace_id))
    }

    pub fn with_span(&self, span_id: SpanId) -> SpanKey {
        SpanKey(*self, span_id)
    }
}

pub struct InvalidBase32;

impl SpanId {
    pub fn generate() -> Self {
        let mut span_id = [0u8; 8];
        rand::thread_rng().fill_bytes(&mut span_id);
        SpanId(span_id)
    }

    pub fn serialize_encore(&self) -> String {
        crate::base32::encode(crate::base32::Alphabet::Encore, &self.0)
    }

    pub fn serialize_std(&self) -> String {
        hex::encode(&self.0)
    }

    pub fn parse_encore(s: &str) -> Result<Self, InvalidBase32> {
        let Some(bytes) = crate::base32::decode(crate::base32::Alphabet::Encore, s) else {
            return Err(InvalidBase32);
        };
        let span_id: [u8; 8] = bytes.try_into().map_err(|_| InvalidBase32)?;
        Ok(SpanId(span_id))
    }

    pub fn parse_std(s: &str) -> Result<Self, hex::FromHexError> {
        let bytes = hex::decode(s)?;
        let span_id: [u8; 8] = bytes
            .try_into()
            .map_err(|_| hex::FromHexError::InvalidStringLength)?;
        Ok(SpanId(span_id))
    }
}

pub struct APICall<'a> {
    pub source: Option<&'a Request>,
    pub target: &'a EndpointName,
}

#[derive(Debug)]
pub struct Request {
    /// The span for this request.
    /// Always set even if the request is not traced, as it's used for request tracking.
    pub span: SpanKey,

    /// The trace that generated this trace.
    pub parent_trace: Option<TraceId>,

    /// The parent span for this request.
    pub parent_span: Option<SpanKey>,

    /// The event ID of the caller, if any.
    pub caller_event_id: Option<TraceEventId>,

    /// The externally-provided correlation ID, if any.
    pub ext_correlation_id: Option<String>,

    /// True if the request originated from the Encore Platform.
    pub is_platform_request: bool,

    /// Who's making the request, if any.
    pub internal_caller: Option<Caller>,

    /// When the request started.
    pub start: Instant,
    pub start_time: SystemTime,

    /// Type-specific data.
    pub data: RequestData,
}

impl Request {
    pub fn allows_private_endpoint_call(&self) -> bool {
        if self.is_platform_request {
            true
        } else if let Some(caller) = &self.internal_caller {
            caller.private_api_access()
        } else {
            false
        }
    }

    pub fn has_authenticated_user(&self) -> bool {
        match &self.data {
            RequestData::RPC(data) => data.auth_user_id.is_some(),
            RequestData::Auth(_) => false,
            RequestData::PubSub(_) => false,
        }
    }

    pub fn take_raw_body(&self) -> Option<axum::body::Body> {
        if let RequestData::RPC(data) = &self.data {
            if let Some(data) = data.parsed_payload.as_ref() {
                if let api::Body::Raw(body) = &data.body {
                    return body.lock().unwrap().take();
                }
            }
        }
        None
    }
}

#[derive(Debug)]
pub enum RequestData {
    RPC(RPCRequestData),
    Auth(AuthRequestData),
    PubSub(PubSubRequestData),
}

#[derive(Debug)]
pub struct RPCRequestData {
    /// The description of the endpoint.
    pub endpoint: Arc<Endpoint>,

    /// The request method.
    pub method: api::schema::Method,

    /// The request path.
    pub path: String,
    pub path_and_query: String,

    /// The request path params, if any.
    pub path_params: Option<IndexMap<String, serde_json::Value>>,

    /// The request headers
    pub req_headers: axum::http::HeaderMap,

    /// The authenticated user id, if any.
    pub auth_user_id: Option<String>,

    /// The user data for the authenticated user, if any.
    pub auth_data: Option<serde_json::Map<String, serde_json::Value>>,

    /// The parsed application payload.
    pub parsed_payload: Option<api::RequestPayload>,
}

pub struct AuthRequestData {
    /// The name of the auth handler.
    pub auth_handler: EndpointName,

    /// The parsed authentication payload.
    pub parsed_payload: auth::AuthPayload,
}

impl Debug for AuthRequestData {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AuthRequestData")
            .field("auth_handler", &self.auth_handler)
            .field("parsed_payload", &self.parsed_payload)
            .finish()
    }
}

#[derive(Debug)]
pub struct PubSubRequestData {
    /// The service processing the message.
    pub service: EncoreName,
    pub topic: EncoreName,
    pub subscription: EncoreName,
    pub message_id: String,
    pub published: chrono::DateTime<Utc>,
    pub attempt: u32,
    pub payload: Vec<u8>,
    pub parsed_payload: Option<serde_json::Value>,
}

#[derive(Debug)]
pub struct Response {
    /// The request this response is for.
    pub request: Arc<Request>,

    /// How long the request took.
    pub duration: std::time::Duration,

    /// The result of the response.
    pub data: ResponseData,
}

#[derive(Debug)]
pub enum ResponseData {
    RPC(RPCResponseData),
    Auth(Result<AuthSuccessResponse, api::Error>),
    PubSub(Result<(), api::Error>),
}

#[derive(Debug)]
pub struct RPCResponseData {
    /// The response status code.
    pub status_code: u16,

    /// The response payload.
    pub resp_payload: Option<JSONPayload>,

    /// The response headers.
    pub resp_headers: axum::http::HeaderMap,

    /// Any error that occurred.
    pub error: Option<api::Error>,
}

#[derive(Debug)]
pub struct AuthSuccessResponse {
    /// The resolved user id.
    pub user_id: String,

    /// The user data.
    pub user_data: serde_json::Map<String, serde_json::Value>,
}

// matches go runtime
pub enum LogLevel {
    Trace = 0,
    Debug,
    Info,
    Warn,
    Error,
}

pub enum LogFieldValue<'a> {
    String(&'a str),
    U64(u64),
    I64(i64),
    F64(f64),
    Bool(bool),
    Json(&'a serde_json::Value),
}

pub struct LogField<'a> {
    pub key: &'a str,
    pub value: LogFieldValue<'a>,
}

impl LogField<'_> {
    pub fn type_byte(&self) -> u8 {
        match self.value {
            LogFieldValue::String(_) => 2,
            LogFieldValue::Bool(_) => 3,
            LogFieldValue::I64(_) => 8,
            LogFieldValue::Json(_) => 7,
            LogFieldValue::U64(_) => 9,
            LogFieldValue::F64(_) => 11,
        }
    }
}
