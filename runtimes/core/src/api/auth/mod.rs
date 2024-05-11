use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use serde::Serialize;

use crate::api::reqauth::CallMeta;
use crate::api::APIResult;
use crate::{api, EndpointName};

use crate::api::schema::encoding::Schema;
pub use local::LocalAuthHandler;
pub use remote::RemoteAuthHandler;

mod local;
mod remote;

pub type AxumRequest = axum::http::Request<axum::body::Body>;

pub struct AuthRequest {
    pub headers: axum::http::HeaderMap,
    pub query: Option<String>,
    pub call_meta: CallMeta,
}

pub enum AuthResponse {
    Authenticated {
        auth_uid: String,
        auth_data: serde_json::Map<String, serde_json::Value>,
    },
    Unauthenticated,
}

/// A trait for handlers that accept auth parameters and return an auth result.
pub trait AuthHandler: Sync + Send + 'static {
    fn name(&self) -> &EndpointName;

    fn handle_auth(
        self: Arc<Self>,
        req: AuthRequest,
    ) -> Pin<Box<dyn Future<Output = APIResult<AuthResponse>> + Send + 'static>>;
}

pub struct Authenticator {
    schema: Schema,
    auth_handler: AuthHandlerType,
}

#[derive(Clone)]
pub enum AuthHandlerType {
    Local(Arc<LocalAuthHandler>),
    Remote(Arc<RemoteAuthHandler>),
}

impl AuthHandlerType {
    fn set_local_handler(&self, handler: Option<Arc<dyn api::TypedHandler>>) {
        if let Self::Local(local) = self {
            local.set_handler(handler);
        }
    }
}

impl Authenticator {
    pub fn new(schema: Schema, auth_handler: AuthHandlerType) -> anyhow::Result<Self> {
        Ok(Self {
            schema,
            auth_handler,
        })
    }

    pub fn local(schema: Schema, local: LocalAuthHandler) -> anyhow::Result<Self> {
        Self::new(schema, AuthHandlerType::Local(Arc::new(local)))
    }

    pub fn remote(schema: Schema, remote: RemoteAuthHandler) -> anyhow::Result<Self> {
        Self::new(schema, AuthHandlerType::Remote(Arc::new(remote)))
    }

    pub fn schema(&self) -> &Schema {
        &self.schema
    }

    pub async fn authenticate<R: InboundRequest>(
        &self,
        req: &R,
        meta: CallMeta,
    ) -> APIResult<AuthResponse> {
        if !self.contains_auth_params(req) {
            return Ok(AuthResponse::Unauthenticated);
        }

        let auth_req = self.build_auth_request(req, meta);
        let resp = match &self.auth_handler {
            AuthHandlerType::Local(local) => local.clone().handle_auth(auth_req).await,
            AuthHandlerType::Remote(remote) => remote.clone().handle_auth(auth_req).await,
        };
        match resp {
            Ok(resp) => Ok(resp),
            Err(err) if err.code == api::ErrCode::Unauthenticated => {
                Ok(AuthResponse::Unauthenticated)
            }
            Err(err) => Err(err),
        }
    }

    pub fn set_local_handler_impl(&self, handler: Option<Arc<dyn api::TypedHandler>>) {
        self.auth_handler.set_local_handler(handler);
    }

    fn build_auth_request<R: InboundRequest>(
        &self,
        inbound: &R,
        mut call_meta: CallMeta,
    ) -> AuthRequest {
        // Ignore the parent span id as gateways don't currently record a span.
        call_meta.parent_span_id = None;

        // Headers.
        let headers = match &self.schema.header {
            None => axum::http::header::HeaderMap::new(),
            Some(schema) => {
                let mut dest = axum::http::header::HeaderMap::with_capacity(schema.len());
                let inbound_headers = inbound.headers();
                for (json_key, field) in schema.fields() {
                    let header_name = field.name_override.as_deref().unwrap_or(json_key.as_ref());
                    let Ok(header_name) =
                        axum::http::HeaderName::from_bytes(header_name.as_bytes())
                    else {
                        continue;
                    };
                    for value in inbound_headers.get_all(&header_name) {
                        dest.append(header_name.clone(), value.to_owned());
                    }
                }
                dest
            }
        };

        // Move query params.
        let query = match &self.schema.query {
            None => None,
            Some(schema) => {
                let query_data = inbound.query().unwrap_or_default().as_bytes();
                let parsed = form_urlencoded::parse(query_data);

                let mut dest = form_urlencoded::Serializer::new(String::new());
                for (key, value) in parsed {
                    if schema.contains_name(key.as_ref()) {
                        dest.append_pair(key.as_ref(), value.as_ref());
                    }
                }

                Some(dest.finish())
            }
        };

        AuthRequest {
            headers,
            query,
            call_meta,
        }
    }

    fn contains_auth_params<R: InboundRequest>(&self, req: &R) -> bool {
        if let Some(query) = &self.schema.query {
            if query.contains_any(req.query().unwrap_or_default().as_bytes()) {
                return true;
            }
        }

        if let Some(header) = &self.schema.header {
            let h = req.headers();
            if header.contains_any(&h) {
                return true;
            }
        }

        false
    }
}

#[derive(Debug, Serialize, Clone)]
pub struct AuthPayload {
    #[serde(flatten)]
    pub query: Option<serde_json::Map<String, serde_json::Value>>,

    #[serde(flatten)]
    pub header: Option<serde_json::Map<String, serde_json::Value>>,
}

pub trait InboundRequest {
    fn headers(&self) -> &axum::http::HeaderMap;
    fn query(&self) -> Option<&str>;
}

impl InboundRequest for axum::http::Request<axum::body::Body> {
    fn headers(&self) -> &axum::http::HeaderMap {
        self.headers()
    }

    fn query(&self) -> Option<&str> {
        self.uri().query()
    }
}
