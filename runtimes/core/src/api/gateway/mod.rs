mod router;
mod websocket;

use std::borrow::Cow;
use std::collections::BTreeMap;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::bail;
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
    upstream_base_path: String,
    upstream_host: Option<String>,
    upstream_require_auth: bool,
    gateway: Arc<Gateway>,
}

impl GatewayCtx {
    fn prepend_base_path(&self, uri: &http::Uri) -> anyhow::Result<http::Uri> {
        let mut builder = http::Uri::builder();
        if let Some(scheme) = uri.scheme() {
            builder = builder.scheme(scheme.clone());
        }
        if let Some(authority) = uri.authority() {
            builder = builder.authority(authority.clone());
        }

        let base_path = self.upstream_base_path.trim_end_matches('/');
        builder = builder.path_and_query(format!(
            "{}{}",
            base_path,
            uri.path_and_query().map_or("", |pq| pq.as_str())
        ));

        builder.build().context("failed to build uri")
    }
}

pub struct Gateway {
    name: EncoreName,
    hostnames: Vec<String>,
    auth_handler: Option<auth::Authenticator>,
    router: router::Router,
    cors_config: CorsHeadersConfig,
}

impl Gateway {
    pub fn new(
        name: EncoreName,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        cors_config: CorsHeadersConfig,
        hostnames: Vec<String>,
    ) -> anyhow::Result<Self> {
        let mut router = router::Router::new();
        router.add_routes(&service_routes)?;

        Ok(Gateway {
            name,
            auth_handler,
            router,
            cors_config,
            hostnames,
        })
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.auth_handler.as_ref()
    }
}

#[derive(Clone)]
pub struct GatewayServer {
    gateways_by_name: BTreeMap<EncoreName, Arc<Gateway>>,
    gateways_by_host: BTreeMap<String, Arc<Gateway>>,
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
            gateways_by_name: BTreeMap::new(),
            gateways_by_host: BTreeMap::new(),
            service_registry,
            healthz,
            own_api_address,
            proxied_push_subs,
            internal_gateway: None,
            platform_validator,
        }
    }

    pub fn is_empty(&self) -> bool {
        self.gateways_by_name.is_empty()
    }

    pub fn gateway_by_name(&self, name: &str) -> Option<&Arc<Gateway>> {
        self.gateways_by_name.get(name)
    }

    pub fn gateway_by_host(&self, host: &str) -> Option<&Arc<Gateway>> {
        self.gateways_by_host.get(host)
    }

    pub fn add_gateway(&mut self, gateway: Gateway) -> anyhow::Result<()> {
        let gateway = Arc::new(gateway);

        let name = gateway.name.clone();
        if self
            .gateways_by_name
            .insert(name.clone(), gateway.clone())
            .is_some()
        {
            bail!("gateway '{}' already registered", &name)
        }

        for hostname in &gateway.hostnames {
            if self
                .gateways_by_host
                .insert(hostname.clone(), gateway.clone())
                .is_some()
            {
                bail!("multiple gateways registerd under hostname '{}'", &hostname)
            }
        }

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

        let by_host = req
            .headers()
            .get(HOST)
            .and_then(|v| v.to_str().ok())
            .inspect(|host| log::info!("hostname is {}", host))
            .and_then(|host| self.gateway_by_host(host));

        if let Some(gw) = by_host {
            log::info!("found gateway by hostname");
            return Some(gw);
        }

        // fallback to legacy behaviour
        self.gateway_by_name("api-gateway")
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

        let target = push_proxy_svc.map_or_else(
            || {
                // Find which service handles the path route
                session
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
                    .and_then(|method| {
                        let target = target_gateway.router.route_to_service(method, path);

                        // If no route found, try the internal gateway
                        if target.is_err() && self.internal_gateway.is_some() {
                            log::trace!("no route found, trying internal gateway");
                            if let Ok(data) =
                                platform::ValidationData::from_req(session.req_header())
                            {
                                if self
                                    .platform_validator
                                    .validate_platform_request(&data)
                                    .is_ok()
                                {
                                    log::trace!(
                                        "platform request validated, routing to internal gateway"
                                    );
                                    if let Ok(internal_target) = self
                                        .internal_gateway
                                        .as_ref()
                                        .unwrap()
                                        .router
                                        .route_to_service(method, path)
                                    {
                                        return Ok(internal_target);
                                    }
                                }
                            }
                        }
                        target
                    })
                    .cloned()
            },
            Ok,
        )?;

        let upstream = self
            .service_registry
            .service_base_url(&target.service_name)
            .or_err(ErrorType::InternalError, "couldn't find upstream")?;

        let upstream_url: Url = upstream
            .parse()
            .or_err(ErrorType::InternalError, "upstream not a valid url")?;

        let upstream_addrs = upstream_url
            .socket_addrs(|| match upstream_url.scheme() {
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

        let tls = upstream_url.scheme() == "https";
        let host = upstream_url.host().map(|h| h.to_string());
        let peer = HttpPeer::new(upstream_addr, tls, host.clone().unwrap_or_default());

        ctx.replace(GatewayCtx {
            upstream_base_path: upstream_url.path().to_string(),
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
            let new_uri = gateway_ctx
                .prepend_base_path(&upstream_request.uri)
                .or_err(
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
