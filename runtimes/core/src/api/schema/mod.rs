use crate::api;
pub use body::*;
pub use header::*;
pub use method::*;
pub use path::*;
pub use query::*;
use std::sync::Arc;

use crate::api::{endpoint, APIResult, PValues, RequestPayload};

use super::ResponsePayload;

mod body;
pub mod encoding;
mod header;
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

impl Request {
    pub async fn extract(
        &self,
        parts: &mut axum::http::request::Parts,
        body: axum::body::Body,
    ) -> APIResult<Option<RequestPayload>> {
        let path = self.path.parse_incoming_request_parts(parts)?;
        let query = match &self.query {
            None => None,
            Some(q) => q.parse_incoming_request_parts(parts)?,
        };
        let header = match &self.header {
            None => None,
            Some(h) => h.parse_incoming_request_parts(parts)?,
        };

        let body = match &self.body {
            RequestBody::Raw => endpoint::Body::Raw(Arc::new(std::sync::Mutex::new(Some(body)))),
            RequestBody::Typed(None) => endpoint::Body::Typed(None),
            RequestBody::Typed(Some(b)) => {
                endpoint::Body::Typed(b.parse_incoming_request_body(body).await?)
            }
        };

        Ok(Some(RequestPayload {
            path,
            query,
            header,
            body,
        }))
    }
}

/// The response schema for an endpoint.
#[derive(Debug)]
pub struct Response {
    /// Response header names returned by the endpoint.
    pub header: Option<Header>,

    /// Response body, if any.
    pub body: Option<Body>,

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

        Ok(ResponsePayload { header, body })
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
