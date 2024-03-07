use std::borrow::Cow;
use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use anyhow::Context;

use crate::api::call::{CallDesc, ServiceRegistry};
use crate::api::gateway::reverseproxy::{Director, InboundRequest, ProxyRequest, ReverseProxy};
use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::{svcauth, CallMeta};
use crate::api::schema::Method;
use crate::api::{auth, schema, APIResult, IntoResponse};
use crate::{api, model, EncoreName};

mod reverseproxy;

pub struct Gateway {
    shared: Arc<SharedGatewayData>,
    router: Mutex<Option<axum::Router>>,
    runtime: tokio::runtime::Handle,
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
        service_routes: HashMap<EncoreName, Vec<Route>>,
        auth_handler: Option<auth::Authenticator>,
        runtime: tokio::runtime::Handle,
    ) -> anyhow::Result<Self> {
        // Register the routes, and track the handlers in a map so we can easily
        // set the request handler when registered.
        let mut router = axum::Router::new();

        async fn fallback(
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

        // Register our fallback route.
        router = router.fallback(fallback);

        let shared = Arc::new(SharedGatewayData {
            name,
            auth: auth_handler,
        });

        for (svc, routes) in service_routes {
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
            for route in routes {
                let Some(filter) = schema::method_filter(route.methods.into_iter()) else {
                    // No routes registered; skip.
                    continue;
                };
                let handler = axum::routing::on(filter, proxy.clone());
                router = router.route(&route.path, handler);
            }
        }

        Ok(Self {
            shared,
            router: Mutex::new(Some(router)),
            runtime,
        })
    }

    pub fn router(&self) -> axum::Router {
        self.router.lock().unwrap().as_ref().unwrap().clone()
    }

    pub fn auth_handler(&self) -> Option<&auth::Authenticator> {
        self.shared.auth.as_ref()
    }

    /// Starts serving the Gateway.
    pub fn start_serving(&self) -> tokio::task::JoinHandle<anyhow::Result<()>> {
        let router = self
            .router
            .lock()
            .unwrap()
            .take()
            .expect("server already started");
        self.runtime.spawn(async move {
            // Determine the listen addr.
            let listen_addr = std::env::var("ENCORE_LISTEN_ADDR")
                .or_else(|_| -> anyhow::Result<_> {
                    let port = std::env::var("PORT").context("PORT env var not set")?;
                    Ok(format!("0.0.0.0:{}", port))
                })
                .context("unable to determine listen address")?;

            let listener = tokio::net::TcpListener::bind(listen_addr)
                .await
                .context("bind to port")?;
            axum::serve(listener, router)
                .await
                .context("serve gateway")?;
            Ok(())
        })
    }
}

struct ServiceDirector {
    shared: Arc<SharedGatewayData>,
    dest_base_url: reqwest::Url,
    svc_auth_method: Arc<dyn svcauth::ServiceAuthMethod>,
}

impl Director for Arc<ServiceDirector> {
    type Future = Pin<Box<dyn Future<Output = APIResult<ProxyRequest>> + Send + 'static>>;

    fn direct(self, mut req: InboundRequest) -> Self::Future {
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
