use chrono::{DateTime, SecondsFormat, Utc};
use napi_derive::napi;

use encore_runtime_core::{api::reqauth::meta::HeaderValueExt, model};

use crate::pvalue::PVals;

pub fn meta(req: &model::Request) -> Result<RequestMeta, serde_json::Error> {
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
                    tags: rpc.endpoint.tags.clone(),
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
                    tags: data.endpoint.tags.clone(),
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
}

#[napi(object)]
pub struct APIDesc {
    pub service: String,
    pub endpoint: String,
    pub raw: bool,
    pub requires_auth: bool,
    pub tags: Vec<String>,
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
        let Ok(v) = v.to_utf8_str() else {
            continue;
        };

        // Skip Encore-internal headers.
        if k.as_str().starts_with("x-encore-meta") {
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

#[cfg(test)]
mod tests {
    use super::serialize_headers;
    use axum::http::{HeaderMap, HeaderValue};
    use serde_json::{json, Value};

    #[test]
    fn strips_encore_internal_meta_headers() {
        let mut headers = HeaderMap::new();
        headers.insert("content-type", HeaderValue::from_static("application/json"));
        headers.insert("authorization", HeaderValue::from_static("Bearer tok"));
        headers.insert("x-encore-meta-userid", HeaderValue::from_static("admin"));
        headers.insert(
            "x-encore-meta-authdata",
            HeaderValue::from_static(r#"{"role":"admin"}"#),
        );
        headers.insert("x-encore-meta-svc-auth", HeaderValue::from_static("sig"));
        headers.insert(
            "x-encore-meta-caller",
            HeaderValue::from_static("api:svc.ep"),
        );

        let out = serialize_headers(&headers);

        // No internal identity/auth meta header may reach user code.
        assert!(
            out.keys().all(|k| !k.starts_with("x-encore-meta")),
            "internal meta header leaked into user-visible headers: {out:?}"
        );

        // Non-internal headers are preserved untouched.
        assert_eq!(
            out.get("content-type"),
            Some(&Value::String("application/json".into()))
        );
        assert_eq!(
            out.get("authorization"),
            Some(&Value::String("Bearer tok".into()))
        );
    }

    #[test]
    fn filters_on_header_name_not_value() {
        // Regression test for the value-vs-key bug: the filter must key off the
        // header NAME. A normal header whose *value* happens to start with
        // "x-encore-meta" must still be passed through to user code.
        let mut headers = HeaderMap::new();
        headers.insert("x-custom", HeaderValue::from_static("x-encore-meta-userid"));

        let out = serialize_headers(&headers);

        assert_eq!(
            out.get("x-custom"),
            Some(&Value::String("x-encore-meta-userid".into()))
        );
    }

    #[test]
    fn repeated_headers_become_an_array() {
        let mut headers = HeaderMap::new();
        headers.append("set-cookie", HeaderValue::from_static("a=1"));
        headers.append("set-cookie", HeaderValue::from_static("b=2"));

        let out = serialize_headers(&headers);

        assert_eq!(out.get("set-cookie"), Some(&json!(["a=1", "b=2"])));
    }
}
