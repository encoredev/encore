mod router;
mod websocket;

use std::borrow::Cow;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::Context;
use axum::async_trait;
use bytes::{BufMut, Bytes, BytesMut};
use http::header::HOST;
use hyper::header;
use pingora::http::{RequestHeader, ResponseHeader};
use pingora::protocols::http::error_resp;
use pingora::proxy::{http_proxy_service, ProxyHttp, Session};
use pingora::server::configuration::{Opt, ServerConf};
use pingora::services::Service;
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, ErrorSource, ErrorType, OkOrErr, OrErr};
use router::Target;
use tokio::sync::watch;
use url::Url;

use crate::api::auth;
use crate::api::call::{CallDesc, ServiceRegistry};
use crate::api::paths::PathSet;
use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::platform;
use crate::api::reqauth::{svcauth, CallMeta};
use crate::{api, model, EncoreName};

use super::auth::InboundRequest;
use super::cors::cors_headers_config::CorsHeadersConfig;
use super::encore_routes::healthz;

pub struct GatewayCtx {
    upstream_service_name: EncoreName,
    upstream_path: String,
    upstream_host: Option<String>,
    upstream_require_auth: bool,
    gateway: Arc<Gateway>,
}

impl GatewayCtx {
    fn with_path(&self, uri: &http::Uri) -> anyhow::Result<http::Uri> {
        let mut builder = http::Uri::builder();
        if let Some(scheme) = uri.scheme() {
            builder = builder.scheme(scheme.clone());
        }
        if let Some(authority) = uri.authority() {
            builder = builder.authority(authority.clone());
        }

        let path_and_query = if let Some(query) = uri.query() {
            format!("{}?{}", self.upstream_path, query)
        } else {
            self.upstream_path.clone()
        };

        builder = builder.path_and_query(path_and_query);
        builder.build().context("failed to build uri")
    }
}

#[derive(Debug)]
pub struct GatewayMatchRule {
    pub hostname: Option<String>,
    pub path_prefix: Option<String>,
}

impl GatewayMatchRule {
    fn matches(&self, host: &str, path: &str) -> bool {
        // if host is specified in the match rule, check that it matches
        if let Some(hostname) = &self.hostname {
            if hostname != host {
                return false;
            }
        }

        // if path prefix is specified in the match rule, check if it matches
        if let Some(path_prefix) = &self.path_prefix {
            if !path.starts_with(path_prefix) {
                return false;
            }
        }

        true
    }
}

pub struct Gateway {
    name: EncoreName,
    auth_handler: Option<auth::Authenticator>,
    router: router::Router,
    internal_router: router::Router,
    cors_config: CorsHeadersConfig,
    match_rules: Vec<GatewayMatchRule>,
}

impl std::fmt::Debug for Gateway {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Gateway")
            .field("name", &self.name)
            .field("auth_handler", &self.auth_handler.is_some())
            .field("cors_config", &self.cors_config)
            .field("match_rules", &self.match_rules)
            .finish()
    }
}

impl Gateway {
    pub fn new(
        name: EncoreName,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        internal_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        cors_config: CorsHeadersConfig,
        match_rules: Vec<GatewayMatchRule>,
    ) -> anyhow::Result<Self> {
        let router = router::Router::new(&service_routes)?;
        let internal_router = router::Router::new(&internal_routes)?;

        Ok(Gateway {
            name,
            auth_handler,
            router,
            internal_router,
            cors_config,
            match_rules,
        })
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.auth_handler.as_ref()
    }
}

#[derive(Clone)]
pub struct GatewayServer {
    gateways: Vec<Arc<Gateway>>,
    service_registry: Arc<ServiceRegistry>,
    healthz: healthz::Handler,
    own_api_address: Option<SocketAddr>,
    proxied_push_subs: HashMap<String, EncoreName>,
    platform_validator: Arc<platform::RequestValidator>,
    internal_gateway: Option<Arc<Gateway>>,
}

impl GatewayServer {
    pub fn new(
        service_registry: Arc<ServiceRegistry>,
        healthz: healthz::Handler,
        own_api_address: Option<SocketAddr>,
        proxied_push_subs: HashMap<String, EncoreName>,
        platform_validator: Arc<platform::RequestValidator>,
    ) -> Self {
        GatewayServer {
            service_registry,
            healthz,
            own_api_address,
            proxied_push_subs,
            internal_gateway: None,
            platform_validator,
            gateways: Vec::new(),
        }
    }

    pub fn is_empty(&self) -> bool {
        self.gateways.is_empty()
    }

    pub fn gateway_by_name(&self, name: &str) -> Option<&Arc<Gateway>> {
        self.gateways.iter().find(|gw| gw.name.as_ref() == name)
    }

    pub fn add_gateway(&mut self, gateway: Gateway) -> anyhow::Result<()> {
        let gateway = Arc::new(gateway);
        self.gateways.push(gateway);

        // TODO(fredr) validate conflicting routing rules etc?

        Ok(())
    }

    pub fn set_internal_gateway(&mut self, gateway: Gateway) {
        self.internal_gateway = Some(Arc::new(gateway));
    }

    pub async fn serve(self, listen_addr: &str) -> anyhow::Result<()> {
        let conf = Arc::new(
            ServerConf::new_with_opt_override(&Opt {
                upgrade: false,
                daemon: false,
                nocapture: false,
                test: false,
                conf: None,
            })
            .unwrap(),
        );
        let mut proxy = http_proxy_service(&conf, self);

        proxy.add_tcp(listen_addr);

        let (_tx, rx) = watch::channel(false);
        proxy
            .start_service(
                #[cfg(unix)]
                None,
                rx,
            )
            .await;

        Ok(())
    }

    fn target(&self, req: &RequestHeader) -> Option<&Arc<Gateway>> {
        // for testing purposes, look at `x-encore-gateway-name` header
        if let Some(name) = req.headers().get("x-encore-gateway-name") {
            if let Ok(name) = name.to_str() {
                return self.gateway_by_name(name);
            }
        }

        let req_headers = req.headers();
        let req_host = req_headers
            .get(HOST)
            .and_then(|hv| hv.to_str().ok())
            .unwrap_or_default();
        let req_path = req.uri.path();

        self.gateways.iter().find(|&gateway| {
            gateway
                .match_rules
                .iter()
                .any(|rule| rule.matches(req_host, req_path))
        })
    }
}

#[async_trait]
impl ProxyHttp for GatewayServer {
    type CTX = Option<GatewayCtx>;

    fn new_ctx(&self) -> Self::CTX {
        None
    }

    // see https://github.com/cloudflare/pingora/blob/main/docs/user_guide/internals.md for
    // details on when different filters are called.

    async fn request_filter(
        &self,
        session: &mut Session,
        ctx: &mut Self::CTX,
    ) -> pingora::Result<bool>
    where
        Self::CTX: Send + Sync,
    {
        if session.req_header().uri.path() == "/__encore/healthz" {
            let healthz_resp = self.healthz.clone().health_check();
            let healthz_bytes: Vec<u8> = serde_json::to_vec(&healthz_resp)
                .or_err(ErrorType::HTTPStatus(500), "could not encode response")?;

            let mut header = ResponseHeader::build(200, None)?;
            header.insert_header(header::CONTENT_LENGTH, healthz_bytes.len())?;
            header.insert_header(header::CONTENT_TYPE, "application/json")?;
            session
                .write_response_header(Box::new(header), false)
                .await?;
            session
                .write_response_body(Some(Bytes::from(healthz_bytes)), true)
                .await?;

            return Ok(true);
        }

        if let Some(GatewayCtx { gateway, .. }) = ctx {
            // preflight request, return early with cors headers
            if axum::http::Method::OPTIONS == session.req_header().method {
                let mut resp = ResponseHeader::build(200, None)?;
                gateway.cors_config.apply(session.req_header(), &mut resp)?;
                resp.insert_header(header::CONTENT_LENGTH, 0)?;
                session.write_response_header(Box::new(resp), true).await?;

                return Ok(true);
            }
        }

        Ok(false)
    }

    async fn upstream_peer(
        &self,
        session: &mut Session,
        ctx: &mut Self::CTX,
    ) -> pingora::Result<Box<HttpPeer>> {
        let target_gateway = self
            .target(session.req_header())
            .ok_or_else(|| api::Error::not_found("gateway not found"))?;

        let path = session.req_header().uri.path();

        // Check if this is a pubsub push request and if we need to proxy it to another service
        let push_proxy_svc = path
            .strip_prefix("/__encore/pubsub/push/")
            .and_then(|sub_id| self.proxied_push_subs.get(sub_id))
            .map(|svc| Target {
                service_name: svc.clone(),
                requires_auth: false,
            });

        if let Some(own_api_addr) = &self.own_api_address {
            if push_proxy_svc.is_none() && path.starts_with("/__encore/") {
                return Ok(Box::new(HttpPeer::new(own_api_addr, false, "".to_string())));
            }
        }

        let upstream_path = session.req_header().uri.path();
        let target = if let Some(proxy_svc) = push_proxy_svc {
            proxy_svc
        } else {
            let target = session
                .req_header()
                .method
                .as_ref()
                .try_into()
                .map_err(|e: anyhow::Error| api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "invalid method".to_string(),
                    internal_message: Some(e.to_string()),
                    stack: None,
                    details: None,
                })
                .and_then(|method| target_gateway.router.route_to_service(method, path))
                .cloned();

            if target.is_err()
                && session
                    .req_header()
                    .headers()
                    .get(platform::ENCORE_AUTH_HEADER)
                    .is_some()
            {
                log::trace!("validating internal credentials");
                if let Ok(data) = platform::ValidationData::from_req(session.req_header()) {
                    self.platform_validator
                        .validate_platform_request(&data)
                        .or_err(ErrorType::HTTPStatus(401), "Unauthorized internal request")?;
                }

                // TODO lookup target to service based on path
                // TODO strip service prefix from `upstream_path`
            }

            target?
        };

        let upstream_base_url: Url = self
            .service_registry
            .service_base_url(&target.service_name)
            .or_err(ErrorType::InternalError, "couldn't find upstream")?
            .parse()
            .or_err(ErrorType::InternalError, "upstream not a valid url")?;

        let upstream_base_path = upstream_base_url.path();
        let upstream_path = format!(
            "{}{}",
            upstream_base_path,
            upstream_path.trim_start_matches('/')
        );

        let upstream_addrs = upstream_base_url
            .socket_addrs(|| match upstream_base_url.scheme() {
                "https" => Some(443),
                "http" => Some(80),
                _ => None,
            })
            .or_err(
                ErrorType::InternalError,
                "couldn't lookup upstream ip address",
            )?;

        let upstream_addr = upstream_addrs.first().or_err(
            ErrorType::InternalError,
            "didn't find any upstream ip addresses",
        )?;

        let tls = upstream_base_url.scheme() == "https";
        let host = upstream_base_url.host().map(|h| h.to_string());
        let peer = HttpPeer::new(upstream_addr, tls, host.clone().unwrap_or_default());

        ctx.replace(GatewayCtx {
            upstream_path: upstream_path.to_string(),
            upstream_host: host,
            upstream_service_name: target.service_name.clone(),
            upstream_require_auth: target.requires_auth,
            gateway: target_gateway.clone(),
        });

        Ok(Box::new(peer))
    }

    async fn response_filter(
        &self,
        session: &mut Session,
        upstream_response: &mut ResponseHeader,
        ctx: &mut Self::CTX,
    ) -> pingora::Result<()>
    where
        Self::CTX: Send + Sync,
    {
        if let Some(GatewayCtx { gateway, .. }) = ctx {
            gateway
                .cors_config
                .apply(session.req_header(), upstream_response)?;
        }

        Ok(())
    }

    async fn upstream_request_filter(
        &self,
        session: &mut Session,
        upstream_request: &mut RequestHeader,
        ctx: &mut Self::CTX,
    ) -> pingora::Result<()>
    where
        Self::CTX: Send + Sync,
    {
        if let Some(gateway_ctx) = ctx.as_ref() {
            let new_uri = gateway_ctx.with_path(&upstream_request.uri).or_err(
                ErrorType::InternalError,
                "failed to prepend upstream base path",
            )?;

            upstream_request.set_uri(new_uri);

            // Do we need to set the host header here?
            // It means the upstream service won't be able to tell
            // what the original Host header was, which is sometimes useful.
            if let Some(ref host) = gateway_ctx.upstream_host {
                upstream_request.insert_header(header::HOST, host)?;
            }

            if session.is_upgrade_req() {
                websocket::update_headers_from_websocket_protocol(upstream_request).or_err(
                    ErrorType::HTTPStatus(400),
                    "invalid auth data passed in websocket protocol header",
                )?;
            }

            let svc_auth_method = self
                .service_registry
                .service_auth_method(&gateway_ctx.upstream_service_name)
                .unwrap_or_else(|| Arc::new(svcauth::Noop));

            let headers = &upstream_request.headers;

            let mut call_meta = CallMeta::parse_without_caller(headers).or_err(
                ErrorType::InternalError,
                "couldn't parse CallMeta from request",
            )?;
            if call_meta.parent_span_id.is_none() {
                call_meta.parent_span_id = Some(model::SpanId::generate());
            }

            let caller = Caller::Gateway {
                gateway: gateway_ctx.gateway.name.clone(),
            };
            let mut desc = CallDesc {
                caller: &caller,
                parent_span: call_meta
                    .parent_span_id
                    .map(|sp| call_meta.trace_id.with_span(sp)),
                parent_event_id: None,
                ext_correlation_id: call_meta
                    .ext_correlation_id
                    .as_ref()
                    .map(|s| Cow::Borrowed(s.as_str())),
                auth_user_id: None,
                auth_data: None,
                svc_auth_method: svc_auth_method.as_ref(),
            };

            if let Some(auth_handler) = &gateway_ctx.gateway.auth_handler {
                let auth_response = auth_handler
                    .authenticate(upstream_request, call_meta.clone())
                    .await
                    .or_err(ErrorType::InternalError, "couldn't authenticate request")?;

                match auth_response {
                    auth::AuthResponse::Authenticated {
                        auth_uid,
                        auth_data,
                    } => {
                        desc.auth_user_id = Some(Cow::Owned(auth_uid));
                        desc.auth_data = Some(auth_data);
                    }
                    auth::AuthResponse::Unauthenticated { error } => {
                        if gateway_ctx.upstream_require_auth {
                            return Err(error.into());
                        }
                    }
                };
            }

            desc.add_meta(upstream_request)
                .or_err(ErrorType::InternalError, "couldn't set request meta")?;
        }

        Ok(())
    }

    async fn fail_to_proxy(&self, session: &mut Session, e: &Error, ctx: &mut Self::CTX) -> u16
    where
        Self::CTX: Send + Sync,
    {
        // modified version of `Session::respond_error` that adds cors headers,
        // and handles specific errors

        let code = match e.etype() {
            ErrorType::HTTPStatus(code) => *code,
            _ => {
                match e.esource() {
                    ErrorSource::Upstream => 502,
                    ErrorSource::Downstream => {
                        match e.etype() {
                            ErrorType::WriteError
                            | ErrorType::ReadError
                            | ErrorType::ConnectionClosed => {
                                /* conn already dead */
                                return 0;
                            }
                            _ => 400,
                        }
                    }
                    ErrorSource::Internal | ErrorSource::Unset => 500,
                }
            }
        };

        let (mut resp, body) = if let Some(api_error) = as_api_error(e) {
            let (resp, body) = api_error_response(api_error);
            (resp, Some(body))
        } else {
            (
                match code {
                    /* common error responses are pre-generated */
                    502 => error_resp::HTTP_502_RESPONSE.clone(),
                    400 => error_resp::HTTP_400_RESPONSE.clone(),
                    _ => error_resp::gen_error_response(code),
                },
                None,
            )
        };

        if let Some(GatewayCtx { gateway, .. }) = ctx {
            if let Err(e) = gateway.cors_config.apply(session.req_header(), &mut resp) {
                log::error!("failed setting cors header in error response: {e}");
            }
        }

        session.set_keepalive(None);
        session
            .write_response_header(Box::new(resp), false)
            .await
            .unwrap_or_else(|e| {
                log::error!("failed to send error response to downstream: {e}");
            });

        session
            .write_response_body(body, true)
            .await
            .unwrap_or_else(|e| log::error!("failed to write body: {e}"));

        code
    }
}

fn as_api_error(err: &pingora::Error) -> Option<&api::Error> {
    err.root_cause().downcast_ref::<api::Error>()
}

fn api_error_response(err: &api::Error) -> (ResponseHeader, bytes::Bytes) {
    let mut buf = BytesMut::with_capacity(128).writer();
    serde_json::to_writer(&mut buf, &err.as_external()).unwrap();

    let mut resp = ResponseHeader::build(err.code.status_code(), Some(5)).unwrap();
    resp.insert_header(header::SERVER, &pingora::protocols::http::SERVER_NAME[..])
        .unwrap();
    resp.insert_header(header::DATE, "Sun, 06 Nov 1994 08:49:37 GMT")
        .unwrap(); // placeholder
    resp.insert_header(header::CONTENT_LENGTH, buf.get_ref().len())
        .unwrap();
    resp.insert_header(header::CACHE_CONTROL, "private, no-store")
        .unwrap();
    resp.insert_header(header::CONTENT_TYPE, mime::APPLICATION_JSON.as_ref())
        .unwrap();

    (resp, buf.into_inner().into())
}

impl crate::api::auth::InboundRequest for RequestHeader {
    fn headers(&self) -> &axum::http::HeaderMap {
        &self.headers
    }

    fn query(&self) -> Option<&str> {
        self.uri.query()
    }
}
