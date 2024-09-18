use std::borrow::Borrow;
use std::collections::HashSet;
use std::fmt::Display;
use std::hash::Hash;
use std::io::Read;

use std::path::Path;
use std::sync::Arc;

use anyhow::Context;
use base64::Engine;
use duct::cmd;
use prost::Message;

use crate::api::reqauth::platform;
pub use names::{CloudName, EncoreName, EndpointName};

use crate::encore::parser::meta::v1 as metapb;
use crate::encore::runtime::v1 as runtimepb;

pub mod api;
mod base32;
pub mod error;
pub mod log;
pub mod meta;
pub mod model;
mod names;
pub mod proccfg;
pub mod pubsub;
pub mod secrets;
pub mod sqldb;
mod trace;

pub mod encore {
    pub mod runtime {
        pub mod v1 {
            include!(concat!(env!("OUT_DIR"), "/encore.runtime.v1.rs"));
        }
    }

    pub mod parser {
        pub mod meta {
            pub mod v1 {
                include!(concat!(env!("OUT_DIR"), "/encore.parser.meta.v1.rs"));
            }
        }

        pub mod schema {
            pub mod v1 {
                include!(concat!(env!("OUT_DIR"), "/encore.parser.schema.v1.rs"));
            }
        }
    }
}

pub struct RuntimeBuilder {
    cfg: Option<runtimepb::RuntimeConfig>,
    proc_cfg: Option<proccfg::ProcessConfig>,
    md: Option<metapb::Data>,
    err: Option<anyhow::Error>,
    test_mode: bool,
    is_worker: bool,
}

impl Default for RuntimeBuilder {
    fn default() -> Self {
        Self::new()
    }
}

impl RuntimeBuilder {
    pub fn new() -> Self {
        Self {
            cfg: None,
            proc_cfg: None,
            md: None,
            err: None,
            test_mode: false,
            is_worker: false,
        }
    }

    pub fn with_test_mode(mut self, enabled: bool) -> Self {
        self.test_mode = enabled;
        if enabled {
            enable_test_mode().unwrap();
        }
        self
    }

    pub fn with_worker(mut self, enabled: bool) -> Self {
        self.is_worker = enabled;
        self
    }

    pub fn with_runtime_config(mut self, cfg: runtimepb::RuntimeConfig) -> Self {
        self.cfg = Some(cfg);
        self
    }

    pub fn with_proc_config(mut self, proc_cfg: proccfg::ProcessConfig) -> Self {
        self.proc_cfg = Some(proc_cfg);
        self
    }

    pub fn with_runtime_config_from_env(mut self) -> Self {
        if self.err.is_none() {
            match runtime_config_from_env() {
                Ok(cfg) => self.cfg = Some(cfg),
                Err(e) => {
                    self.err = Some(anyhow::Error::new(e).context("unable to parse runtime config"))
                }
            }
            match proc_config_from_env() {
                Ok(cfg) => self.proc_cfg = cfg,
                Err(e) => {
                    self.err = Some(anyhow::Error::new(e).context("unable to parse process config"))
                }
            }
        }
        self
    }

    pub fn with_meta(mut self, md: metapb::Data) -> Self {
        self.md = Some(md);
        self
    }

    pub fn with_meta_autodetect(mut self) -> Self {
        fn auto_detect() -> Result<metapb::Data, anyhow::Error> {
            match meta_from_env() {
                Ok(md) => Ok(md),
                Err(ParseError::EnvNotPresent) => {
                    let path = Path::new("/encore/meta");
                    parse_meta(path).context("unable to parse app metadata file")
                }
                Err(e) => {
                    Err(anyhow::Error::new(e)
                        .context("unable to parse app metadata from environment"))
                }
            }
        }

        if self.err.is_none() {
            match auto_detect() {
                Ok(md) => self.md = Some(md),
                Err(e) => self.err = Some(e),
            }
        }
        self
    }

    pub fn with_meta_from_env(mut self) -> Self {
        if self.err.is_none() {
            match meta_from_env() {
                Ok(md) => self.md = Some(md),
                Err(e) => {
                    self.err = Some(anyhow::Error::new(e).context("unable to parse app metadata"))
                }
            }
        }
        self
    }

    pub fn with_meta_from_path(mut self, meta_path: &Path) -> Self {
        if self.err.is_none() {
            match parse_meta(meta_path) {
                Ok(md) => self.md = Some(md),
                Err(e) => {
                    self.err = Some(anyhow::Error::new(e).context("unable to parse app metadata"))
                }
            }
        }
        self
    }

    pub fn build(self) -> anyhow::Result<Runtime> {
        if let Some(err) = self.err {
            return Err(err);
        }
        let mut cfg = self.cfg.context("runtime config not provided")?;
        let md = self.md.context("metadata not provided")?;
        if let Some(proc_config) = self.proc_cfg {
            proc_config.apply(&mut cfg)?;
        }
        Runtime::new(cfg, md, self.test_mode, self.is_worker)
    }
}

pub struct Runtime {
    md: metapb::Data,
    pubsub: pubsub::Manager,
    secrets: secrets::Manager,
    sqldb: sqldb::Manager,
    api: api::Manager,
    app_meta: meta::AppMeta,
    runtime: tokio::runtime::Runtime,
}

impl Runtime {
    pub fn builder() -> RuntimeBuilder {
        RuntimeBuilder::new()
    }

    pub fn new(
        mut cfg: runtimepb::RuntimeConfig,
        md: metapb::Data,
        testing: bool,
        is_worker: bool,
    ) -> anyhow::Result<Self> {
        // Initialize OpenSSL system root certificates, so that libraries can find them.
        openssl_probe::init_ssl_cert_env_vars();

        let tokio_rt = tokio::runtime::Builder::new_multi_thread()
            .enable_all()
            .build()
            .context("failed to build tokio runtime")?;

        let app_meta = meta::AppMeta::new(&cfg, &md);
        let environment = cfg.environment.take().unwrap_or_default();
        let mut infra = cfg.infra.take().unwrap_or_default();
        let resources = infra.resources.take().unwrap_or_default();
        let creds = infra.credentials.take().unwrap_or_default();
        let encore_platform = cfg.encore_platform.take().unwrap_or_default();

        let mut deployment = cfg.deployment.take().unwrap_or_default();
        let service_discovery = deployment.service_discovery.take().unwrap_or_default();

        let http_client = reqwest::Client::builder()
            .build()
            .context("failed to build http client")?;

        let secrets = secrets::Manager::new(resources.app_secrets);
        let platform_validator = platform::RequestValidator::new(
            &secrets,
            encore_platform.platform_signing_keys.clone(),
        );
        let platform_validator = Arc::new(platform_validator);

        // Set up observability.
        let disable_tracing =
            testing || std::env::var("ENCORE_NOTRACE").is_ok_and(|v| !v.is_empty());
        let tracer = if !disable_tracing {
            let observability = deployment.observability.take().unwrap_or_default();
            let trace_endpoint = observability
                .tracing
                .into_iter()
                .find_map(|p| match p.provider {
                    Some(runtimepb::tracing_provider::Provider::Encore(encore)) => {
                        Some(encore.trace_endpoint)
                    }
                    _ => None,
                })
                .and_then(|ep| match reqwest::Url::parse(&ep) {
                    Ok(ep) => Some(ep),
                    Err(err) => {
                        ::log::warn!("disabling tracing: invalid trace endpoint {}: {}", ep, err);
                        None
                    }
                });

            match trace_endpoint {
                Some(trace_endpoint) => {
                    let config = trace::ReporterConfig {
                        app_id: environment.app_id.clone(),
                        env_id: environment.env_id.clone(),
                        deploy_id: deployment.deploy_id.clone(),
                        app_commit: md.app_revision.clone(),
                        trace_endpoint,
                        platform_validator: platform_validator.clone(),
                    };

                    let (tracer, reporter) = trace::streaming_tracer(http_client.clone(), config);
                    tokio_rt.spawn(reporter.start_reporting());
                    tracer
                }
                None => trace::Tracer::noop(),
            }
        } else {
            trace::Tracer::noop()
        };

        log::set_tracer(tracer.clone());

        let pubsub = pubsub::Manager::new(tracer.clone(), resources.pubsub_clusters, &md);
        let sqldb = sqldb::ManagerConfig {
            clusters: resources.sql_clusters,
            creds: &creds,
            secrets: &secrets,
            tracer: tracer.clone(),
            runtime: tokio_rt.handle().clone(),
        }
        .build()
        .context("unable to initialize sqldb proxy")?;

        let api = api::ManagerConfig {
            meta: &md,
            environment: &environment,
            gateways: resources.gateways,
            hosted_services: deployment.hosted_services,
            hosted_gateway_rids: deployment.hosted_gateways,
            svc_auth_methods: deployment.auth_methods,
            deploy_id: deployment.deploy_id,
            platform: &encore_platform,
            secrets: &secrets,
            service_discovery,
            http_client: http_client.clone(),
            tracer,
            platform_validator,
            pubsub_push_registry: pubsub.push_registry(),
            runtime: tokio_rt.handle().clone(),
            is_worker,
        }
        .build()
        .context("unable to initialize api manager")?;

        let sqldb_handle = sqldb.start_serving();
        tokio_rt.spawn(async move {
            if let Err(err) = sqldb_handle.await {
                ::log::error!("sqldb proxy failed: {err}");
            }
        });

        ::log::debug!("encore runtime successfully initialized");

        Ok(Self {
            md,
            pubsub,
            secrets,
            sqldb,
            api,
            app_meta,
            runtime: tokio_rt,
        })
    }

    #[inline]
    pub fn pubsub(&self) -> &pubsub::Manager {
        &self.pubsub
    }

    #[inline]
    pub fn secrets(&self) -> &secrets::Manager {
        &self.secrets
    }

    #[inline]
    pub fn sqldb(&self) -> &sqldb::Manager {
        &self.sqldb
    }

    #[inline]
    pub fn metadata(&self) -> &metapb::Data {
        &self.md
    }

    #[inline]
    pub fn api(&self) -> &api::Manager {
        &self.api
    }

    #[inline]
    pub fn tokio_handle(&self) -> &tokio::runtime::Handle {
        self.runtime.handle()
    }

    #[inline]
    pub fn run_blocking(&self) {
        self.runtime.block_on(async move {
            let api_handle = self.api().start_serving();

            if let Err(err) = api_handle.await {
                ::log::error!("failed to start serving: {:?}", err);
            }
        });
    }

    #[inline]
    pub fn app_meta(&self) -> &meta::AppMeta {
        &self.app_meta
    }
}

#[derive(Debug)]
enum ParseError {
    EnvNotPresent,
    EnvVar(std::env::VarError),
    Base64(base64::DecodeError),
    Proto(prost::DecodeError),
    IO(std::io::Error),
}

impl Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ParseError::EnvNotPresent => write!(f, "environment variable not present"),
            ParseError::EnvVar(e) => write!(f, "failed to read environment variable: {}", e),
            ParseError::Base64(e) => write!(f, "failed to decode environment variable: {}", e),
            ParseError::Proto(e) => write!(f, "failed to parse environment variable: {}", e),
            ParseError::IO(e) => write!(f, "failed to read file: {}", e),
        }
    }
}

impl std::error::Error for ParseError {}

fn runtime_config_from_env() -> Result<runtimepb::RuntimeConfig, ParseError> {
    let cfg = match std::env::var("ENCORE_RUNTIME_CONFIG") {
        Ok(cfg) => cfg,
        Err(std::env::VarError::NotPresent) => return Err(ParseError::EnvNotPresent),
        Err(e) => return Err(ParseError::EnvVar(e)),
    };

    if cfg.starts_with("gzip:") {
        // Parse the remainder as base64-encoded gzip data.
        let cfg = cfg.as_bytes();
        let cfg = &cfg["gzip:".len()..];
        let gzip_data = base64::engine::general_purpose::STANDARD
            .decode(cfg)
            .map_err(ParseError::Base64)?;

        let mut decoder = flate2::read::GzDecoder::new(&gzip_data[..]);
        let mut raw_data = Vec::new();
        decoder.read_to_end(&mut raw_data).map_err(ParseError::IO)?;
        runtimepb::RuntimeConfig::decode(&raw_data[..]).map_err(ParseError::Proto)
    } else {
        let decoded = base64::engine::general_purpose::STANDARD
            .decode(cfg.as_bytes())
            .map_err(ParseError::Base64)?;
        runtimepb::RuntimeConfig::decode(&decoded[..]).map_err(ParseError::Proto)
    }
}

fn proc_config_from_env() -> Result<Option<proccfg::ProcessConfig>, ParseError> {
    let encoded_config = match std::env::var("ENCORE_PROCESS_CONFIG") {
        Ok(config) => config,
        Err(std::env::VarError::NotPresent) => return Ok(None),
        Err(e) => return Err(ParseError::EnvVar(e)),
    };

    let decoded = base64::engine::general_purpose::STANDARD
        .decode(encoded_config)
        .map_err(ParseError::Base64)?;

    let json_str = String::from_utf8(decoded)
        .map_err(|e| ParseError::IO(std::io::Error::new(std::io::ErrorKind::InvalidData, e)))?;

    let config = serde_json::from_str(&json_str)
        .map_err(|e| ParseError::IO(std::io::Error::new(std::io::ErrorKind::InvalidData, e)))?;

    Ok(Some(config))
}

fn meta_from_env() -> Result<metapb::Data, ParseError> {
    let cfg = match std::env::var("ENCORE_APP_META") {
        Ok(cfg) => cfg,
        Err(std::env::VarError::NotPresent) => {
            // Not present. Check the ENCORE_APP_META_PATH environment variable.
            match std::env::var("ENCORE_APP_META_PATH") {
                Ok(path) => {
                    let path = Path::new(&path);
                    return parse_meta(path);
                }
                Err(std::env::VarError::NotPresent) => return Err(ParseError::EnvNotPresent),
                Err(e) => return Err(ParseError::EnvVar(e)),
            }
        }
        Err(e) => return Err(ParseError::EnvVar(e)),
    };

    if cfg.starts_with("gzip:") {
        // Parse the remainder as base64-encoded gzip data.
        let cfg = cfg.as_bytes();
        let cfg = &cfg["gzip:".len()..];
        let gzip_data = base64::engine::general_purpose::STANDARD
            .decode(cfg)
            .map_err(ParseError::Base64)?;

        let mut decoder = flate2::read::GzDecoder::new(&gzip_data[..]);
        let mut raw_data = Vec::new();
        decoder.read_to_end(&mut raw_data).map_err(ParseError::IO)?;
        metapb::Data::decode(&raw_data[..]).map_err(ParseError::Proto)
    } else {
        let decoded = base64::engine::general_purpose::STANDARD
            .decode(cfg.as_bytes())
            .map_err(ParseError::Base64)?;
        metapb::Data::decode(&decoded[..]).map_err(ParseError::Proto)
    }
}

fn parse_meta(path: &Path) -> Result<metapb::Data, ParseError> {
    let data = std::fs::read(path).map_err(ParseError::IO)?;
    metapb::Data::decode(&data[..]).map_err(ParseError::Proto)
}

fn enable_test_mode() -> Result<(), ParseError> {
    if std::env::var("ENCORE_APP_META").is_ok() {
        return Ok(());
    }

    let out = cmd!("encore", "test", "--prepare")
        .stdout_capture()
        .stderr_capture()
        .run()
        .map_err(ParseError::IO)?;
    if !out.status.success() {
        return Err(ParseError::IO(std::io::Error::new(
            std::io::ErrorKind::Other,
            String::from_utf8(out.stderr).unwrap(),
        )));
    }

    let data = String::from_utf8(out.stdout)
        .map_err(|e| ParseError::IO(std::io::Error::new(std::io::ErrorKind::Other, e)))?;

    for line in data.split('\n') {
        let Some((name, value)) = line.split_once('=') else {
            continue;
        };
        match name {
            "ENCORE_APP_META" => std::env::set_var("ENCORE_APP_META", value),
            "ENCORE_RUNTIME_CONFIG" => std::env::set_var("ENCORE_RUNTIME_CONFIG", value),
            _ => (),
        }
    }

    Ok(())
}

/// Describes which services or gateways are hosted by this server.
#[derive(Debug, Clone)]
pub struct Hosted(pub HashSet<String>);

impl Hosted {
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    pub fn iter(&self) -> impl Iterator<Item = &String> {
        self.0.iter()
    }

    /// Reports whether the given service/entity is hosted by this runtime.
    pub fn contains<Q>(&self, name: &Q) -> bool
    where
        String: Borrow<Q>,
        Q: Eq + Hash + ?Sized,
    {
        self.0.contains(name)
    }
}

impl FromIterator<String> for Hosted {
    fn from_iter<I: IntoIterator<Item = String>>(iter: I) -> Self {
        Self(iter.into_iter().map(Into::into).collect())
    }
}

/// Returns the version of the Encore runtime.
pub fn version() -> &'static str {
    option_env!("ENCORE_VERSION").unwrap_or(env!("CARGO_PKG_VERSION"))
}

/// Returns the git commit used to build the Encore runtime.
pub fn build_commit() -> &'static str {
    env!("ENCORE_BINARY_GIT_HASH")
}
