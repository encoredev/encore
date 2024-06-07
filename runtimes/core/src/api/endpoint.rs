use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::Context;
use axum::http::HeaderValue;
use axum::RequestExt;
use bytes::{BufMut, BytesMut};
use indexmap::IndexMap;
use serde::Serialize;

use crate::api::reqauth::{platform, svcauth, CallMeta};
use crate::api::schema::encoding::{
    request_encoding, response_encoding, ReqSchemaUnderConstruction, SchemaUnderConstruction,
};
use crate::api::schema::{JSONPayload, Method};
use crate::api::{jsonschema, schema, ErrCode, Error};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::parser::meta::v1::rpc;
use crate::log::LogFromRust;
use crate::names::EndpointName;
use crate::trace;
use crate::{model, Hosted};

#[derive(Debug)]
pub struct SuccessResponse {
    pub status: axum::http::StatusCode,
    pub headers: axum::http::HeaderMap,
    pub body: Option<serde_json::Value>,
}

/// Represents the result of calling an API endpoint.
pub type Response = APIResult<SuccessResponse>;

pub type APIResult<T> = Result<T, Error>;

pub trait IntoResponse {
    fn into_response(self) -> axum::http::Response<axum::body::Body>;
}

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
                    Err(err) => Error::internal(err).into_response(),
                }
            }
            None => bld.body(axum::body::Body::empty()).unwrap(),
        }
    }
}

impl IntoResponse for &Error {
    fn into_response(self) -> axum::http::Response<axum::body::Body> {
        let mut buf = BytesMut::with_capacity(128).writer();
        serde_json::to_writer(&mut buf, &self).unwrap();

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

impl<A, B> IntoResponse for Result<A, B>
where
    A: IntoResponse,
    B: IntoResponse,
{
    fn into_response(self) -> axum::http::Response<axum::body::Body> {
        match self {
            Ok(a) => a.into_response(),
            Err(b) => b.into_response(),
        }
    }
}

pub type HandlerRequest = Arc<model::Request>;
pub type HandlerResponse = APIResult<JSONPayload>;

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
    Typed(APIResult<JSONPayload>),
    Raw(axum::http::Response<axum::body::Body>),
}

/// Represents a single API Endpoint.
#[derive(Debug)]
pub struct Endpoint {
    pub name: EndpointName,
    pub path: meta::Path,
    pub raw: bool,
    pub request: Vec<Arc<schema::Request>>,
    pub response: Arc<schema::Response>,

    /// Whether the service is exposed publicly.
    pub exposed: bool,

    /// Whether the service requires authentication data.
    pub requires_auth: bool,
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
    pub path: Option<IndexMap<String, serde_json::Value>>,

    #[serde(flatten)]
    pub query: Option<serde_json::Map<String, serde_json::Value>>,

    #[serde(flatten)]
    pub header: Option<serde_json::Map<String, serde_json::Value>>,

    #[serde(flatten, skip_serializing_if = "Body::is_raw")]
    pub body: Body,
}

#[derive(Debug, Serialize, Clone)]
#[serde(untagged)]
pub enum Body {
    Typed(Option<serde_json::Map<String, serde_json::Value>>),
    #[serde(skip)]
    Raw(Arc<std::sync::Mutex<Option<axum::body::Body>>>),
}

impl Body {
    pub fn is_raw(&self) -> bool {
        matches!(self, Body::Raw(_))
    }
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

            let request_schemas = request_encoding(&mut registry_builder, md, ep)?;
            let response_schema = response_encoding(&mut registry_builder, md, ep)?;

            endpoints.push(EndpointUnderConstruction {
                svc,
                ep,
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
        for req_schema in ep.request_schemas {
            let req_schema = req_schema.build(&registry)?;
            request_schemas.push(Arc::new(schema::Request {
                methods: req_schema.methods,
                path: req_schema
                    .schema
                    .path
                    .context("endpoint must have path defined")?,
                header: req_schema.schema.header,
                query: req_schema.schema.query,
                body: if raw {
                    schema::RequestBody::Raw
                } else {
                    schema::RequestBody::Typed(req_schema.schema.body)
                },
            }));
        }
        let resp_schema = ep.response_schema.build(&registry)?;

        // We only support a single gateway right now.
        let exposed = ep.ep.expose.contains_key("api-gateway");
        let raw =
            rpc::Protocol::try_from(ep.ep.proto).is_ok_and(|proto| proto == rpc::Protocol::Raw);

        let endpoint = Endpoint {
            name: EndpointName::new(ep.svc.name.clone(), ep.ep.name.clone()),
            path: ep.ep.path.clone().unwrap_or_else(|| meta::Path {
                r#type: meta::path::Type::Url as i32,
                segments: vec![meta::PathSegment {
                    r#type: meta::path_segment::SegmentType::Literal as i32,
                    value_type: meta::path_segment::ParamType::String as i32,
                    value: format!("/{}.{}", ep.ep.service_name, ep.ep.name),
                }],
            }),
            exposed,
            raw,
            requires_auth: !ep.ep.allow_unauthenticated,
            request: request_schemas,
            response: Arc::new(schema::Response {
                header: resp_schema.header,
                body: resp_schema.body,
            }),
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

        let (mut parts, body) = axum_req.with_limited_body().into_parts();

        // Authenticate the request from the platform, if applicable.
        let platform_seal_of_approval = match self.authenticate_platform(&parts) {
            Ok(seal) => seal,
            Err(_err) => None,
        };

        let meta = CallMeta::parse_with_caller(&self.shared.inbound_svc_auth, &parts.headers)?;

        let parsed_payload = match req_schema.extract(&mut parts, body).await {
            Ok(req) => req,
            Err(err) => return Err(err),
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
            data: model::RequestData::RPC(model::RPCRequestData {
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
            }),
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
                Err(err) => return err.into_response(),
            };

            // If the endpoint isn't exposed, return a 404.
            if !self.endpoint.exposed && !request.allows_private_endpoint_call() {
                return Error {
                    code: ErrCode::NotFound,
                    message: "endpoint not found".into(),
                    internal_message: Some("the endpoint was found, but is not exposed".into()),
                    stack: None,
                }
                .into_response();
            } else if self.endpoint.requires_auth && !request.has_authenticated_user() {
                return Error {
                    code: ErrCode::Unauthenticated,
                    message: "endpoint requires auth but none provided".into(),
                    internal_message: None,
                    stack: None,
                }
                .into_response();
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

            let (status_code, mut encoded_resp, resp_payload, error) = match resp {
                ResponseData::Raw(resp) => (resp.status().as_u16(), resp, None, None),
                ResponseData::Typed(Ok(payload)) => (
                    200,
                    self.endpoint
                        .response
                        .encode(&payload)
                        .unwrap_or_else(|err| err.into_response()),
                    Some(payload),
                    None,
                ),
                ResponseData::Typed(Err(err)) => (
                    err.code.status_code().as_u16(),
                    (&err).into_response(),
                    None,
                    Some(err),
                ),
            };

            {
                let model_resp = model::Response {
                    request: request.clone(),
                    duration,
                    data: model::ResponseData::RPC(model::RPCResponseData {
                        status_code,
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
