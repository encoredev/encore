use std::collections::HashMap;
use std::future::Future;
use std::sync::{Arc, Mutex};

use anyhow::Context;

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
    Endpoint, IntoResponse,
};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as runtime;
use crate::trace::Tracer;
use crate::{api, model, pubsub, secrets, EncoreName, EndpointName, Hosted};

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
}

pub struct Manager {
    app_revision: String,
    deploy_id: String,
    listener: Mutex<Option<std::net::TcpListener>>,
    service_registry: Arc<ServiceRegistry>,
    pubsub_push_registry: pubsub::PushHandlerRegistry,

    api_server: Option<server::Server>,
    gateways: HashMap<EncoreName, Arc<Gateway>>,
    runtime: tokio::runtime::Handle,
}

impl ManagerConfig<'_> {
    pub fn build(mut self) -> anyhow::Result<Manager> {
        let listener = {
            let addr = listen_addr();
            std::net::TcpListener::bind(addr).context("unable to bind to port")?
        };

        let own_address = listener
            .local_addr()
            .context("unable to determine listen address")?
            .to_string();

        let hosted_services = Hosted::from_iter(self.hosted_services.into_iter().map(|s| s.name));
        let (endpoints, hosted_endpoints) = endpoints_from_meta(&self.meta, &hosted_services)
            .context("unable to compute endpoints descriptions")?;

        let inbound_svc_auth = {
            let mut entries = Vec::with_capacity(self.svc_auth_methods.len());
            for auth in self.svc_auth_methods.drain(..) {
                let auth_method =
                    reqauth::service_auth_method(&self.secrets, &self.environment, auth)
                        .context("unable to initialize service auth method")?;
                entries.push(auth_method);
            }
            entries
        };

        let service_registry = ServiceRegistry::new(
            &self.secrets,
            endpoints.clone(),
            &self.environment,
            self.service_discovery,
            &own_address,
            &inbound_svc_auth,
            &hosted_services,
            self.deploy_id.clone(),
            self.http_client.clone(),
            self.tracer.clone(),
        )
        .context("unable to create service registry")?;
        let service_registry = Arc::new(service_registry);

        let api_server = if !hosted_services.is_empty() {
            let server = server::Server::new(
                endpoints.clone(),
                hosted_endpoints,
                self.platform_validator,
                inbound_svc_auth,
                self.tracer.clone(),
                self.runtime.clone(),
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
        let gateways = {
            let routes = paths::compute(
                endpoints
                    .iter()
                    .map(|(_, ep)| RoutePerService(ep.to_owned())),
            );
            let mut gateways = HashMap::new();
            for gw in &self.meta.gateways {
                let Some(gw_cfg) = hosted_gateways.get(gw.encore_name.as_str()) else {
                    continue;
                };
                let Some(cors_cfg) = &gw_cfg.cors else {
                    anyhow::bail!("missing CORS configuration for gateway {}", gw.encore_name);
                };

                let auth_handler = build_auth_handler(
                    &self.meta,
                    gw,
                    &service_registry,
                    self.http_client.clone(),
                    self.tracer.clone(),
                )
                .context("unable to build authenticator")?;

                let meta_headers =
                    cors::MetaHeaders::from_schema(&endpoints, auth_handler.as_ref());
                let cors = cors::layer(cors_cfg, meta_headers)
                    .context("failed to parse CORS configuration")?;

                let gateway = Gateway::new(
                    gw.encore_name.clone().into(),
                    self.http_client.clone(),
                    service_registry.clone(),
                    routes.clone(),
                    auth_handler,
                    cors,
                )
                .context("unable to create gateway")?;
                gateways.insert(gw.encore_name.clone().into(), Arc::new(gateway));
            }
            gateways
        };

        Ok(Manager {
            app_revision: self.meta.app_revision.clone(),
            deploy_id: self.deploy_id,
            listener: Mutex::new(Some(listener)),
            service_registry,
            api_server,
            gateways,
            pubsub_push_registry: self.pubsub_push_registry,
            runtime: self.runtime,
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

        let schema = cfg
            .compute(auth_params)
            .context("unable to compute auth handler schema")?;
        schema
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
    pub fn gateway(&self, name: EncoreName) -> Option<Arc<Gateway>> {
        self.gateways.get(&name).cloned()
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
        let gateway = self.gateways.values().next().map(|gw| gw.router());
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
            healthz: encore_routes::healthz::Handler {
                app_revision: self.app_revision.clone(),
                // Remove the "roll_" prefix from the deploy_id.
                deploy_id: self
                    .deploy_id
                    .strip_prefix("roll_")
                    .unwrap_or(&self.deploy_id)
                    .to_string(),
            },
            push_registry: self.pubsub_push_registry.clone(),
        }
        .router();

        let fallback = axum::Router::new().fallback(fallback);
        let server = HttpServer::new(encore_routes, gateway, api, fallback);

        let listener = self.listener.lock().unwrap().take();
        self.runtime.spawn(async move {
            let listener = listener.context("server already started")?;
            listener
                .set_nonblocking(true)
                .context("unable to set nonblocking")?;
            let listener = tokio::net::TcpListener::from_std(listener)
                .context("unable to convert listener to tokio")?;
            axum::serve(listener, server)
                .await
                .context("serve api")
                .inspect_err(|err| log::error!("api server failed: {:?}", err))?;
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
