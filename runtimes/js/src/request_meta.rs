use chrono::{DateTime, SecondsFormat, Utc};
use napi_derive::napi;

use encore_runtime_core::model;

use crate::pvalue::PVals;

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("invalid json")]
    SerializeJson(#[from] serde_json::Error),
    #[error("mutex poisoned")]
    MutexPoisoned,
}

impl<T> From<std::sync::PoisonError<T>> for Error {
    fn from(_value: std::sync::PoisonError<T>) -> Self {
        Self::MutexPoisoned
    }
}

impl From<Error> for napi::Error {
    fn from(err: Error) -> Self {
        match err {
            Error::SerializeJson(error) => napi::Error::from(error),
            Error::MutexPoisoned => napi::Error::from(err),
        }
    }
}

pub fn meta(req: &model::Request) -> Result<RequestMeta, Error> {
    let dt: DateTime<Utc> = req.start_time.into();
    let started_at = dt.to_rfc3339_opts(SecondsFormat::Secs, true);

    let (api_call, pubsub_message) = match &req.data {
        model::RequestData::RPC(rpc) => {
            let api = APICallData {
                api: APIDesc {
                    service: rpc.endpoint.name.service().to_string(),
                    endpoint: rpc.endpoint.name.endpoint().to_string(),
                    raw: rpc.endpoint.raw,
                    requires_auth: rpc.endpoint.requires_auth,
                },
                method: rpc.method.as_str().to_string(),
                path: rpc.path.clone(),
                path_and_query: rpc.path_and_query.clone(),
                path_params: rpc
                    .path_params
                    .as_ref()
                    .map(serde_json::to_value)
                    .transpose()?,
                parsed_payload: rpc
                    .parsed_payload
                    .as_ref()
                    .map(serde_json::to_value)
                    .transpose()?,
                headers: serialize_headers(&rpc.req_headers),
                middleware_data: req.middleware_data.lock()?.clone().map(PVals),
            };
            (Some(api), None)
        }

        model::RequestData::Stream(data) => {
            let api = APICallData {
                api: APIDesc {
                    service: data.endpoint.name.service().to_string(),
                    endpoint: data.endpoint.name.endpoint().to_string(),
                    raw: data.endpoint.raw,
                    requires_auth: data.endpoint.requires_auth,
                },
                method: Default::default(),
                path: data.path.clone(),
                path_and_query: data.path_and_query.clone(),
                path_params: data
                    .path_params
                    .as_ref()
                    .map(serde_json::to_value)
                    .transpose()?,
                parsed_payload: data
                    .parsed_payload
                    .as_ref()
                    .map(serde_json::to_value)
                    .transpose()?,
                headers: Default::default(),
                middleware_data: req.middleware_data.lock()?.clone().map(PVals),
            };
            (Some(api), None)
        }
        model::RequestData::PubSub(msg) => {
            let pubsub_message = PubSubMessageData {
                service: msg.service.to_string(),
                topic: msg.topic.to_string(),
                subscription: msg.subscription.to_string(),
                id: msg.message_id.clone(),
                published_at: msg.published.to_rfc3339_opts(SecondsFormat::Secs, true),
                delivery_attempt: msg.attempt,
                parsed_payload: msg.parsed_payload.as_ref().map(|pv| PVals(pv.clone())),
            };
            (None, Some(pubsub_message))
        }
        model::RequestData::Auth(_) => (None, None),
    };

    let trace = Some(TraceData {
        trace_id: req.span.0.serialize_encore(),
        span_id: req.span.1.serialize_encore(),
        parent_trace_id: req.parent_trace.map(|id| id.serialize_encore()),
        parent_span_id: req.parent_span.map(|id| id.1.serialize_encore()),
        ext_correlation_id: req.ext_correlation_id.clone(),
    });

    Ok(RequestMeta {
        started_at,
        trace,
        api_call,
        pubsub_message,
    })
}

#[napi(object)]
pub struct RequestMeta {
    pub started_at: String,
    pub trace: Option<TraceData>,
    pub api_call: Option<APICallData>,
    pub pubsub_message: Option<PubSubMessageData>,
}

#[napi(object)]
pub struct APICallData {
    pub api: APIDesc,
    pub method: String,
    pub path: String,
    pub path_and_query: String,
    pub path_params: Option<serde_json::Value>,
    pub parsed_payload: Option<serde_json::Value>,
    pub headers: serde_json::Map<String, serde_json::Value>,
    pub middleware_data: Option<PVals>,
}

#[napi(object)]
pub struct APIDesc {
    pub service: String,
    pub endpoint: String,
    pub raw: bool,
    pub requires_auth: bool,
}

#[napi(object)]
pub struct PubSubMessageData {
    pub service: String,
    pub topic: String,
    pub subscription: String,
    pub id: String,
    pub published_at: String,
    pub delivery_attempt: u32,
    pub parsed_payload: Option<PVals>,
}

#[napi(object)]
pub struct TraceData {
    pub trace_id: String,
    pub span_id: String,
    pub parent_trace_id: Option<String>,
    pub parent_span_id: Option<String>,
    pub ext_correlation_id: Option<String>,
}

fn serialize_headers(
    headers: &axum::http::HeaderMap,
) -> serde_json::Map<String, serde_json::Value> {
    use serde_json::{map::Entry, Map, Value};
    let mut map = Map::with_capacity(headers.len());

    for (k, v) in headers {
        let Ok(v) = v.to_str() else {
            continue;
        };

        // Skip Encore-internal headers.
        if v.starts_with("x-encore-meta") {
            continue;
        }

        let v = Value::String(v.to_string());

        // Insert the value as a string value if the entry does not yet exist.
        // If it does exist, convert it to an array and append the new value.
        match map.entry(k.as_str().to_string()) {
            Entry::Vacant(entry) => {
                entry.insert(v);
            }

            Entry::Occupied(entry) => {
                let arr = entry.into_mut();
                match arr {
                    Value::String(s) => {
                        let str = std::mem::replace(s, "".to_string());
                        *arr = Value::Array(vec![Value::String(str), v]);
                    }
                    Value::Array(arr) => {
                        arr.push(v);
                    }
                    _ => unreachable!(),
                }
            }
        }
    }

    map
}
