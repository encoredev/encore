use std::collections::HashMap;
use std::fmt::Debug;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex, RwLock};

use anyhow::Context;

use crate::api;
use crate::api::endpoint::{EndpointHandler, SharedEndpointData};
use crate::api::paths::Pather;
use crate::api::reqauth::svcauth;
use crate::api::{paths, reqauth, schema, BoxedHandler, EndpointMap, IntoResponse};
use crate::encore::parser::meta::v1 as meta;
use crate::names::EndpointName;
use crate::trace;

/// An alias for the concrete type of a server handler.
type ServerHandler = ReplaceableHandler<EndpointHandler>;

/// Server is an API server. It serves the registered API endpoints.
///
/// When running tests there's not a single entrypoint, so the server
/// is designed to support incrementally adding endpoints.
///
/// We handle this by registering all handlers with axum up-front, and add
/// the handler once it has been registered.
#[derive(Debug)]
pub struct Server {
    endpoints: Arc<EndpointMap>,

    hosted_endpoints: Mutex<HashMap<EndpointName, ServerHandler>>,

    router: Mutex<Option<axum::Router>>,

    /// Data shared between all endpoints.
    shared: Arc<SharedEndpointData>,

    runtime: tokio::runtime::Handle,
}

impl Server {
    pub fn new(
        endpoints: Arc<EndpointMap>,
        hosted_endpoints: Vec<EndpointName>,
        platform_auth: Arc<reqauth::platform::RequestValidator>,
        inbound_svc_auth: Vec<Arc<dyn svcauth::ServiceAuthMethod>>,
        tracer: trace::Tracer,
        runtime: tokio::runtime::Handle,
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

        let mut handler_map = HashMap::with_capacity(hosted_endpoints.len());
        let path_set = paths::compute(hosted_endpoints.iter().map(|ep| EndpointPathResolver {
            ep: endpoints.get(ep).unwrap().to_owned(),
        }));

        let mut register = |paths: &[(Arc<api::Endpoint>, Vec<String>)],
                            mut router: axum::Router|
         -> axum::Router {
            for (ep, paths) in paths {
                match schema::method_filter(ep.methods()) {
                    Some(filter) => {
                        let server_handler = ServerHandler::default();
                        let handler = axum::routing::on(filter, server_handler.clone());
                        for path in paths {
                            router = router.route(path, handler.clone());
                        }
                        handler_map.insert(ep.name.clone(), server_handler);
                    }
                    None => {
                        log::warn!("no methods for endpoint {}, skipping", ep.name,);
                    }
                }
            }
            router
        };

        if let Some(main_paths) = path_set.main.get(&()) {
            router = register(main_paths, router);
        }
        if let Some(fallback_paths) = path_set.fallback.get(&()) {
            fallback_router = register(fallback_paths, fallback_router);
        }

        // Register our fallback route.
        router = router.fallback_service(fallback_router);

        let shared = Arc::new(SharedEndpointData {
            tracer,
            platform_auth,
            inbound_svc_auth,
        });

        Ok(Self {
            endpoints,
            hosted_endpoints: Mutex::new(handler_map),
            router: Mutex::new(Some(router)),
            shared,
            runtime,
        })
    }

    pub fn router(&self) -> axum::Router {
        self.router.lock().unwrap().as_ref().unwrap().clone()
    }

    /// Registers a handler for the given endpoint.
    /// Reports an error if the handler was not found.
    pub fn register_handler(
        &self,
        endpoint_name: EndpointName,
        handler: Arc<dyn BoxedHandler>,
    ) -> anyhow::Result<()> {
        match self.hosted_endpoints.lock().unwrap().remove(&endpoint_name) {
            None => Ok(()), // anyhow::bail!("no handler found for endpoint: {}", endpoint_name),
            Some(h) => {
                let endpoint = self.endpoints.get(&endpoint_name).unwrap().to_owned();

                let handler = EndpointHandler {
                    endpoint,
                    handler,
                    shared: self.shared.clone(),
                };

                h.set(handler);
                Ok(())
            }
        }
    }

    /// Starts serving the API.
    pub fn start_serving(&self) -> tokio::task::JoinHandle<anyhow::Result<()>> {
        let router = self
            .router
            .lock()
            .unwrap()
            .take()
            .expect("server already started");
        self.runtime.spawn(async move {
            // Determine the listen addr.
            let listen_addr = determine_listen_addr()?;
            log::debug!(addr = listen_addr; "encore api server listening for incoming requests");
            let listener = tokio::net::TcpListener::bind(listen_addr)
                .await
                .context("bind to port")?;

            axum::serve(listener, router).await.context("serve api")?;
            Ok(())
        })
    }
}

struct EndpointPathResolver {
    ep: Arc<api::Endpoint>,
}

impl Pather for EndpointPathResolver {
    type Key = ();
    type Value = Arc<api::Endpoint>;

    fn key(&self) {
        
    }

    fn value(&self) -> Self::Value {
        self.ep.clone()
    }

    fn path(&self) -> &meta::Path {
        &self.ep.path
    }
}

/// A replaceable handler is a handler that can be replaced at runtime.
/// It is used to support incremental registration of endpoints.
#[derive(Clone)]
struct ReplaceableHandler<H> {
    /// Underlying handler. The RwLock is used to be able to inject the underlying handler.
    handler: Arc<RwLock<Option<H>>>,
}

impl<H> Debug for ReplaceableHandler<H> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ReplaceableHandler").finish()
    }
}

impl<H> Default for ReplaceableHandler<H> {
    fn default() -> Self {
        Self::new()
    }
}

impl<H> ReplaceableHandler<H> {
    pub fn new() -> Self {
        Self {
            handler: Arc::new(RwLock::new(None)),
        }
    }

    /// Set sets the handler.
    pub fn set(&self, handler: H) {
        *self.handler.write().unwrap() = Some(handler);
    }
}

impl<H> axum::handler::Handler<(), ()> for ReplaceableHandler<H>
where
    H: axum::handler::Handler<(), ()> + Sync,
{
    type Future = MaybeHandlerFuture<H::Future>;

    fn call(self, req: axum::extract::Request, state: ()) -> Self::Future {
        match self.handler.read().unwrap().as_ref() {
            None => MaybeHandlerFuture { fut: None },
            Some(handler) => MaybeHandlerFuture {
                fut: Some(Box::pin(handler.clone().call(req, state))),
            },
        }
    }
}

/// A MaybeHandlerFuture is a future that may or may not have a future.
/// If there is no future, it returns a 404 response.
struct MaybeHandlerFuture<F> {
    fut: Option<Pin<Box<F>>>,
}

impl<F> Future for MaybeHandlerFuture<F>
where
    F: Future<Output = axum::response::Response> + Send + 'static,
{
    type Output = axum::response::Response;

    fn poll(
        mut self: Pin<&mut Self>,
        cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<axum::response::Response> {
        match self.fut.as_mut() {
            // If we have a future, poll it.
            Some(fut) => fut.as_mut().poll(cx),

            // Otherwise we return a 404 response.
            None => {
                let resp = api::Error {
                    code: api::ErrCode::NotFound,
                    message: "endpoint not found".to_string(),
                    internal_message: Some("no handler registered for endpoint".to_string()),
                    stack: None,
                }
                .into_response();
                std::task::Poll::Ready(resp)
            }
        }
    }
}

fn determine_listen_addr() -> anyhow::Result<String> {
    let candidates = [
        ("ENCORE_LISTEN_ADDR", false),
        ("PORT", true),
        ("HTTP_PORT", true),
    ];
    for c in candidates {
        if let Ok(mut var) = std::env::var(c.0) {
            if c.1 {
                var = format!("0.0.0.0:{}", var);
            }
            return Ok(var);
        }
    }
    anyhow::bail!("unable to determine listen address")
}
