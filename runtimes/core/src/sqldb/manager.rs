use anyhow::Context;
use std::borrow::Cow;
use std::collections::HashMap;
use std::sync::Arc;
use tokio_postgres::proxy;

use tokio_postgres::proxy::{AcceptConn, AuthMethod, ClientBouncer, RejectConn};

use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::secrets;
use crate::sqldb::Pool;
use crate::trace::Tracer;

pub struct Manager {
    databases: Arc<HashMap<EncoreName, Arc<DatabaseImpl>>>,
    proxy_port: u16,
}

impl Manager {
    pub fn new(
        clusters: Vec<pb::SqlCluster>,
        creds: &pb::infrastructure::Credentials,
        secrets: &secrets::Manager,
        proxy_port: u16,
        tracer: Tracer,
    ) -> anyhow::Result<Self> {
        let databases = databases_from_cfg(clusters, creds, secrets, proxy_port, tracer)
            .context("failed to parse SQL clusters")?;
        let databases = Arc::new(databases);

        Ok(Self {
            databases,
            proxy_port,
        })
    }

    pub fn database(&self, name: &EncoreName) -> Arc<dyn Database> {
        match self.databases.get(name) {
            Some(db) => db.clone(),
            None => {
                let proxy_conn_string = proxy_conn_string(name, self.proxy_port);
                Arc::new(NoopDatabase {
                    name: name.clone(),
                    proxy_conn_string,
                })
            }
        }
    }

    pub fn start_serving(&self) -> tokio::task::JoinHandle<anyhow::Result<()>> {
        let proxy_port = self.proxy_port;
        let manager = proxy::ProxyManager::new(Bouncer {
            databases: self.databases.clone(),
        });

        tokio::spawn(async move {
            let listener = tokio::net::TcpListener::bind(("127.0.0.1", proxy_port))
                .await
                .context("failed to bind proxy listener")?;

            log::debug!("encore runtime database proxy listening for incoming requests");

            loop {
                let (stream, _) = listener.accept().await?;
                let mgr = manager.clone();
                tokio::spawn(mgr.handle_conn(stream));
            }
        })
    }
}

pub trait Database: Send + Sync {
    // The name of the database.
    fn name(&self) -> &EncoreName;

    fn pool_config(&self) -> anyhow::Result<PoolConfig>;
    fn config(&self) -> anyhow::Result<&tokio_postgres::Config>;
    fn tls(&self) -> anyhow::Result<&postgres_native_tls::MakeTlsConnector>;
    fn new_pool(&self) -> anyhow::Result<Pool>;

    /// Returns the connection string for connecting to this database via the proxy.
    fn proxy_conn_string(&self) -> &str;
}

/// Represents a SQL Database available to the runtime.
pub struct DatabaseImpl {
    name: EncoreName,
    config: Arc<tokio_postgres::Config>,
    tls: postgres_native_tls::MakeTlsConnector,
    proxy_conn_string: String,
    tracer: Tracer,

    min_conns: u32,
    max_conns: u32,
}

#[derive(Debug, Clone)]
pub struct PoolConfig {
    pub min_conns: u32,
    pub max_conns: u32,
}

impl Database for DatabaseImpl {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn pool_config(&self) -> anyhow::Result<PoolConfig> {
        Ok(PoolConfig {
            min_conns: self.min_conns,
            max_conns: self.max_conns,
        })
    }

    fn config(&self) -> anyhow::Result<&tokio_postgres::Config> {
        Ok(&self.config)
    }

    fn tls(&self) -> anyhow::Result<&postgres_native_tls::MakeTlsConnector> {
        Ok(&self.tls)
    }

    fn new_pool(&self) -> anyhow::Result<Pool> {
        Pool::new(self, self.tracer.clone())
    }

    fn proxy_conn_string(&self) -> &str {
        &self.proxy_conn_string
    }
}

struct NoopDatabase {
    name: EncoreName,
    proxy_conn_string: String,
}

impl Database for NoopDatabase {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn pool_config(&self) -> anyhow::Result<PoolConfig> {
        anyhow::bail!("this database is not configured for use by this process")
    }

    fn config(&self) -> anyhow::Result<&tokio_postgres::Config> {
        anyhow::bail!("this database is not configured for use by this process")
    }

    fn tls(&self) -> anyhow::Result<&postgres_native_tls::MakeTlsConnector> {
        anyhow::bail!("this database is not configured for use by this process")
    }

    fn new_pool(&self) -> anyhow::Result<Pool> {
        anyhow::bail!("this database is not configured for use by this process")
    }

    fn proxy_conn_string(&self) -> &str {
        // We need to return a valid connection string here,
        // as this is typically called during initialization.
        // The proxy will reject any connections to the database.
        &self.proxy_conn_string
    }
}

#[derive(Clone)]
struct Bouncer {
    databases: Arc<HashMap<EncoreName, Arc<DatabaseImpl>>>,
}

impl ClientBouncer for Bouncer {
    // TODO support TLS
    type Tls = postgres_native_tls::MakeTlsConnector;
    type Future = futures::future::Ready<Result<AcceptConn<Self::Tls>, RejectConn>>;

    fn handle_startup(
        &self,
        info: &postgres_protocol::message::startup::StartupData,
    ) -> Self::Future {
        let resolve = move || {
            let db_name = info
                .parameters
                .get("database")
                .ok_or(RejectConn::UnknownDatabase)?;
            let db_name =
                String::from_utf8(db_name.to_vec()).map_err(|_| RejectConn::UnknownDatabase)?;
            let db = self
                .databases
                .get(&db_name)
                .ok_or(RejectConn::UnknownDatabase)?;

            Ok(AcceptConn {
                auth_method: AuthMethod::Trust,
                tls: db.tls.clone(),
                backend_config: db.config.clone(),
            })
        };
        futures::future::ready(resolve())
    }
}

/// Returns the connection string for connecting to the database via the proxy.
fn proxy_conn_string(db_encore_name: &str, proxy_port: u16) -> String {
    format!(
        "postgresql://encore:password@127.0.0.1:{}/{}?sslmode=disable",
        proxy_port, db_encore_name,
    )
}

/// Computes the database configuration for the given clusters.
fn databases_from_cfg(
    clusters: Vec<pb::SqlCluster>,
    creds: &pb::infrastructure::Credentials,
    secrets: &secrets::Manager,
    proxy_port: u16,
    tracer: Tracer,
) -> anyhow::Result<HashMap<EncoreName, Arc<DatabaseImpl>>> {
    let mut databases = HashMap::new();
    for c in clusters {
        // Get the primary server.
        let server = c
            .servers
            .into_iter()
            .find(|s| s.kind() == pb::ServerKind::Primary);
        let Some(server) = server else {
            log::warn!("no primary server found for cluster {}, skipping", c.rid);
            continue;
        };

        for db in c.databases {
            // Get the read-write pool for this db.
            let pool = db.conn_pools.into_iter().find(|p| !p.is_readonly);
            let Some(pool) = pool else {
                log::warn!(
                    "no read-write pool found for database {}, skipping",
                    db.encore_name
                );
                continue;
            };

            // Get the role to authenticate with.
            let role = creds
                .sql_roles
                .iter()
                .find(|r| r.rid == pool.role_rid)
                .with_context(|| {
                    format!(
                        "no role found with rid {} for database {}",
                        pool.role_rid, db.encore_name
                    )
                })?;

            let mut config = tokio_postgres::Config::new();

            // Add host/port configuration
            if server.host.starts_with('/') {
                // Unix socket
                config.host(&server.host);
            } else if let Some((host, port)) = server.host.split_once(':') {
                config.host(host);
                config.port(port.parse::<u16>().context("invalid port")?);
            } else {
                config.host(&server.host);
                config.port(5432);
            }

            config.user(&role.username);
            if let Some(password) = &role.password {
                let sec = secrets.load(password.clone());
                let password = sec.get().context("failed to resolve password")?;
                config.password(password);
            }

            config.dbname(&db.cloud_name);
            config.application_name("encore");

            let mut tls_builder = native_tls::TlsConnector::builder();
            if let Some(tls_config) = &server.tls_config {
                if let Some(server_ca_cert) = &tls_config.server_ca_cert {
                    let cert = native_tls::Certificate::from_pem(server_ca_cert.as_bytes())
                        .context("unable to parse server ca certificate")?;
                    tls_builder.add_root_certificate(cert);
                    config.ssl_mode(tokio_postgres::config::SslMode::Require);
                } else {
                    config.ssl_mode(tokio_postgres::config::SslMode::Prefer);
                }

                if tls_config.disable_tls_hostname_verification {
                    tls_builder.danger_accept_invalid_hostnames(true);
                }
            } else {
                config.ssl_mode(tokio_postgres::config::SslMode::Disable);
            }

            if let Some(client_cert_rid) = &role.client_cert_rid {
                // Add a client certificate.
                let client_cert = creds
                    .client_certs
                    .iter()
                    .find(|c| c.rid == *client_cert_rid)
                    .with_context(|| {
                        format!(
                            "no client certificate found with rid {} for database {}",
                            client_cert_rid, db.encore_name
                        )
                    })?;

                // Parse the client key secret.
                let client_key = client_cert
                    .key
                    .as_ref()
                    .context("client certificate has no key")?;
                let client_key = secrets.load(client_key.clone());
                let client_key = client_key.get().context("failed to resolve client key")?;

                let client_key = convert_client_key_if_necessary(client_key)
                    .context("failed to convert client key to PKCS#8")?;
                let identity = native_tls::Identity::from_pkcs8(
                    client_cert.cert.as_bytes(),
                    client_key.as_ref(),
                )
                .context("failed to parse client certificate")?;
                tls_builder.identity(identity);
            }

            let tls = tls_builder
                .build()
                .context("failed to build TLS connector")?;
            let tls = postgres_native_tls::MakeTlsConnector::new(tls);

            let proxy_conn_string = proxy_conn_string(&db.encore_name, proxy_port);

            let name: EncoreName = db.encore_name.into();
            databases.insert(
                name.clone(),
                Arc::new(DatabaseImpl {
                    name,
                    config: Arc::new(config),
                    tls,
                    proxy_conn_string,
                    tracer: tracer.clone(),

                    min_conns: pool.min_connections as u32,
                    max_conns: pool.max_connections as u32,
                }),
            );
        }
    }

    Ok(databases)
}

/// Converts the client key from PKCS#1 to PKCS#8 if necessary.
fn convert_client_key_if_necessary(pem: &[u8]) -> anyhow::Result<Cow<'_, [u8]>> {
    let Ok(pem_str) = std::str::from_utf8(pem) else {
        // Assume the key is already in PKCS#8 format.
        return Ok(Cow::Borrowed(pem));
    };
    if !pem_str.starts_with("-----BEGIN RSA PRIVATE KEY-----") {
        // Key is not in PKCS#1 format, assume it's already in PKCS#8 format.
        return Ok(Cow::Borrowed(pem));
    }

    use rsa::{pkcs1::DecodeRsaPrivateKey, pkcs8::EncodePrivateKey};

    let pkey = rsa::RsaPrivateKey::from_pkcs1_pem(pem_str)
        .context("failed to parse PKCS#1 private key")?;
    let pkcs8 = pkey
        .to_pkcs8_pem(rsa::pkcs8::LineEnding::LF)
        .context("failed to convert PKCS#1 private key to PKCS#8")?;
    Ok(Cow::Owned(pkcs8.as_bytes().to_owned()))
}
