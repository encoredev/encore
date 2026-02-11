use crate::api;
pub use body::*;
use bytes::Bytes;
pub use cookie::*;
pub use header::*;
use http_body_util::BodyExt;
pub use httpstatus::*;
use indexmap::IndexMap;
pub use method::*;
pub use path::*;
pub use query::*;
use std::sync::Arc;

use crate::api::{endpoint, APIResult, PValue, PValues, RequestPayload};

use super::ResponsePayload;

mod body;
mod cookie;
pub mod encoding;
mod header;
mod httpstatus;
mod method;
mod path;
mod query;

pub type JSONPayload = Option<PValues>;

pub trait ToOutgoingRequest<Request> {
    fn to_outgoing_request(&self, payload: &mut JSONPayload, req: &mut Request) -> APIResult<()>;
}

pub trait ToResponse {
    fn to_response(
        &self,
        payload: &JSONPayload,
        resp: axum::http::response::Builder,
    ) -> APIResult<axum::http::response::Builder>;
}

pub trait ParseResponse {
    type Output: Sized;
    fn parse_response(&self, resp: &mut reqwest::Response) -> APIResult<Self::Output>;
}

/// The request schema for a set of methods.
#[derive(Debug)]
pub struct Request {
    /// The methods the schema is applicable for.
    pub methods: Vec<Method>,

    /// Path to reach the endpoint.
    pub path: Path,

    /// Header names used by the endpoint.
    pub header: Option<Header>,

    /// Cookie names used by the endpoint.
    pub cookie: Option<Cookie>,

    /// Query string names used by the endpoint.
    pub query: Option<Query>,

    /// Request body.
    pub body: RequestBody,

    /// If this is a streamed request
    pub stream: bool,
}

#[derive(Debug)]
pub enum RequestBody {
    Typed(Option<Body>),
    Raw,
}

pub struct RequestExtractError {
    pub err: api::Error,
    pub partial_payload: Option<RequestPayload>,
    pub raw_body: Option<Bytes>,
}

async fn collect_body_bytes(body: axum::body::Body) -> APIResult<Bytes> {
    let bytes = body
        .collect()
        .await
        .map_err(|e| api::Error::invalid_argument("unable to read request body", e))?
        .to_bytes();
    Ok(bytes)
}

async fn raw_body_bytes_for_error(
    body: axum::body::Body,
    capture_raw_body: bool,
    raw_body_cap: Option<usize>,
) -> Option<Bytes> {
    if !capture_raw_body {
        return None;
    }

    if let Some(limit) = raw_body_cap {
        let body = http_body_util::Limited::new(body, limit as usize);
        body.collect().await.ok().map(|b| b.to_bytes())
    } else {
        body.collect().await.ok().map(|b| b.to_bytes())
    }
}

impl Request {
    pub async fn extract(
        &self,
        parts: &mut axum::http::request::Parts,
        body: axum::body::Body,
    ) -> APIResult<Option<RequestPayload>> {
        self.extract_with_partial(parts, body, false, None)
            .await
            .map_err(|err| err.err)
    }

    pub async fn extract_with_partial(
        &self,
        parts: &mut axum::http::request::Parts,
        body: axum::body::Body,
        capture_raw_body: bool,
        raw_body_cap: Option<usize>,
    ) -> Result<Option<RequestPayload>, RequestExtractError> {
        let mut path: Option<IndexMap<String, PValue>> = None;
        let mut query: Option<PValues> = None;
        let mut header: Option<PValues> = None;
        let mut cookie: Option<PValues> = None;

        path = match self.path.parse_incoming_request_parts(parts) {
            Ok(val) => val,
            Err(err) => {
                let raw_body = raw_body_bytes_for_error(body, capture_raw_body, raw_body_cap).await;
                let partial_payload = Some(RequestPayload {
                    path,
                    query,
                    header,
                    cookie,
                    body: endpoint::Body::Typed(None),
                });
                return Err(RequestExtractError {
                    err,
                    partial_payload,
                    raw_body,
                });
            }
        };

        query = match &self.query {
            None => None,
            Some(q) => match q.parse_incoming_request_parts(parts) {
                Ok(val) => val,
                Err(err) => {
                    let raw_body =
                        raw_body_bytes_for_error(body, capture_raw_body, raw_body_cap).await;
                    let partial_payload = Some(RequestPayload {
                        path,
                        query,
                        header,
                        cookie,
                        body: endpoint::Body::Typed(None),
                    });
                    return Err(RequestExtractError {
                        err,
                        partial_payload,
                        raw_body,
                    });
                }
            },
        };

        header = match &self.header {
            None => None,
            Some(h) => match h.parse_incoming_request_parts(parts) {
                Ok(val) => val,
                Err(err) => {
                    let raw_body =
                        raw_body_bytes_for_error(body, capture_raw_body, raw_body_cap).await;
                    let partial_payload = Some(RequestPayload {
                        path,
                        query,
                        header,
                        cookie,
                        body: endpoint::Body::Typed(None),
                    });
                    return Err(RequestExtractError {
                        err,
                        partial_payload,
                        raw_body,
                    });
                }
            },
        };

        cookie = match &self.cookie {
            None => None,
            Some(c) => match c.parse_incoming_request_parts(parts) {
                Ok(val) => val,
                Err(err) => {
                    let raw_body =
                        raw_body_bytes_for_error(body, capture_raw_body, raw_body_cap).await;
                    let partial_payload = Some(RequestPayload {
                        path,
                        query,
                        header,
                        cookie,
                        body: endpoint::Body::Typed(None),
                    });
                    return Err(RequestExtractError {
                        err,
                        partial_payload,
                        raw_body,
                    });
                }
            },
        };

        let body = match &self.body {
            RequestBody::Raw => endpoint::Body::Raw(Arc::new(std::sync::Mutex::new(Some(body)))),
            RequestBody::Typed(None) => endpoint::Body::Typed(None),
            RequestBody::Typed(Some(b)) => {
                let bytes = match collect_body_bytes(body).await {
                    Ok(val) => val,
                    Err(err) => {
                        let partial_payload = Some(RequestPayload {
                            path,
                            query,
                            header,
                            cookie,
                            body: endpoint::Body::Typed(None),
                        });
                        return Err(RequestExtractError {
                            err,
                            partial_payload,
                            raw_body: None,
                        });
                    }
                };

                match b.parse_incoming_request_body_bytes(bytes.clone()) {
                    Ok(parsed) => endpoint::Body::Typed(parsed),
                    Err(err) => {
                        let raw_body = if capture_raw_body {
                            match raw_body_cap {
                                Some(cap) if bytes.len() > cap => Some(bytes.slice(0..cap)),
                                _ => Some(bytes),
                            }
                        } else {
                            None
                        };
                        let partial_payload = Some(RequestPayload {
                            path,
                            query,
                            header,
                            cookie,
                            body: endpoint::Body::Typed(None),
                        });
                        return Err(RequestExtractError {
                            err,
                            partial_payload,
                            raw_body,
                        });
                    }
                }
            }
        };

        Ok(Some(RequestPayload {
            path,
            query,
            header,
            cookie,
            body,
        }))
    }
}

/// The response schema for an endpoint.
#[derive(Debug)]
pub struct Response {
    /// Response header names returned by the endpoint.
    pub header: Option<Header>,

    /// Response cookie names returned by the endpoint.
    pub cookie: Option<Cookie>,

    /// Response body, if any.
    pub body: Option<Body>,

    /// HTTP status code field, if any.
    pub http_status: Option<HttpStatus>,

    /// If this is a streamed response
    pub stream: bool,
}

impl Response {
    pub fn encode(
        &self,
        payload: &JSONPayload,
        status: u16,
    ) -> APIResult<axum::http::Response<axum::body::Body>> {
        let mut bld = axum::http::Response::builder().status(status);

        if let Some(hdr) = &self.header {
            bld = hdr.to_response(payload, bld)?
        };
        if let Some(c) = &self.cookie {
            bld = c.to_response(payload, bld)?;
        }
        if let Some(hs) = &self.http_status {
            bld = hs.to_response(payload, bld)?;
        }
        match &self.body {
            Some(body) => body.to_response(payload, bld),
            None => bld
                .body(axum::body::Body::empty())
                .map_err(api::Error::internal),
        }
    }

    pub async fn extract(&self, resp: reqwest::Response) -> APIResult<ResponsePayload> {
        let header = match &self.header {
            None => None,
            Some(h) => h.parse(resp.headers())?,
        };

        let cookie = match &self.cookie {
            None => None,
            Some(c) => c.parse_resp(resp.headers())?,
        };

        // Do we have a body schema?
        let body = endpoint::Body::Typed(match &self.body {
            None => None,
            Some(body_schema) => {
                // If so we expect a JSON body.
                match resp.headers().get(axum::http::header::CONTENT_TYPE) {
                    Some(content_type) if content_type == "application/json" => {
                        // Collect the bytes of the request body.
                        // Serde doesn't support async streaming reads (at least not yet).
                        let bytes = resp.bytes().await.map_err(|e| {
                            api::Error::invalid_argument("unable to read response body", e)
                        })?;

                        body_schema.parse_response_body(bytes).await?
                    }
                    _ => {
                        // We didn't get a JSON body, so return an error.
                        return Err(api::Error::internal(anyhow::anyhow!("expected json body")));
                    }
                }
            }
        });

        Ok(ResponsePayload {
            header,
            body,
            cookie,
        })
    }
}

pub struct Stream {
    incoming: Box<dyn StreamMessage>,
    outgoing: Box<dyn StreamMessage>,
}

impl Stream {
    pub fn new<I, O>(incoming: I, outgoing: O) -> Self
    where
        I: StreamMessage,
        O: StreamMessage,
    {
        Stream {
            incoming: Box::new(incoming),
            outgoing: Box::new(outgoing),
        }
    }

    pub fn to_outgoing_message(&self, msg: PValues) -> APIResult<Vec<u8>> {
        let body_schema = self.outgoing.body().ok_or_else(|| {
            super::Error::internal(anyhow::anyhow!("outgoing message body can't be empty"))
        })?;

        body_schema.to_outgoing_payload(&Some(msg))
    }

    pub async fn parse_incoming_message(&self, bytes: &[u8]) -> APIResult<PValues> {
        let Some(body) = self.incoming.body() else {
            return Err(super::Error {
                code: super::ErrCode::InvalidArgument,
                message: "invalid streaming body type in schema".to_string(),
                internal_message: None,
                stack: None,
                details: None,
            });
        };

        let value = body
            .parse_incoming_request_body(bytes.to_vec().into())
            .await?
            .ok_or_else(|| super::Error {
                code: super::ErrCode::InvalidArgument,
                message: "missing payload".to_string(),
                internal_message: None,
                stack: None,
                details: None,
            })?;

        Ok(value)
    }
}

pub trait StreamMessage: Send + Sync + 'static {
    fn body(&self) -> Option<&Body>;
}

impl StreamMessage for Arc<Response> {
    fn body(&self) -> Option<&Body> {
        self.body.as_ref()
    }
}

impl StreamMessage for Arc<Request> {
    fn body(&self) -> Option<&Body> {
        if let RequestBody::Typed(body) = &self.body {
            body.as_ref()
        } else {
            None
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::jsonschema;
    use crate::api::jsonschema::{Basic, BasicOrValue, Field, Struct, Value};
    use crate::encore::parser::meta::v1 as meta;
    use axum::body::Body as AxumBody;
    use http::header::CONTENT_TYPE;
    use std::collections::HashMap;

    fn schema_with_number_field(name: &str) -> jsonschema::JSONSchema {
        let md = meta::Data::default();
        let mut builder = jsonschema::Builder::new(&md);
        let mut fields = HashMap::new();
        fields.insert(
            name.to_string(),
            Field {
                value: BasicOrValue::Basic(Basic::Number),
                optional: false,
                name_override: None,
            },
        );
        let idx = builder.register_value(Value::Struct(Struct { fields }));
        let registry = builder.build();
        registry.schema(idx)
    }

    fn simple_path() -> Path {
        Path::from_segments(vec![Segment::Literal("foo".into())])
    }

    #[tokio::test]
    async fn extract_with_partial_includes_raw_body_for_typed() {
        let body_schema = Body::new(jsonschema::JSONSchema::any());
        let req_schema = Request {
            methods: vec![Method::POST],
            path: simple_path(),
            header: None,
            cookie: None,
            query: None,
            body: RequestBody::Typed(Some(body_schema)),
            stream: false,
        };

        let raw = "{bad json";
        let req = axum::http::Request::builder()
            .method("POST")
            .uri("/foo")
            .header(CONTENT_TYPE, "application/json")
            .body(AxumBody::from(raw))
            .unwrap();

        let (mut parts, body) = req.into_parts();
        let err = req_schema
            .extract_with_partial(&mut parts, body, true, None)
            .await
            .unwrap_err();
        let payload = err.partial_payload.expect("partial payload");
        assert!(matches!(payload.body, endpoint::Body::Typed(None)));
        let bytes = err.raw_body.expect("raw body");
        assert_eq!(bytes.len(), raw.as_bytes().len());
        assert_eq!(bytes[0], raw.as_bytes()[0]);
    }

    #[tokio::test]
    async fn extract_with_partial_caps_raw_body_for_raw_endpoint() {
        let query_schema = Query::new(schema_with_number_field("foo"));
        let req_schema = Request {
            methods: vec![Method::GET],
            path: simple_path(),
            header: None,
            cookie: None,
            query: Some(query_schema),
            body: RequestBody::Raw,
            stream: false,
        };

        let raw_cap = 10 * 1024;
        let raw_body = vec![b'a'; raw_cap + 32];
        let req = axum::http::Request::builder()
            .method("GET")
            .uri("/foo?foo=abc")
            .header(CONTENT_TYPE, "text/plain")
            .body(AxumBody::from(raw_body))
            .unwrap();

        let (mut parts, body) = req.into_parts();
        let err = req_schema
            .extract_with_partial(&mut parts, body, true, Some(raw_cap))
            .await
            .unwrap_err();
        let payload = err.partial_payload.expect("partial payload");
        assert!(matches!(payload.body, endpoint::Body::Typed(None)));
        let bytes = err.raw_body.expect("raw body");
        assert_eq!(bytes.len(), raw_cap);
    }

    #[tokio::test]
    async fn extract_with_partial_skips_raw_body_when_disabled() {
        let body_schema = Body::new(jsonschema::JSONSchema::any());
        let req_schema = Request {
            methods: vec![Method::POST],
            path: simple_path(),
            header: None,
            cookie: None,
            query: None,
            body: RequestBody::Typed(Some(body_schema)),
            stream: false,
        };

        let req = axum::http::Request::builder()
            .method("POST")
            .uri("/foo")
            .header(CONTENT_TYPE, "application/octet-stream")
            .body(AxumBody::from("{bad json"))
            .unwrap();

        let (mut parts, body) = req.into_parts();
        let err = req_schema
            .extract_with_partial(&mut parts, body, false, None)
            .await
            .unwrap_err();
        let payload = err.partial_payload.expect("partial payload");
        match payload.body {
            endpoint::Body::Typed(None) => {}
            _ => panic!("unexpected body type"),
        }
        assert!(err.raw_body.is_none());
    }
}
