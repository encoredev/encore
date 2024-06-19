use std::borrow::Cow;
use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::Context;
use axum::async_trait;
use hyper::header;
use pingora::http::{RequestHeader, ResponseHeader};
use pingora::protocols::http::error_resp;
use pingora::proxy::{http_proxy_service, ProxyHttp, Session};
use pingora::server::configuration::Opt;
use pingora::server::Server;
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, ErrorSource, ErrorType};
use url::Url;

use crate::api::auth;
use crate::api::call::{CallDesc, ServiceRegistry};
use crate::api::paths::PathSet;
use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::{svcauth, CallMeta};
use crate::api::schema::Method;
use crate::{api, model, EncoreName};

use super::cors::cors_headers_config::CorsHeadersConfig;

#[derive(Clone)]
pub struct GatewayProxy {
    listen_addr: String,
    shared: Arc<SharedGatewayData>,
    service_registry: Arc<ServiceRegistry>,
    router: matchit::Router<MethodRoute>,
    cors_config: CorsHeadersConfig,
}

#[derive(Clone, Default)]
pub struct MethodRoute {
    get: Option<EncoreName>,
    head: Option<EncoreName>,
    post: Option<EncoreName>,
    put: Option<EncoreName>,
    delete: Option<EncoreName>,
    option: Option<EncoreName>,
    trace: Option<EncoreName>,
    patch: Option<EncoreName>,
}

impl MethodRoute {
    fn for_method(&self, method: api::schema::Method) -> Option<&EncoreName> {
        match method {
            Method::GET => self.get.as_ref(),
            Method::HEAD => self.head.as_ref(),
            Method::POST => self.post.as_ref(),
            Method::PUT => self.put.as_ref(),
            Method::DELETE => self.delete.as_ref(),
            Method::OPTIONS => self.option.as_ref(),
            Method::TRACE => self.trace.as_ref(),
            Method::PATCH => self.patch.as_ref(),
        }
    }
}

impl GatewayProxy {
    pub fn new(
        name: EncoreName,
        listen_addr: String,
        service_registry: Arc<ServiceRegistry>,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        cors: CorsHeadersConfig,
    ) -> anyhow::Result<Self> {
        let shared = Arc::new(SharedGatewayData {
            name,
            auth: auth_handler,
        });

        let mut router = matchit::Router::new();
        for (svc, routes) in [&service_routes.main, &service_routes.fallback]
            .into_iter()
            .flatten()
        {
            for (endpoint, paths) in routes {
                for path in paths {
                    let method_route = match router.at_mut(path) {
                        Ok(m) => m.value,
                        Err(_) => {
                            router.insert(path, MethodRoute::default())?;
                            router.at_mut(path).unwrap().value
                        }
                    };

                    for method in endpoint.methods() {
                        let prev = match method {
                            Method::GET => method_route.get.replace(svc.clone()),
                            Method::HEAD => method_route.head.replace(svc.clone()),
                            Method::POST => method_route.post.replace(svc.clone()),
                            Method::PUT => method_route.put.replace(svc.clone()),
                            Method::DELETE => method_route.delete.replace(svc.clone()),
                            Method::OPTIONS => method_route.option.replace(svc.clone()),
                            Method::TRACE => method_route.trace.replace(svc.clone()),
                            Method::PATCH => method_route.patch.replace(svc.clone()),
                        };

                        if prev.is_some() {
                            anyhow::bail!(
                                "tried to register same route twice {} {}",
                                method.as_str(),
                                path
                            );
                        }
                    }
                }
            }
        }

        Ok(GatewayProxy {
            shared,
            service_registry,
            router,
            listen_addr,
            cors_config: cors,
        })
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.shared.auth.as_ref()
    }

    pub fn run_forever(self) -> ! {
        let mut server = Server::new(Some(Opt {
            upgrade: false,
            daemon: false,
            nocapture: false,
            test: false,
            conf: None,
        }))
        .context("couldn't start gateway proxy")
        .unwrap();

        let listen_addr = self.listen_addr.clone();

        let mut proxy = http_proxy_service(&server.configuration, self);
        proxy.add_tcp(&listen_addr);
        server.add_service(proxy);

        server.run_forever()
    }
}

#[async_trait]
impl ProxyHttp for GatewayProxy {
    type CTX = Option<EncoreName>;
    fn new_ctx(&self) -> Option<EncoreName> {
        None
    }

    // see https://github.com/cloudflare/pingora/blob/main/docs/user_guide/internals.md for
    // details on when different filters are called.

    async fn upstream_peer(
        &self,
        session: &mut Session,
        ctx: &mut Self::CTX,
    ) -> pingora::Result<Box<HttpPeer>> {
        let path = session.req_header().uri.path();
        let method: Method = session
            .req_header()
            .method
            .clone()
            .try_into()
            .map_err(|e| Error::because(ErrorType::HTTPStatus(400), "invalid http method", e))?;

        let route = self
            .router
            .at(path)
            .map_err(|e| Error::because(ErrorType::HTTPStatus(404), "route not found", e))?;
        let service_name = route
            .value
            .for_method(method)
            .ok_or_else(|| Error::explain(ErrorType::HTTPStatus(405), "no route for method"))?;

        ctx.replace(service_name.clone());

        let upstream = self
            .service_registry
            .service_base_url(service_name)
            .ok_or_else(|| Error::explain(ErrorType::InternalError, "couldn't find upstream"))?;

        let upstream_url: Url = upstream
            .parse()
            .map_err(|e| Error::because(ErrorType::InternalError, "upstream not a valid url", e))?;
        let port = upstream_url
            .port()
            .ok_or_else(|| Error::explain(ErrorType::InternalError, "no port specified"))?;
        let upstream_addr: SocketAddr = match upstream_url.host() {
            Some(url::Host::Ipv4(ip)) => Ok((ip, port).into()),
            Some(url::Host::Ipv6(ip)) => Ok((ip, port).into()),
            _ => Err(Error::explain(
                ErrorType::InternalError,
                "upstream not a valid ipv4 or ipv6 address",
            )),
        }?;

        let peer = HttpPeer::new(upstream_addr, false, "".to_string());

        Ok(Box::new(peer))
    }

    async fn request_filter(
        &self,
        session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<bool>
    where
        Self::CTX: Send + Sync,
    {
        // preflight request, return early with cors headers
        if axum::http::Method::OPTIONS == session.req_header().method {
            let mut resp = ResponseHeader::build(200, None)?;
            self.cors_config.apply(session.req_header(), &mut resp)?;
            resp.insert_header(header::CONTENT_LENGTH, 0)?;
            session.write_response_header(Box::new(resp)).await?;

            return Ok(true);
        }

        Ok(false)
    }

    async fn response_filter(
        &self,
        session: &mut Session,
        upstream_response: &mut ResponseHeader,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<()>
    where
        Self::CTX: Send + Sync,
    {
        self.cors_config
            .apply(session.req_header(), upstream_response)?;
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
        let service_name = ctx
            .as_ref()
            .ok_or_else(|| Error::explain(ErrorType::InternalError, "ctx not set"))?;

        let svc_auth_method = self
            .service_registry
            .service_auth_method(service_name)
            .unwrap_or_else(|| Arc::new(svcauth::Noop));

        let headers = &session.req_header().headers;

        let mut call_meta = CallMeta::parse_without_caller(headers).map_err(|e| {
            Error::because(
                ErrorType::InternalError,
                "couldn't parse CallMeta from request",
                e,
            )
        })?;
        if call_meta.parent_span_id.is_none() {
            call_meta.parent_span_id = Some(model::SpanId::generate());
        }

        let caller = Caller::Gateway {
            gateway: self.shared.name.clone(),
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

        if let Some(auth_handler) = &self.shared.auth {
            let auth_response = auth_handler
                .authenticate(session.req_header(), call_meta.clone())
                .await
                .map_err(|e| {
                    Error::because(ErrorType::InternalError, "couldn't authenticate request", e)
                })?;

            if let auth::AuthResponse::Authenticated {
                auth_uid,
                auth_data,
            } = auth_response
            {
                desc.auth_user_id = Some(Cow::Owned(auth_uid));
                desc.auth_data = Some(auth_data);
            }
        }

        desc.add_meta(upstream_request).map_err(|e| {
            Error::because(ErrorType::InternalError, "couldn't set request meta", e)
        })?;

        Ok(())
    }

    async fn fail_to_proxy(&self, session: &mut Session, e: &Error, _ctx: &mut Self::CTX) -> u16
    where
        Self::CTX: Send + Sync,
    {
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
                                0
                            }
                            _ => 400,
                        }
                    }
                    ErrorSource::Internal | ErrorSource::Unset => 500,
                }
            }
        };
        if code > 0 {
            // modified version of `Session::respond_error` that adds cors headers
            let mut resp = match code {
                /* common error responses are pre-generated */
                502 => error_resp::HTTP_502_RESPONSE.clone(),
                400 => error_resp::HTTP_400_RESPONSE.clone(),
                _ => error_resp::gen_error_response(code),
            };

            if let Err(e) = self.cors_config.apply(session.req_header(), &mut resp) {
                log::error!("failed setting cors header in error response: {e}");
            }

            session.set_keepalive(None);

            session
                .write_response_header(Box::new(resp))
                .await
                .unwrap_or_else(|e| {
                    log::error!("failed to send error response to downstream: {e}");
                });
        }
        code
    }
}

impl crate::api::auth::InboundRequest for RequestHeader {
    fn headers(&self) -> &axum::http::HeaderMap {
        &self.headers
    }

    fn query(&self) -> Option<&str> {
        self.uri.query()
    }
}

#[derive(Debug, Clone)]
pub struct Route {
    pub methods: Vec<Method>,
    pub path: String,
}

struct SharedGatewayData {
    name: EncoreName,
    auth: Option<auth::Authenticator>,
}
