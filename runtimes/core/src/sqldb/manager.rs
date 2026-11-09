use anyhow::Context;
use std::borrow::Cow;
use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};
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
    listener: Mutex<Option<std::net::TcpListener>>,
    runtime: tokio::runtime::Handle,
}

pub struct ManagerConfig<'a> {
    pub clusters: Vec<pb::SqlCluster>,
    pub creds: &'a pb::infrastructure::Credentials,
    pub secrets: &'a secrets::Manager,
    pub tracer: Tracer,
    pub runtime: tokio::runtime::Handle,
}

impl ManagerConfig<'_> {
    pub fn build(self) -> anyhow::Result<Manager> {
        // Start listening so we can tell which port it is when we generate configuration.
        let listener =
            std::net::TcpListener::bind("127.0.0.1:0").context("unable to bind to sqldb port")?;
        let proxy_port = listener
            .local_addr()
            .context("unable to get local address")?
            .port();

        let databases = databases_from_cfg(
            self.clusters,
            self.creds,
            self.secrets,
            proxy_port,
            self.tracer,
        )
        .context("failed to parse SQL clusters")?;
        let databases = Arc::new(databases);

        Ok(Manager {
            databases,
            proxy_port,
            runtime: self.runtime,
            listener: Mutex::new(Some(listener)),
        })
    }
}

impl Manager {
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
        let manager = proxy::ProxyManager::new(Bouncer {
            databases: self.databases.clone(),
        });

        let listener = self.listener.lock().unwrap().take();
        let runtime = self.runtime.clone();
        self.runtime.spawn(async move {
            let listener = listener.context("sqldb server already started")?;
            listener
                .set_nonblocking(true)
                .context("unable to set nonblocking")?;
            let listener = tokio::net::TcpListener::from_std(listener)
                .context("unable to convert listener to tokio")?;

            let addr = listener
                .local_addr()
                .map(|a| a.to_string())
                .unwrap_or("unknown".to_string());

            log::debug!(addr=addr; "encore runtime database proxy listening for incoming requests");

            loop {
                let (stream, _) = listener.accept().await?;
                let mgr = manager.clone();
                runtime.spawn(mgr.handle_conn(stream));
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

    /// Returns the connection pool for this database, opening it on first use.
    ///
    /// The pool belongs to the database, not to the caller: every handle to the
    /// same database shares it. Handing out a fresh pool per handle would let a
    /// host that resolves the same database repeatedly — a test runner
    /// re-evaluating the module that declares it, say — accumulate a full set of
    /// connections per resolution, with nothing closing the earlier ones.
    ///
    /// The error is shared rather than cloned because [`anyhow::Error`] isn't
    /// `Clone`, and a pool that failed to build stays failed.
    fn pool(&self) -> Result<&Pool, Arc<anyhow::Error>>;

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

    /// The connection pool, built on first use and shared from then on.
    pool: OnceLock<Result<Pool, Arc<anyhow::Error>>>,
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

    fn pool(&self) -> Result<&Pool, Arc<anyhow::Error>> {
        match self
            .pool
            .get_or_init(|| Pool::new(self, self.tracer.clone()).map_err(Arc::new))
        {
            Ok(pool) => Ok(pool),
            Err(err) => Err(err.clone()),
        }
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

    fn pool(&self) -> Result<&Pool, Arc<anyhow::Error>> {
        Err(Arc::new(anyhow::anyhow!(
            "this database is not configured for use by this process"
        )))
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
    format!("postgresql://encore:password@127.0.0.1:{proxy_port}/{db_encore_name}?sslmode=disable",)
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
                if tls_config.disable_ca_validation {
                    tls_builder.danger_accept_invalid_certs(true);
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

                    pool: OnceLock::new(),
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

#[cfg(test)]
mod test {
    use super::*;

    fn database(dbname: &str, max_conns: u32) -> DatabaseImpl {
        let tls = native_tls::TlsConnector::new().expect("build tls connector");
        let mut config = tokio_postgres::Config::new();
        config
            .host("localhost")
            .port(5432)
            .dbname(dbname)
            .user("encore");
        DatabaseImpl {
            name: dbname.into(),
            config: Arc::new(config),
            tls: postgres_native_tls::MakeTlsConnector::new(tls),
            proxy_conn_string: String::new(),
            tracer: Tracer::noop(),
            min_conns: 0,
            max_conns,
            pool: OnceLock::new(),
        }
    }

    /// Every handle to a database shares one pool.
    ///
    /// Building a pool per handle meant a host that resolves the same database
    /// repeatedly — a test runner re-evaluating the module that declares it —
    /// opened a fresh set of up to `max_conns` connections each time, with
    /// nothing ever closing the earlier ones, until the server refused more.
    #[tokio::test]
    async fn pool_is_shared_across_lookups() {
        let db = database("shared_lookups", 30);
        let first = db.pool().expect("first pool");
        let second = db.pool().expect("second pool");
        assert!(
            std::ptr::eq(first, second),
            "each lookup built its own pool"
        );
    }

    /// A process can accumulate runtimes — in test mode each `Runtime`
    /// construction builds its own, and a test runner that re-evaluates the
    /// declaring module builds one per test file. Each brings its own
    /// `DatabaseImpl`, so pooling per database object still multiplied the
    /// connections; they have to be keyed on the connection instead.
    #[tokio::test]
    async fn connections_are_shared_across_runtimes() {
        let one = database("shared_runtimes", 30);
        let two = database("shared_runtimes", 30);
        assert!(one
            .pool()
            .expect("first pool")
            .shares_connections_with(two.pool().expect("second pool")));
    }

    #[tokio::test]
    async fn connections_are_not_shared_across_databases() {
        let one = database("distinct_a", 30);
        let two = database("distinct_b", 30);
        assert!(!one
            .pool()
            .expect("first pool")
            .shares_connections_with(two.pool().expect("second pool")));
    }

    /// Pool sizing is part of what makes connections interchangeable: a handle
    /// asking for a different ceiling must not be handed a pool built for
    /// another one.
    #[tokio::test]
    async fn connections_are_not_shared_across_pool_sizes() {
        let one = database("distinct_sizes", 10);
        let two = database("distinct_sizes", 30);
        assert!(!one
            .pool()
            .expect("first pool")
            .shares_connections_with(two.pool().expect("second pool")));
    }
}
