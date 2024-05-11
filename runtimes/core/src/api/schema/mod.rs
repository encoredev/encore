use crate::api;
pub use body::*;
pub use header::*;
pub use method::*;
pub use path::*;
pub use query::*;
use std::sync::Arc;

use crate::api::{endpoint, APIResult, RequestPayload};

mod body;
pub mod encoding;
mod header;
mod method;
mod path;
mod query;

pub type JSONPayload = Option<serde_json::Map<String, serde_json::Value>>;

pub trait ToOutgoingRequest {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
    ) -> APIResult<()>;
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
}

impl Response {
    pub fn encode(
        &self,
        payload: &JSONPayload,
    ) -> APIResult<axum::http::Response<axum::body::Body>> {
        let mut bld = axum::http::Response::builder().status(200);

        match &self.header {
            Some(hdr) => bld = hdr.to_response(payload, bld)?,
            None => {}
        };
        match &self.body {
            Some(body) => body.to_response(payload, bld),
            None => bld
                .body(axum::body::Body::empty())
                .map_err(api::Error::internal),
        }
    }

    // pub fn decode(&self, mut resp: reqwest::Response) -> APIResult<JSONPayload> {
    //     self.header.to_response()
    // }
}
