use std::borrow::Cow;
use std::net::TcpListener;
use std::sync::Arc;

use anyhow::Context;
use axum::async_trait;
use bytes::{BufMut, BytesMut};
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
pub struct Gateway {
    shared: Arc<SharedGatewayData>,
    service_registry: Arc<ServiceRegistry>,
    router: matchit::Router<MethodRoute>,
    cors_config: CorsHeadersConfig,
}

pub struct GatewayCtx {
    base_path: String,
    service_name: EncoreName,
}

impl GatewayCtx {
    fn prepend_base_path(&self, uri: http::Uri) -> anyhow::Result<http::Uri> {
        let base_path = self
            .base_path
            .strip_suffix('/')
            .unwrap_or(self.base_path.as_str());

        let mut parts = uri.into_parts();
        parts.path_and_query = Some(
            format!(
                "{}{}",
                base_path,
                parts
                    .path_and_query
                    .map(|p| p.to_string())
                    .unwrap_or_else(|| "/".to_string())
            )
            .parse()?,
        );

        Ok(http::Uri::from_parts(parts)?)
    }
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

impl Gateway {
    pub fn new(
        name: EncoreName,
        service_registry: Arc<ServiceRegistry>,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        cors_config: CorsHeadersConfig,
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

        Ok(Gateway {
            shared,
            service_registry,
            router,
            cors_config,
        })
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.shared.auth.as_ref()
    }

    pub fn run_forever(self, listener: TcpListener) -> ! {
        let mut server = Server::new(Some(Opt {
            upgrade: false,
            daemon: false,
            nocapture: false,
            test: false,
            conf: None,
        }))
        .context("couldn't start gateway proxy")
        .unwrap();

        let mut proxy = http_proxy_service(&server.configuration, self);
        let listen_addr = listener.local_addr().unwrap().to_string();

        // unbind the address and let pingora re-bind it
        drop(listener);
        proxy.add_tcp(&listen_addr);
        server.add_service(proxy);

        server.run_forever()
    }
}

#[async_trait]
impl ProxyHttp for Gateway {
    type CTX = Option<GatewayCtx>;
    fn new_ctx(&self) -> Self::CTX {
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

        let route = self.router.at(path).map_err(|_| api::Error {
            code: api::ErrCode::NotFound,
            message: "endpoint not found".to_string(),
            internal_message: Some(format!("no such endpoint exists: {}", path)),
            stack: None,
        })?;

        let service_name = route
            .value
            .for_method(method)
            .ok_or_else(|| Error::explain(ErrorType::HTTPStatus(405), "no route for method"))?;

        let upstream = self
            .service_registry
            .service_base_url(service_name)
            .ok_or_else(|| Error::explain(ErrorType::InternalError, "couldn't find upstream"))?;

        let upstream_url: Url = upstream
            .parse()
            .map_err(|e| Error::because(ErrorType::InternalError, "upstream not a valid url", e))?;

        let upstream_addrs = upstream_url
            .socket_addrs(|| match upstream_url.scheme() {
                "https" => Some(443),
                "http" => Some(80),
                _ => None,
            })
            .map_err(|e| {
                Error::because(
                    ErrorType::InternalError,
                    "couldn't lookup upstream ip address",
                    e,
                )
            })?;

        let upstream_addr = upstream_addrs.first().ok_or_else(|| {
            Error::explain(
                ErrorType::InternalError,
                "didn't find any upstream ip addresses",
            )
        })?;

        let peer = HttpPeer::new(upstream_addr, false, "".to_string());

        ctx.replace(GatewayCtx {
            base_path: upstream_url.path().to_string(),
            service_name: service_name.clone(),
        });
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
        let gateway_ctx = ctx
            .as_ref()
            .ok_or_else(|| Error::explain(ErrorType::InternalError, "ctx not set"))?;

        let new_uri = gateway_ctx
            .prepend_base_path(upstream_request.uri.clone())
            .map_err(|e| {
                Error::because(
                    ErrorType::InternalError,
                    "failed to prepend upstream base path",
                    e,
                )
            })?;

        upstream_request.set_uri(new_uri);

        let svc_auth_method = self
            .service_registry
            .service_auth_method(&gateway_ctx.service_name)
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

            session.write_response_body(body.unwrap()).await.unwrap();
        };

        code
    }
}

fn as_api_error(err: &pingora::Error) -> Option<&api::Error> {
    if let Some(cause) = &err.cause {
        cause.downcast_ref::<api::Error>()
    } else {
        None
    }
}

fn api_error_response(err: &api::Error) -> (ResponseHeader, bytes::Bytes) {
    let mut buf = BytesMut::with_capacity(128).writer();
    serde_json::to_writer(&mut buf, &err).unwrap();

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

#[derive(Debug, Clone)]
pub struct Route {
    pub methods: Vec<Method>,
    pub path: String,
}

struct SharedGatewayData {
    name: EncoreName,
    auth: Option<auth::Authenticator>,
}
