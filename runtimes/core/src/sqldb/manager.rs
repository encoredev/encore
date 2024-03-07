use anyhow::Context;
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
    databases: Arc<HashMap<EncoreName, Arc<Database>>>,
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

    pub fn database(&self, name: &EncoreName) -> Option<Arc<Database>> {
        self.databases.get(name).cloned()
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

            loop {
                let (stream, _) = listener.accept().await?;
                let mgr = manager.clone();
                tokio::spawn(mgr.handle_conn(stream));
            }
        })
    }
}

/// Represents a SQL Database available to the runtime.
pub struct Database {
    name: EncoreName,
    config: Arc<tokio_postgres::Config>,
    tls: postgres_native_tls::MakeTlsConnector,
    proxy_conn_string: String,
    tracer: Tracer,
}

impl Database {
    /// Returns the connection string for connecting to this database via the proxy.
    pub fn proxy_conn_string(&self) -> &str {
        &self.proxy_conn_string
    }

    pub fn name(&self) -> &EncoreName {
        &self.name
    }

    pub(crate) fn config(&self) -> &tokio_postgres::Config {
        &self.config
    }

    pub(crate) fn tls(&self) -> &postgres_native_tls::MakeTlsConnector {
        &self.tls
    }

    pub fn new_pool(&self) -> Pool {
        Pool::new(self, self.tracer.clone())
    }
}

#[derive(Clone)]
struct Bouncer {
    databases: Arc<HashMap<EncoreName, Arc<Database>>>,
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

/// Computes the database configuration for the given clusters.
fn databases_from_cfg(
    clusters: Vec<pb::SqlCluster>,
    creds: &pb::infrastructure::Credentials,
    secrets: &secrets::Manager,
    proxy_port: u16,
    tracer: Tracer,
) -> anyhow::Result<HashMap<EncoreName, Arc<Database>>> {
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
                config.host(&host);
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
            if let Some(server_ca_cert) = &server.server_ca_cert {
                let cert = native_tls::Certificate::from_pem(server_ca_cert.as_bytes())
                    .context("unable to parse server ca certificate")?;
                tls_builder.add_root_certificate(cert);
                config.ssl_mode(tokio_postgres::config::SslMode::Require);
            } else {
                config.ssl_mode(tokio_postgres::config::SslMode::Prefer);
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
                let identity =
                    native_tls::Identity::from_pkcs8(client_cert.cert.as_bytes(), client_key)
                        .context("failed to parse client certificate")?;
                tls_builder.identity(identity);
            }

            let tls = tls_builder
                .build()
                .context("failed to build TLS connector")?;
            let tls = postgres_native_tls::MakeTlsConnector::new(tls);

            let proxy_conn_string = format!(
                "postgresql://encore:password@localhost:{}/{}?sslmode=disable",
                proxy_port, db.encore_name
            );

            let name: EncoreName = db.encore_name.into();
            databases.insert(
                name.clone(),
                Arc::new(Database {
                    name,
                    config: Arc::new(config),
                    tls,
                    proxy_conn_string,
                    tracer: tracer.clone(),
                }),
            );
        }
    }

    Ok(databases)
}
