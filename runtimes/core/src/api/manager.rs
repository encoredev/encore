use std::collections::HashMap;
use std::future::{Future, IntoFuture};
use std::sync::{Arc, Mutex};

use anyhow::Context;
use axum::response::IntoResponse;

use crate::api::auth::{LocalAuthHandler, RemoteAuthHandler};
use crate::api::call::ServiceRegistry;
use crate::api::gateway::Gateway;
use crate::api::http_server::HttpServer;
use crate::api::paths::Pather;
use crate::api::reqauth::platform;
use crate::api::schema::encoding::EncodingConfig;
use crate::api::schema::JSONPayload;
use crate::api::{
    auth, cors, encore_routes, endpoints_from_meta, jsonschema, paths, reqauth, server, APIResult,
    Endpoint,
};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as runtime;
use crate::trace::Tracer;
use crate::{api, model, pubsub, secrets, EncoreName, EndpointName, Hosted};

use super::encore_routes::healthz;

pub struct ManagerConfig<'a> {
    pub meta: &'a meta::Data,
    pub environment: &'a runtime::Environment,
    pub gateways: Vec<runtime::Gateway>,
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
    pub is_worker: bool,
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

    gateways: HashMap<EncoreName, Gateway>,
}

impl ManagerConfig<'_> {
    pub fn build(mut self) -> anyhow::Result<Manager> {
        let gateway_listen_addr = if !self.hosted_gateway_rids.is_empty() && !self.is_worker {
            // We have a gateway. Have the gateway listen on the provided listen_addr.
            Some(listen_addr())
        } else {
            None
        };

        let api_listener = if !self.hosted_services.is_empty() && !self.is_worker {
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
            self.is_worker,
        )
        .context("unable to create service registry")?;
        let service_registry = Arc::new(service_registry);

        let api_server = if !hosted_services.is_empty() && !self.is_worker {
            let server = server::Server::new(
                endpoints.clone(),
                hosted_endpoints,
                self.platform_validator,
                inbound_svc_auth,
                self.tracer.clone(),
            )
            .context("unable to create API server")?;
            Some(server)
        } else {
            None
        };

        let gateways_by_rid: HashMap<String, runtime::Gateway> =
            HashMap::from_iter(self.gateways.drain(..).map(|gw| (gw.rid.clone(), gw)));

        let hosted_gateways: HashMap<&str, &runtime::Gateway> =
            HashMap::from_iter(self.hosted_gateway_rids.iter().filter_map(|rid| {
                gateways_by_rid
                    .get(rid)
                    .map(|gw| (gw.encore_name.as_str(), gw))
            }));
        let mut gateways = HashMap::new();
        let routes = paths::compute(
            endpoints
                .iter()
                .map(|(_, ep)| RoutePerService(ep.to_owned())),
        );

        for gw in &self.meta.gateways {
            if self.is_worker {
                continue;
            }
            let Some(gw_cfg) = hosted_gateways.get(gw.encore_name.as_str()) else {
                continue;
            };
            let Some(cors_cfg) = &gw_cfg.cors else {
                anyhow::bail!("missing CORS configuration for gateway {}", gw.encore_name);
            };

            let auth_handler = build_auth_handler(
                self.meta,
                gw,
                &service_registry,
                self.http_client.clone(),
                self.tracer.clone(),
            )
            .context("unable to build authenticator")?;

            let meta_headers = cors::MetaHeaders::from_schema(&endpoints, auth_handler.as_ref());
            let cors_config = cors::config(cors_cfg, meta_headers)
                .context("failed to parse CORS configuration")?;

            gateways.insert(
                gw.encore_name.clone().into(),
                Gateway::new(
                    gw.encore_name.clone().into(),
                    service_registry.clone(),
                    routes.clone(),
                    auth_handler,
                    cors_config,
                    healthz_handler.clone(),
                    own_api_address,
                    self.proxied_push_subs.clone(),
                )
                .context("couldn't create gateway")?,
            );
        }

        Ok(Manager {
            gateway_listen_addr,
            api_listener: Mutex::new(api_listener),
            service_registry,
            api_server,
            gateways,
            pubsub_push_registry: self.pubsub_push_registry,
            runtime: self.runtime,
            healthz: healthz_handler,
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

    let registry = builder.build();
    let schema = schema
        .build(&registry)
        .context("unable to build auth handler schema")?;

    // let is_local = hosted_services.contains(&explicit.service_name);
    let is_local = true;
    let name = EndpointName::new(explicit.service_name.clone(), auth.name.clone());

    let auth_handler = if is_local {
        auth::Authenticator::local(
            schema.clone(),
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
            RemoteAuthHandler::new(name, service_registry, http_client)?,
        )?
    };

    Ok(Some(auth_handler))
}

impl Manager {
    pub fn gateway(&self, name: &EncoreName) -> Option<&Gateway> {
        self.gateways.get(name)
    }

    pub fn server(&self) -> Option<&server::Server> {
        self.api_server.as_ref()
    }

    pub fn call<'a>(
        &'a self,
        endpoint_name: &'a EndpointName,
        data: JSONPayload,
        source: Option<&'a model::Request>,
    ) -> impl Future<Output = APIResult<JSONPayload>> + 'a {
        self.service_registry.api_call(endpoint_name, data, source)
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
            }
            .into_response()
        }

        let encore_routes = encore_routes::Desc {
            healthz: self.healthz.clone(),
            push_registry: self.pubsub_push_registry.clone(),
        }
        .router();

        let fallback = axum::Router::new().fallback(fallback);
        let server = HttpServer::new(encore_routes, api, fallback);

        let api_listener = self.api_listener.lock().unwrap().take();
        let gateway_listener = self.gateway_listen_addr.clone();

        // TODO handle multiple gateways
        let gateway = self.gateways.values().next().cloned();

        self.runtime.spawn(async move {
            let gateway_parts = (gateway, gateway_listener);
            let gateway_fut = match gateway_parts {
                (Some(gw), Some(ref ln)) => Some(gw.serve(ln)),
                (Some(_), None) => {
                    ::log::error!("internal encore error: misconfigured api gateway (missing listener), skipping");
                    None
                }
                (None, Some(_)) => {
                    ::log::error!("internal encore error: misconfigured api gateway (missing gateway config), skipping");
                    None
                }
                (None, None) => None,
            };

            let api_fut = match api_listener {
                Some(ln) => {
                    ln
                        .set_nonblocking(true)
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
                    res.context("serve gateway").inspect_err(|err| log::error!("api gateway failed: {:?}", err))?;
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
    "0.0.0.0:0".to_string()
}
