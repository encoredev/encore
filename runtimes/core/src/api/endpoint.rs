use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use anyhow::Context;
use axum::extract::{FromRequestParts, WebSocketUpgrade};
use axum::http::HeaderValue;
use axum::response::IntoResponse;
use bytes::{BufMut, BytesMut};
use http::HeaderMap;
use indexmap::IndexMap;
use serde::Serialize;

use crate::api::reqauth::{platform, svcauth, CallMeta};
use crate::api::schema::encoding::{
    handshake_encoding, request_encoding, response_encoding, HandshakeSchemaUnderConstruction,
    ReqSchemaUnderConstruction, SchemaUnderConstruction,
};
use crate::api::schema::{JSONPayload, Method};
use crate::api::{jsonschema, schema, ErrCode, Error};
use crate::encore::parser::meta::v1::rpc;
use crate::encore::parser::meta::v1::{self as meta, selector};
use crate::log::LogFromRust;
use crate::model::StreamDirection;
use crate::names::EndpointName;
use crate::trace;
use crate::{model, Hosted};

use super::pvalue::{PValue, PValues};
use super::reqauth::caller::Caller;

#[derive(Debug)]
pub struct SuccessResponse {
    pub status: axum::http::StatusCode,
    pub headers: axum::http::HeaderMap,
    pub body: Option<PValue>,
}

/// Represents the result of calling an API endpoint.
pub type Response = APIResult<SuccessResponse>;

pub type APIResult<T> = Result<T, Error>;

impl IntoResponse for SuccessResponse {
    fn into_response(self) -> axum::http::Response<axum::body::Body> {
        // Serialize the response body.
        // Use a small initial capacity of 128 bytes like serde_json::to_vec
        // https://docs.rs/serde_json/1.0.82/src/serde_json/ser.rs.html#2189
        let bld = {
            let mut bld = axum::http::Response::builder();
            *(bld.headers_mut().unwrap()) = self.headers;
            bld
        }
        .status(self.status);

        match self.body {
            Some(body) => {
                let bld = bld.header(
                    axum::http::header::CONTENT_TYPE,
                    HeaderValue::from_static(mime::APPLICATION_JSON.as_ref()),
                );
                let mut buf = BytesMut::with_capacity(128).writer();
                match serde_json::to_writer(&mut buf, &body) {
                    Ok(()) => bld
                        .body(axum::body::Body::from(buf.into_inner().freeze()))
                        .unwrap(),
                    Err(err) => Error::internal(err).to_response(None),
                }
            }
            None => bld.body(axum::body::Body::empty()).unwrap(),
        }
    }
}

pub trait ToResponse {
    fn to_response(&self, caller: Option<Caller>) -> axum::response::Response;
}

impl ToResponse for Error {
    fn to_response(&self, caller: Option<Caller>) -> axum::http::Response<axum::body::Body> {
        // considure response to be external if caller is gateway, or if the caller is
        // unknown
        let internal_call = caller.map(|caller| !caller.is_gateway()).unwrap_or(false);

        let mut buf = BytesMut::with_capacity(128).writer();

        if internal_call {
            serde_json::to_writer(&mut buf, &self).unwrap();
        } else {
            serde_json::to_writer(&mut buf, &self.as_external()).unwrap();
        }

        axum::http::Response::builder()
            .status::<axum::http::status::StatusCode>(self.code.into())
            .header(
                axum::http::header::CONTENT_TYPE,
                HeaderValue::from_static(mime::APPLICATION_JSON.as_ref()),
            )
            .body(axum::body::Body::from(buf.into_inner().freeze()))
            .unwrap()
    }
}

pub type HandlerRequest = Arc<model::Request>;
pub type HandlerResponse = APIResult<HandlerResponseInner>;

pub struct HandlerResponseInner {
    pub payload: JSONPayload,
    pub extra_headers: Option<HeaderMap>,
    pub status: Option<u16>,
}

/// A trait for handlers that accept a request and return a response.
pub trait TypedHandler: Send + Sync + 'static {
    fn call(
        self: Arc<Self>,
        req: HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = HandlerResponse> + Send + 'static>>;
}

/// A trait for handlers that accept a request and return a response.
pub trait BoxedHandler: Send + Sync + 'static {
    fn call(
        self: Arc<Self>,
        req: HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = ResponseData> + Send + 'static>>;
}

pub enum ResponseData {
    Typed(HandlerResponse),
    Raw(axum::http::Response<axum::body::Body>),
}

/// Schema variations for stream handshake
#[derive(Debug)]
pub enum HandshakeSchema {
    // Handshake with only a path, no parseable data
    Path(schema::Path),
    // Handshake with a request schema
    Request(schema::Request),
}

impl HandshakeSchema {
    pub fn path(&self) -> &schema::Path {
        match self {
            HandshakeSchema::Path(path) => path,
            HandshakeSchema::Request(schema::Request { path, .. }) => path,
        }
    }
}

/// Represents a single API Endpoint.
#[derive(Debug)]
pub struct Endpoint {
    pub name: EndpointName,
    pub path: meta::Path,
    pub handshake: Option<Arc<HandshakeSchema>>,
    pub request: Vec<Arc<schema::Request>>,
    pub response: Arc<schema::Response>,

    /// Whether this is a raw endpoint.
    pub raw: bool,

    /// Whether the service is exposed publicly.
    pub exposed: bool,

    /// Whether the service requires authentication data.
    pub requires_auth: bool,

    /// The maximum size of the request body.
    /// If None, no limits are applied.
    pub body_limit: Option<u64>,

    /// The static assets to serve from this endpoint.
    /// Set only for static asset endpoints.
    pub static_assets: Option<meta::rpc::StaticAssets>,

    /// The tags for this endpoint.
    pub tags: Vec<String>,
}

impl Endpoint {
    pub fn methods(&self) -> impl Iterator<Item = Method> + '_ {
        self.request
            .iter()
            .flat_map(|schema| schema.methods.iter().copied())
    }
}

#[derive(Debug, Serialize, Clone)]
pub struct RequestPayload {
    #[serde(flatten)]
    pub path: Option<IndexMap<String, PValue>>,

    #[serde(flatten)]
    pub query: Option<PValues>,

    #[serde(flatten)]
    pub header: Option<PValues>,

    #[serde(flatten, skip_serializing_if = "Body::is_raw")]
    pub body: Body,
}

#[derive(Debug, Serialize, Clone)]
#[serde(untagged)]
pub enum Body {
    Typed(Option<PValues>),
    #[serde(skip)]
    Raw(Arc<std::sync::Mutex<Option<axum::body::Body>>>),
}

impl Body {
    pub fn is_raw(&self) -> bool {
        matches!(self, Body::Raw(_))
    }
}

#[derive(Debug, Serialize, Clone)]
pub struct ResponsePayload {
    #[serde(flatten)]
    pub header: Option<PValues>,

    #[serde(flatten, skip_serializing_if = "Body::is_raw")]
    pub body: Body,
}

pub type EndpointMap = HashMap<EndpointName, Arc<Endpoint>>;

/// Compute a set of endpoint descriptions based on metadata
/// and a list of which endpoints are hosted by this runtime.
pub fn endpoints_from_meta(
    md: &meta::Data,
    hosted_services: &Hosted,
) -> anyhow::Result<(Arc<EndpointMap>, Vec<EndpointName>)> {
    let mut registry_builder = jsonschema::Builder::new(md);

    struct EndpointUnderConstruction<'a> {
        svc: &'a meta::Service,
        ep: &'a meta::Rpc,
        handshake_schema: Option<HandshakeSchemaUnderConstruction>,
        request_schemas: Vec<ReqSchemaUnderConstruction>,
        response_schema: SchemaUnderConstruction,
    }

    // Compute the schemas for each endpoint.
    let mut endpoints = Vec::new();
    let mut hosted_endpoints = Vec::new();
    for svc in &md.svcs {
        let is_hosted = hosted_services.contains(&svc.name);
        for ep in &svc.rpcs {
            // If this endpoint is hosted, mark it as such.
            if is_hosted {
                hosted_endpoints.push(EndpointName::new(&svc.name, &ep.name));
            }

            let handshake_schema = handshake_encoding(&mut registry_builder, md, ep)?;
            let request_schemas = request_encoding(&mut registry_builder, md, ep)?;
            let response_schema = response_encoding(&mut registry_builder, md, ep)?;

            endpoints.push(EndpointUnderConstruction {
                svc,
                ep,
                handshake_schema,
                request_schemas,
                response_schema,
            });
        }
    }

    let registry = registry_builder.build();

    let mut endpoint_map = EndpointMap::with_capacity(endpoints.len());

    for ep in endpoints {
        let mut request_schemas = Vec::with_capacity(ep.request_schemas.len());
        let raw = rpc::Protocol::try_from(ep.ep.proto).is_ok_and(|p| p == rpc::Protocol::Raw);

        let handshake_schema = ep
            .handshake_schema
            .map(|schema| schema.build(&registry))
            .transpose()?;

        let handshake = handshake_schema
            .map(|handshake_schema| -> anyhow::Result<Arc<HandshakeSchema>> {
                let path = handshake_schema
                    .schema
                    .path
                    .context("endpoint must have a path defined")?;

                let handshake_schema = if handshake_schema.parse_data {
                    let handshake_schema = schema::Request {
                        methods: vec![],
                        path,
                        header: handshake_schema.schema.header,
                        query: handshake_schema.schema.query,
                        body: schema::RequestBody::Typed(None),
                        stream: false,
                    };

                    HandshakeSchema::Request(handshake_schema)
                } else {
                    HandshakeSchema::Path(path)
                };

                Ok(Arc::new(handshake_schema))
            })
            .transpose()?;

        for req_schema in ep.request_schemas {
            let req_schema = req_schema.build(&registry)?;
            let path = req_schema
                .schema
                .path
                .or_else(|| handshake.as_ref().map(|hs| hs.path().clone()))
                .context("endpoint must have path defined")?;

            request_schemas.push(Arc::new(schema::Request {
                methods: req_schema.methods,
                path,
                header: req_schema.schema.header,
                query: req_schema.schema.query,
                body: if raw {
                    schema::RequestBody::Raw
                } else {
                    schema::RequestBody::Typed(req_schema.schema.body)
                },
                stream: ep.ep.streaming_request,
            }));
        }
        let resp_schema = ep.response_schema.build(&registry)?;

        // We only support a single gateway right now.
        let exposed = ep.ep.expose.contains_key("api-gateway");
        let raw =
            rpc::Protocol::try_from(ep.ep.proto).is_ok_and(|proto| proto == rpc::Protocol::Raw);

        let tags = ep
            .ep
            .tags
            .iter()
            .filter(|item| item.r#type() == selector::Type::Tag)
            .map(|item| item.value.clone())
            .collect();

        let endpoint = Endpoint {
            name: EndpointName::new(ep.svc.name.clone(), ep.ep.name.clone()),
            path: ep.ep.path.clone().unwrap_or_else(|| meta::Path {
                r#type: meta::path::Type::Url as i32,
                segments: vec![meta::PathSegment {
                    r#type: meta::path_segment::SegmentType::Literal as i32,
                    value_type: meta::path_segment::ParamType::String as i32,
                    value: format!("/{}.{}", ep.ep.service_name, ep.ep.name),
                    validation: None,
                }],
            }),
            handshake,
            request: request_schemas,
            response: Arc::new(schema::Response {
                header: resp_schema.header,
                body: resp_schema.body,
                stream: ep.ep.streaming_response,
            }),
            raw,
            exposed,
            requires_auth: !ep.ep.allow_unauthenticated,
            body_limit: ep.ep.body_limit,
            static_assets: ep.ep.static_assets.clone(),
            tags,
        };

        endpoint_map.insert(
            EndpointName::new(&ep.svc.name, &ep.ep.name),
            Arc::new(endpoint),
        );
    }

    Ok((Arc::new(endpoint_map), hosted_endpoints))
}

pub(super) struct EndpointHandler {
    pub endpoint: Arc<Endpoint>,
    pub handler: Arc<dyn BoxedHandler>,
    pub shared: Arc<SharedEndpointData>,
}

#[derive(Debug)]
pub(super) struct SharedEndpointData {
    pub tracer: trace::Tracer,
    pub platform_auth: Arc<platform::RequestValidator>,
    pub inbound_svc_auth: Vec<Arc<dyn svcauth::ServiceAuthMethod>>,

    /// The schema to use when parsing auth data, if any.
    /// NOTE: This assumes there's at most a single API Gateway.
    /// When we support multiple this needs to be made into a map, and the
    /// correct schema looked up based on the gateway being used.
    pub auth_data_schemas: HashMap<String, Option<jsonschema::JSONSchema>>,
}

impl Clone for EndpointHandler {
    fn clone(&self) -> Self {
        Self {
            endpoint: self.endpoint.clone(),
            handler: self.handler.clone(),
            shared: self.shared.clone(),
        }
    }
}

impl EndpointHandler {
    async fn parse_request(
        &self,
        axum_req: axum::extract::Request,
    ) -> APIResult<Arc<model::Request>> {
        let method = axum_req.method();
        // Method conversion should never fail since we only register valid methods.
        let api_method = Method::try_from(method.clone()).expect("invalid method");

        let req_schema = self
            .endpoint
            .request
            .iter()
            .find(|schema| schema.methods.contains(&api_method))
            .expect("request schema must exist for all endpoints");

        let streaming_request = req_schema.stream;
        let streaming_response = self.endpoint.response.stream;

        let stream_direction = match (streaming_request, streaming_response) {
            (true, true) => Some(StreamDirection::InOut),
            (true, false) => Some(StreamDirection::In),
            (false, true) => Some(StreamDirection::Out),
            (false, false) => None,
        };

        let (mut parts, body) = axum_req
            .map(|b| match self.endpoint.body_limit {
                None => b,
                Some(limit) => {
                    axum::body::Body::new(http_body_util::Limited::new(b, limit as usize))
                }
            })
            .into_parts();

        // Authenticate the request from the platform, if applicable.
        #[allow(clippy::manual_unwrap_or_default)]
        let platform_seal_of_approval = match self.authenticate_platform(&parts) {
            Ok(seal) => seal,
            Err(_err) => None,
        };

        let meta = CallMeta::parse_with_caller(
            &self.shared.inbound_svc_auth,
            &parts.headers,
            &self.shared.auth_data_schemas,
        )?;

        let parsed_payload = if let Some(handshake_schema) = &self.endpoint.handshake {
            match handshake_schema.as_ref() {
                HandshakeSchema::Request(req_schema) => {
                    req_schema.extract(&mut parts, body).await?
                }
                HandshakeSchema::Path(_) => None,
            }
        } else {
            req_schema.extract(&mut parts, body).await?
        };

        // Extract caller information.
        let (internal_caller, auth_user_id, auth_data) = match meta.internal {
            Some(internal) => (Some(internal.caller), internal.auth_uid, internal.auth_data),
            None => (None, None, None),
        };

        let trace_id = meta.trace_id;
        let span_id = meta.this_span_id.unwrap_or_else(model::SpanId::generate);
        let span = trace_id.with_span(span_id);
        let parent_span = meta.parent_span_id.map(|sp| trace_id.with_span(sp));

        let data = if let Some(direction) = stream_direction {
            let websocket_upgrade = Mutex::new(Some(
                WebSocketUpgrade::from_request_parts(&mut parts, &()).await?,
            ));

            model::RequestData::Stream(model::StreamRequestData {
                endpoint: self.endpoint.clone(),
                path: parts.uri.path().to_string(),
                path_and_query: parts
                    .uri
                    .path_and_query()
                    .map(|q| q.to_string())
                    .unwrap_or_default(),
                path_params: parsed_payload.as_ref().and_then(|p| p.path.clone()),
                req_headers: parts.headers,
                auth_user_id,
                auth_data,
                parsed_payload,
                websocket_upgrade,
                direction,
            })
        } else {
            model::RequestData::RPC(model::RPCRequestData {
                endpoint: self.endpoint.clone(),
                method: api_method,
                path: parts.uri.path().to_string(),
                path_and_query: parts
                    .uri
                    .path_and_query()
                    .map(|q| q.to_string())
                    .unwrap_or_default(),
                path_params: parsed_payload.as_ref().and_then(|p| p.path.clone()),
                req_headers: parts.headers,
                auth_user_id,
                auth_data,
                parsed_payload,
            })
        };

        let request = Arc::new(model::Request {
            span,
            parent_trace: None,
            parent_span,
            caller_event_id: meta.parent_event_id,
            ext_correlation_id: meta.ext_correlation_id,
            start: tokio::time::Instant::now(),
            start_time: std::time::SystemTime::now(),
            is_platform_request: platform_seal_of_approval.is_some(),
            internal_caller,
            data,
        });

        Ok(request)
    }

    fn handle(
        self,
        axum_req: axum::extract::Request,
    ) -> Pin<Box<dyn Future<Output = axum::http::Response<axum::body::Body>> + Send + 'static>>
    {
        Box::pin(async move {
            let request = match self.parse_request(axum_req).await {
                Ok(req) => req,
                Err(err) => return err.to_response(None),
            };

            let internal_caller = request.internal_caller.clone();

            // If the endpoint isn't exposed, return a 404.
            if !self.endpoint.exposed && !request.allows_private_endpoint_call() {
                return Error {
                    code: ErrCode::NotFound,
                    message: "endpoint not found".into(),
                    internal_message: Some("the endpoint was found, but is not exposed".into()),
                    stack: None,
                    details: None,
                }
                .to_response(internal_caller);
            } else if self.endpoint.requires_auth && !request.has_authenticated_user() {
                return Error {
                    code: ErrCode::Unauthenticated,
                    message: "endpoint requires auth but none provided".into(),
                    internal_message: None,
                    stack: None,
                    details: None,
                }
                .to_response(internal_caller);
            }

            let logger = crate::log::root();
            logger.info(Some(&request), "starting request", None);

            self.shared.tracer.request_span_start(&request);

            let resp: ResponseData = self.handler.call(request.clone()).await;

            let duration = tokio::time::Instant::now().duration_since(request.start);

            // If we had a request failure, log that separately.

            if let ResponseData::Typed(Err(err)) = &resp {
                logger.error(Some(&request), "request failed", Some(err), {
                    let mut fields = crate::log::Fields::new();
                    fields.insert(
                        "code".into(),
                        serde_json::Value::String(err.code.to_string()),
                    );
                    Some(fields)
                });
            }

            logger.info(Some(&request), "request completed", {
                let mut fields = crate::log::Fields::new();
                let dur_ms = (duration.as_secs() as f64 * 1000f64)
                    + (duration.subsec_nanos() as f64 / 1_000_000f64);

                fields.insert(
                    "duration".into(),
                    serde_json::Value::Number(serde_json::Number::from_f64(dur_ms).unwrap_or_else(
                        || {
                            // Fall back to integer if the f64 conversion fails
                            serde_json::Number::from(duration.as_millis() as u64)
                        },
                    )),
                );

                let code = match &resp {
                    ResponseData::Typed(Ok(_)) => "ok".to_string(),
                    ResponseData::Typed(Err(err)) => err.code.to_string(),
                    ResponseData::Raw(resp) => ErrCode::from(resp.status()).to_string(),
                };

                fields.insert("code".into(), serde_json::Value::String(code));
                Some(fields)
            });

            let (mut encoded_resp, resp_payload, extra_headers, error) = match resp {
                ResponseData::Raw(resp) => (resp, None, None, None),
                ResponseData::Typed(Ok(response)) => (
                    self.endpoint
                        .response
                        .encode(&response.payload, response.status.unwrap_or(200))
                        .unwrap_or_else(|err| err.to_response(internal_caller)),
                    Some(response.payload),
                    response.extra_headers,
                    None,
                ),
                ResponseData::Typed(Err(err)) => (
                    err.as_ref().to_response(internal_caller),
                    None,
                    None,
                    Some(err),
                ),
            };

            {
                let model_resp = model::Response {
                    request: request.clone(),
                    duration,
                    data: model::ResponseData::RPC(model::RPCResponseData {
                        status_code: encoded_resp.status().as_u16(),
                        resp_payload,
                        error,
                        resp_headers: encoded_resp.headers().clone(),
                    }),
                };
                self.shared.tracer.request_span_end(&model_resp);
            }

            if let Ok(val) = HeaderValue::from_str(request.span.0.serialize_encore().as_str()) {
                encoded_resp.headers_mut().insert("x-encore-trace-id", val);
            }

            if let Some(extra_headers) = extra_headers {
                encoded_resp.headers_mut().extend(extra_headers)
            }

            encoded_resp
        })
    }

    fn authenticate_platform(
        &self,
        req: &axum::http::request::Parts,
    ) -> Result<Option<platform::SealOfApproval>, platform::ValidationError> {
        let Some(x_encore_auth_header) = req.headers.get("x-encore-auth") else {
            return Ok(None);
        };
        let x_encore_auth_header = x_encore_auth_header
            .to_str()
            .map_err(|_| platform::ValidationError::InvalidMac)?;

        let Some(date_header) = req.headers.get("Date") else {
            return Err(platform::ValidationError::InvalidDateHeader);
        };
        let date_header = date_header
            .to_str()
            .map_err(|_| platform::ValidationError::InvalidDateHeader)?;

        let request_path = req.uri.path();
        let req = platform::ValidationData {
            request_path,
            date_header,
            x_encore_auth_header,
        };

        self.shared
            .platform_auth
            .validate_platform_request(&req)
            .map(Some)
    }
}

impl axum::handler::Handler<(), ()> for EndpointHandler {
    type Future =
        Pin<Box<dyn Future<Output = axum::http::Response<axum::body::Body>> + Send + 'static>>;

    fn call(self, axum_req: axum::extract::Request, _state: ()) -> Self::Future {
        self.handle(axum_req)
    }
}

pub fn path_supports_tsr(path: &str) -> bool {
    path != "/" && !path.ends_with('/') && !path.contains("/*")
}
