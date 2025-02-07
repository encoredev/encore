use std::collections::HashMap;
use std::future::{Future, IntoFuture};
use std::sync::{Arc, Mutex};

use anyhow::Context;

use crate::api::auth::{LocalAuthHandler, RemoteAuthHandler};
use crate::api::call::ServiceRegistry;
use crate::api::gateway::GatewayServer;
use crate::api::http_server::HttpServer;
use crate::api::paths::Pather;
use crate::api::reqauth::platform;
use crate::api::schema::encoding::EncodingConfig;
use crate::api::schema::JSONPayload;
use crate::api::{
    auth, cors, encore_routes, endpoints_from_meta, jsonschema, paths, reqauth, server, APIResult,
    Endpoint, ToResponse,
};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as runtime;
use crate::trace::Tracer;
use crate::{api, model, pubsub, secrets, EncoreName, EndpointName, Hosted};

use super::encore_routes::healthz;
use super::gateway::Gateway;
use super::paths::PathSet;
use super::websocket_client::WebSocketClient;
use super::ResponsePayload;

pub struct ManagerConfig<'a> {
    pub meta: &'a meta::Data,
    pub environment: &'a runtime::Environment,
    pub gateways: Vec<runtime::Gateway>,
    pub internal_gateway: Option<runtime::Gateway>,
    pub hosted_services: Vec<runtime::HostedService>,
    pub hosted_gateway_rids: Vec<String>,
    pub svc_auth_methods: Vec<runtime::ServiceAuth>,
    pub deploy_id: String,
    pub platform: &'a runtime::EncorePlatform,
    pub secrets: &'a secrets::Manager,
    pub service_discovery: runtime::ServiceDiscovery,
    pub http_client: reqwest::Client,
    pub tracer: Tracer,
    pub platform_validator: Arc<platform::RequestValidator>,
    pub pubsub_push_registry: pubsub::PushHandlerRegistry,
    pub runtime: tokio::runtime::Handle,
    pub testing: bool,
    pub proxied_push_subs: HashMap<String, EncoreName>,
}

pub struct Manager {
    gateway_listen_addr: Option<String>,
    api_listener: Mutex<Option<std::net::TcpListener>>,
    service_registry: Arc<ServiceRegistry>,
    healthz: healthz::Handler,
    pubsub_push_registry: pubsub::PushHandlerRegistry,

    api_server: Option<server::Server>,
    runtime: tokio::runtime::Handle,

    gateway_server: Option<GatewayServer>,
    testing: bool,
}

impl ManagerConfig<'_> {
    pub fn build(mut self) -> anyhow::Result<Manager> {
        let gateway_listen_addr = if !self.hosted_gateway_rids.is_empty() {
            // We have a gateway. Have the gateway listen on the provided listen_addr.
            Some(listen_addr())
        } else {
            None
        };

        let api_listener = if !self.hosted_services.is_empty() {
            // If we already have a gateway, it's listening on the externally provided listen addr.
            // Use a random local port in that case.
            let addr = if gateway_listen_addr.is_some() {
                "127.0.0.1:0".to_string()
            } else {
                listen_addr()
            };
            let ln = std::net::TcpListener::bind(addr).context("unable to bind to port")?;
            Some(ln)
        } else {
            None
        };

        // Get the local address for use by the service registry
        // for calling services hosted by this instance.
        let own_api_address = match api_listener {
            None => None,
            Some(ref ln) => {
                let addr = ln
                    .local_addr()
                    .context("unable to determine listen address")?;
                Some(addr)
            }
        };

        let healthz_handler = encore_routes::healthz::Handler {
            app_revision: self.meta.app_revision.clone(),
            // Remove the "roll_" prefix from the deploy_id.
            deploy_id: self
                .deploy_id
                .strip_prefix("roll_")
                .unwrap_or(&self.deploy_id)
                .to_string(),
        };

        let hosted_services = Hosted::from_iter(self.hosted_services.into_iter().map(|s| s.name));
        let (endpoints, hosted_endpoints) = endpoints_from_meta(self.meta, &hosted_services)
            .context("unable to compute endpoints descriptions")?;

        let inbound_svc_auth = {
            let mut entries = Vec::with_capacity(self.svc_auth_methods.len());
            for auth in self.svc_auth_methods.drain(..) {
                let auth_method =
                    reqauth::service_auth_method(self.secrets, self.environment, auth)
                        .context("unable to initialize service auth method")?;
                entries.push(auth_method);
            }
            entries
        };

        let service_registry = ServiceRegistry::new(
            self.secrets,
            endpoints.clone(),
            self.environment,
            self.service_discovery,
            own_api_address
                .as_ref()
                .map(|addr| addr.to_string())
                .as_deref(),
            &inbound_svc_auth,
            &hosted_services,
            self.deploy_id.clone(),
            self.http_client.clone(),
            self.tracer.clone(),
        )
        .context("unable to create service registry")?;
        let service_registry = Arc::new(service_registry);

        let gateways_by_rid: HashMap<String, runtime::Gateway> =
            HashMap::from_iter(self.gateways.drain(..).map(|gw| (gw.rid.clone(), gw)));

        let hosted_gateways: HashMap<&str, &runtime::Gateway> =
            HashMap::from_iter(self.hosted_gateway_rids.iter().filter_map(|rid| {
                gateways_by_rid
                    .get(rid)
                    .map(|gw| (gw.encore_name.as_str(), gw))
            }));

        let mut auth_data_schemas = HashMap::new();
        let mut gateway_server = GatewayServer::new(
            service_registry.clone(),
            healthz_handler.clone(),
            own_api_address,
            self.proxied_push_subs.clone(),
            self.platform_validator.clone(),
        );

        if let Some(gw_cfg) = self.internal_gateway {
            // internal gateway exposes all routes
            let routes = paths::compute(
                endpoints
                    .iter()
                    .map(|(_, ep)| RoutePerService(ep.to_owned())),
            );

            let gw = build_gateway(
                self.meta,
                &gw_cfg,
                service_registry.clone(),
                endpoints.clone(),
                routes,
                self.http_client.clone(),
                self.tracer.clone(),
            )?;

            auth_data_schemas.insert(
                gw_cfg.encore_name,
                gw.auth_handler().map(|ah| ah.auth_data().clone()),
            );

            gateway_server.set_internal_gateway(gw);
        }

        for (name, gw_cfg) in hosted_gateways {
            let routes = paths::compute(
                endpoints
                    .iter()
                    .filter(|(_, ep)| ep.exposed.contains(name))
                    .map(|(_, ep)| RoutePerService(ep.to_owned())),
            );

            let gw = build_gateway(
                self.meta,
                gw_cfg,
                service_registry.clone(),
                endpoints.clone(),
                routes,
                self.http_client.clone(),
                self.tracer.clone(),
            )?;

            auth_data_schemas.insert(
                name.to_string(),
                gw.auth_handler().map(|ah| ah.auth_data().clone()),
            );

            gateway_server
                .add_gateway(gw)
                .context("couldn't create gateway")?;
        }

        let api_server = if !hosted_services.is_empty() {
            let server = server::Server::new(
                endpoints.clone(),
                hosted_endpoints,
                self.platform_validator,
                inbound_svc_auth,
                self.tracer.clone(),
                auth_data_schemas,
            )
            .context("unable to create API server")?;
            Some(server)
        } else {
            None
        };

        let gateway_server = if self.meta.gateways.is_empty() {
            None
        } else {
            Some(gateway_server)
        };

        Ok(Manager {
            gateway_listen_addr,
            api_listener: Mutex::new(api_listener),
            service_registry,
            api_server,
            gateway_server,
            pubsub_push_registry: self.pubsub_push_registry,
            runtime: self.runtime,
            healthz: healthz_handler,
            testing: self.testing,
        })
    }
}

#[derive(Debug)]
struct RoutePerService(Arc<Endpoint>);

impl Pather for RoutePerService {
    type Key = EncoreName;
    type Value = Arc<Endpoint>;

    fn key(&self) -> Self::Key {
        self.0.name.service().into()
    }
    fn value(&self) -> Self::Value {
        self.0.clone()
    }
    fn path(&self) -> &meta::Path {
        &self.0.path
    }
}

fn build_gateway(
    meta: &meta::Data,
    gw_cfg: &runtime::Gateway,
    service_registry: Arc<ServiceRegistry>,
    endpoints: Arc<HashMap<EndpointName, Arc<Endpoint>>>,
    routes: PathSet<EncoreName, Arc<Endpoint>>,
    http_client: reqwest::Client,
    tracer: Tracer,
) -> anyhow::Result<Gateway> {
    let gw_meta = meta
        .gateways
        .iter()
        .find(|gw| gw.encore_name == gw_cfg.encore_name)
        .ok_or_else(|| {
            anyhow::anyhow!(
                "missing meta configuration for gateway {}",
                gw_cfg.encore_name
            )
        })?;

    let cors_cfg = gw_cfg.cors.as_ref().ok_or_else(|| {
        anyhow::anyhow!(
            "missing CORS configuration for gateway {}",
            gw_cfg.encore_name
        )
    })?;

    let auth_handler = build_auth_handler(meta, gw_meta, &service_registry, http_client, tracer)
        .context("unable to build authenticator")?;

    let meta_headers =
        cors::MetaHeaders::from_schema(&gw_cfg.encore_name, &endpoints, auth_handler.as_ref());
    let cors_config =
        cors::config(cors_cfg, meta_headers).context("failed to parse CORS configuration")?;

    Gateway::new(
        gw_cfg.encore_name.clone().into(),
        routes,
        auth_handler,
        cors_config,
        gw_cfg.hostnames.clone(),
    )
}

fn build_auth_handler(
    meta: &meta::Data,
    gw: &meta::Gateway,
    service_registry: &ServiceRegistry,
    http_client: reqwest::Client,
    tracer: Tracer,
) -> anyhow::Result<Option<auth::Authenticator>> {
    let Some(explicit) = &gw.explicit else {
        return Ok(None);
    };
    let Some(auth) = &explicit.auth_handler else {
        return Ok(None);
    };

    let auth_params = auth.params.as_ref().context("missing auth params")?;
    let mut builder = jsonschema::Builder::new(meta);
    let schema = {
        let mut cfg = EncodingConfig {
            meta,
            registry_builder: &mut builder,
            default_loc: None,
            rpc_path: None,
            supports_body: false,
            supports_query: true,
            supports_header: true,
            supports_path: false,
        };

        cfg.compute(auth_params)
            .context("unable to compute auth handler schema")?
    };

    let auth_data_schema_idx =
        builder.register_type(auth.auth_data.as_ref().context("missing auth data")?)?;

    let registry = builder.build();
    let schema = schema
        .build(&registry)
        .context("unable to build auth handler schema")?;

    // let is_local = hosted_services.contains(&explicit.service_name);
    let is_local = true;
    let name = EndpointName::new(explicit.service_name.clone(), auth.name.clone());

    let auth_data = registry.schema(auth_data_schema_idx);
    let auth_handler = if is_local {
        auth::Authenticator::local(
            schema.clone(),
            auth_data,
            LocalAuthHandler {
                name,
                schema,
                handler: Default::default(),
                tracer,
            },
        )?
    } else {
        auth::Authenticator::remote(
            schema,
            auth_data.clone(),
            RemoteAuthHandler::new(name, service_registry, http_client, auth_data)?,
        )?
    };

    Ok(Some(auth_handler))
}

impl Manager {
    pub fn gateway(&self, name: &EncoreName) -> Option<&Arc<Gateway>> {
        self.gateway_server
            .as_ref()
            .and_then(|gws| gws.gateway_by_name(name))
    }

    pub fn server(&self) -> Option<&server::Server> {
        self.api_server.as_ref()
    }

    pub fn call(
        &self,
        target: EndpointName,
        data: JSONPayload,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = APIResult<ResponsePayload>> + 'static {
        self.service_registry.api_call(target, data, source)
    }

    pub fn stream(
        &self,
        endpoint_name: EndpointName,
        data: JSONPayload,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = APIResult<WebSocketClient>> + 'static {
        self.service_registry
            .connect_stream(endpoint_name, data, source)
    }

    /// Starts serving the API.
    pub fn start_serving(&self) -> tokio::task::JoinHandle<anyhow::Result<()>> {
        let api = self.api_server.as_ref().map(|srv| srv.router());

        async fn fallback(
            req: axum::http::Request<axum::body::Body>,
        ) -> axum::response::Response<axum::body::Body> {
            api::Error {
                code: api::ErrCode::NotFound,
                message: "endpoint not found".to_string(),
                internal_message: Some(format!("no such endpoint exists: {}", req.uri().path())),
                stack: None,
                details: None,
            }
            .to_response(None)
        }

        let encore_routes = encore_routes::Desc {
            healthz: self.healthz.clone(),
            push_registry: self.pubsub_push_registry.clone(),
        }
        .router();

        let fallback = axum::Router::new().fallback(fallback);
        let server = HttpServer::new(encore_routes, api, fallback);

        let api_listener = self.api_listener.lock().unwrap().take();

        let testing = self.testing;
        let gateway_server = self.gateway_server.clone();
        let gateway_listener = self.gateway_listen_addr.clone();

        self.runtime.spawn(async move {
            let gateway_parts = (gateway_server, gateway_listener);
            let gateway_fut = match gateway_parts {
                (Some(gws), Some(ref ln)) => {
                    if testing {
                        // No need to run gateway server in tests
                        None
                    } else {
                        log::debug!(addr = ln; "gateway listening for incoming requests");
                        Some(gws.serve(ln))
                    }
                },
                (Some(_), None) => {
                    log::error!("internal encore error: misconfigured gateway server (missing listener), skipping");
                    None
                },
                (None, Some(_)) => {
                    log::error!("internal encore error: misconfigured gateway server (missing gateway config), skipping");
                    None
                },
                (None, None) => None,
            };

            let api_fut = match api_listener {
                Some(ln) => {
                    let addr = ln
                        .local_addr()
                        .map(|addr| addr.to_string())
                        .unwrap_or_default();
                    log::debug!(addr = addr; "api server listening for incoming requests");

                    ln.set_nonblocking(true)
                        .context("unable to set nonblocking")?;
                    let axum_listener = tokio::net::TcpListener::from_std(ln)
                        .context("unable to convert listener to tokio")?;
                    let fut = axum::serve(axum_listener, server).into_future();
                    Some(fut)
                }
                None => None,
            };

            tokio::select! {
                res = async { gateway_fut.unwrap().await }, if gateway_fut.is_some() => {
                    res.context("serve gateway").inspect_err(|err| log::error!("gateway server failed: {:?}", err))?;
                },
                res = async { api_fut.unwrap().await }, if api_fut.is_some() => {
                    res.context("serve api").inspect_err(|err| log::error!("api server failed: {:?}", err))?;
                },
                else => {
                    // Nothing to serve.
                    ::log::debug!("no api server or gateway to serve");
                }
            };
            Ok(())
        })
    }
}

fn listen_addr() -> String {
    if let Ok(addr) = std::env::var("ENCORE_LISTEN_ADDR") {
        return addr;
    }
    if let Ok(port) = std::env::var("PORT") {
        return format!("0.0.0.0:{}", port);
    }
    "0.0.0.0:8080".to_string()
}
