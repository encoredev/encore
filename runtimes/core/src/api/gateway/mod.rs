use std::borrow::Cow;
use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use anyhow::Context;
use axum::async_trait;
use pingora::http::RequestHeader;
use pingora::proxy::{http_proxy_service, ProxyHttp, Session};
use pingora::server::configuration::Opt;
use pingora::server::Server;
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, ErrorType};
use url::Url;

use crate::api::call::{CallDesc, ServiceRegistry};
use crate::api::gateway::reverseproxy::{Director, InboundRequest, ProxyRequest, ReverseProxy};
use crate::api::paths::PathSet;
use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::{svcauth, CallMeta};
use crate::api::schema::Method;
use crate::api::{auth, schema, APIResult, IntoResponse};
use crate::{api, model, EncoreName};

use super::reqauth::svcauth::ServiceAuthMethod;

mod reverseproxy;

#[derive(Clone)]
pub struct GatewayProxy {
    shared: Arc<SharedGatewayData>,
    upstream: HttpPeer,
    service_registry: Arc<ServiceRegistry>,
    service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
}

impl GatewayProxy {
    pub fn new(
        name: EncoreName,
        upstream_addr: &str,
        service_registry: Arc<ServiceRegistry>,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        // TODO: cors layer ? / handle cors?
    ) -> anyhow::Result<Self> {
        let shared = Arc::new(SharedGatewayData {
            name,
            auth: auth_handler,
        });

        let upstream = HttpPeer::new(upstream_addr, false, "".to_string());

        Ok(GatewayProxy {
            shared,
            upstream,
            service_routes,
            service_registry,
        })
    }

    pub fn run_forever(self) -> ! {
        let mut server = Server::new(Some(Opt {
            upgrade: false,
            daemon: false,
            nocapture: false,
            test: false,
            conf: None,
        }))
        .unwrap();
        let mut proxy = http_proxy_service(&server.configuration, self);

        proxy.add_tcp("0.0.0.0:8888");
        server.add_service(proxy);

        server.run_forever();
    }

    fn auth_method_for_path(
        &self,
        path: &str,
    ) -> anyhow::Result<Arc<dyn svcauth::ServiceAuthMethod>> {
        for (svc, service_routes) in &self.service_routes.main {
            let dest_base_url: Url = self
                .service_registry
                .service_base_url(svc)
                .with_context(|| format!("service {} not found", svc))?
                .parse()
                .context("invalid service base url")?;

            for (endpoint, routes) in service_routes {
                for route in routes {
                    //if route == path {
                    let svc_auth_method = self
                        .service_registry
                        .service_auth_method(svc)
                        .unwrap_or_else(|| Arc::new(svcauth::Noop));
                    return Ok(svc_auth_method);
                    //}
                }
            }
        }

        //Ok(Arc::new(svcauth::Noop))
        anyhow::bail!("OOH NOO");
    }
}

#[async_trait]
impl ProxyHttp for GatewayProxy {
    type CTX = ();
    fn new_ctx(&self) {}

    // see https://github.com/cloudflare/pingora/blob/main/docs/user_guide/internals.md for
    // details on when different filters are called.

    async fn upstream_peer(
        &self,
        _session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<Box<HttpPeer>> {
        Ok(Box::new(self.upstream.clone()))
    }

    async fn upstream_request_filter(
        &self,
        session: &mut Session,
        upstream_request: &mut RequestHeader,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<()>
    where
        Self::CTX: Send + Sync,
    {
        let path = session.req_header().uri.path();
        println!(
            "upstream request filter, req header: {:?}",
            session.req_header()
        );
        let svc_auth_method = self.auth_method_for_path(path).map_err(|e| {
            Error::because(
                ErrorType::HTTPStatus(404),
                "couldn't determine upstream service",
                e,
            )
        })?;

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
}

impl crate::api::auth::InboundRequest for RequestHeader {
    fn headers(&self) -> &axum::http::HeaderMap {
        &self.headers
    }

    fn query(&self) -> Option<&str> {
        self.uri.query()
    }
}

pub struct Gateway {
    shared: Arc<SharedGatewayData>,
    router: Mutex<Option<axum::Router>>,
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

impl Gateway {
    pub fn new(
        name: EncoreName,
        http_client: reqwest::Client,
        service_registry: Arc<ServiceRegistry>,
        service_routes: PathSet<EncoreName, Arc<api::Endpoint>>,
        auth_handler: Option<auth::Authenticator>,
        cors: tower_http::cors::CorsLayer,
    ) -> anyhow::Result<Self> {
        // Register the routes, and track the handlers in a map so we can easily
        // set the request handler when registered.
        let mut router = axum::Router::new();

        async fn not_found_handler(
            req: axum::http::Request<axum::body::Body>,
        ) -> axum::response::Response<axum::body::Body> {
            api::Error {
                code: api::ErrCode::NotFound,
                message: "endpoint not found".to_string(),
                internal_message: Some(format!("no such endpoint exists: {}", req.uri().path())),
                stack: None,
            }
            .into_response()
        }
        let mut fallback_router = axum::Router::new();
        fallback_router = fallback_router.fallback(not_found_handler);

        let shared = Arc::new(SharedGatewayData {
            name,
            auth: auth_handler,
        });

        #[allow(clippy::type_complexity)]
        let register_routes =
            |paths: HashMap<EncoreName, Vec<(Arc<api::Endpoint>, Vec<String>)>>,
             mut router: axum::Router|
             -> anyhow::Result<axum::Router> {
                for (svc, service_routes) in paths {
                    let dest_base_url = service_registry
                        .service_base_url(&svc)
                        .with_context(|| format!("service {} not found", svc))?
                        .parse()
                        .context("invalid service base url")?;

                    let director = Arc::new(ServiceDirector {
                        shared: shared.clone(),
                        dest_base_url,
                        svc_auth_method: service_registry
                            .service_auth_method(&svc)
                            .unwrap_or_else(|| Arc::new(svcauth::Noop)),
                    });
                    let proxy = ReverseProxy::new(director, http_client.clone());
                    for (endpoint, routes) in service_routes {
                        let Some(filter) = schema::method_filter(endpoint.methods()) else {
                            // No routes registered; skip.
                            continue;
                        };
                        let handler = axum::routing::on(filter, proxy.clone());
                        for route in routes {
                            router = router.route(&route, handler.clone());
                        }
                    }
                }
                Ok(router)
            };

        router = register_routes(service_routes.main, router)?;
        fallback_router = register_routes(service_routes.fallback, fallback_router)?;

        router = router.fallback_service(fallback_router);
        router = router.layer(cors.clone());

        Ok(Self {
            shared,
            router: Mutex::new(Some(router)),
        })
    }

    pub fn router(&self) -> axum::Router {
        self.router.lock().unwrap().as_ref().unwrap().clone()
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.shared.auth.as_ref()
    }
}

struct ServiceDirector {
    shared: Arc<SharedGatewayData>,
    dest_base_url: reqwest::Url,
    svc_auth_method: Arc<dyn svcauth::ServiceAuthMethod>,
}

impl Director for Arc<ServiceDirector> {
    type Future = Pin<Box<dyn Future<Output = APIResult<ProxyRequest>> + Send + 'static>>;

    fn direct(self, req: InboundRequest) -> Self::Future {
        Box::pin(async move {
            let mut call_meta = CallMeta::parse_without_caller(&req.headers)?;
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
                svc_auth_method: self.svc_auth_method.as_ref(),
            };

            if let Some(auth_handler) = &self.shared.auth {
                let auth_response = auth_handler.authenticate(&req, call_meta.clone()).await?;
                if let auth::AuthResponse::Authenticated {
                    auth_uid,
                    auth_data,
                } = auth_response
                {
                    desc.auth_user_id = Some(Cow::Owned(auth_uid));
                    desc.auth_data = Some(auth_data);
                }
            }

            let mut proxy = req.build(self.dest_base_url.clone())?;

            desc.add_meta(&mut proxy.headers)
                .map_err(api::Error::internal)?;

            Ok(proxy)
        })
    }
}
