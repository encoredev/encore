use std::collections::HashMap;
use std::sync::Arc;

use anyhow::Context;
use mongodb::options::{ClientOptions, Credential, ServerAddress, Tls, TlsOptions};

use crate::encore::runtime::v1 as pb;
use crate::mongodb::collection::{self, Collection, Transaction};
use crate::mongodb::noop::NoopDatabase;
use crate::names::EncoreName;
use crate::secrets;
use crate::trace::Tracer;

/// Manager manages MongoDB database connections.
pub struct Manager {
    databases: Arc<HashMap<EncoreName, Arc<DatabaseImpl>>>,
}

/// Configuration for creating a Manager.
pub struct ManagerConfig<'a> {
    pub clusters: Vec<pb::MongoCluster>,
    pub creds: &'a pb::infrastructure::Credentials,
    pub secrets: &'a secrets::Manager,
    pub tracer: Tracer,
}

impl ManagerConfig<'_> {
    pub fn build(self) -> anyhow::Result<Manager> {
        let databases = databases_from_cfg(self.clusters, self.creds, self.secrets, self.tracer)
            .context("failed to parse MongoDB clusters")?;
        Ok(Manager {
            databases: Arc::new(databases),
        })
    }
}

impl Manager {
    /// Returns a database by name.
    /// If the database is not configured, returns a NoopDatabase
    /// that errors on all operations.
    pub fn database(&self, name: &EncoreName) -> Arc<dyn Database> {
        match self.databases.get(name) {
            Some(db) => db.clone(),
            None => Arc::new(NoopDatabase::new(name.clone())),
        }
    }
}

/// Trait representing a MongoDB database.
pub trait Database: Send + Sync {
    /// Returns the Encore name of the database.
    fn name(&self) -> &EncoreName;

    /// Returns a handle to the collection with the given name.
    fn collection(&self, name: &str) -> anyhow::Result<Collection>;

    /// Returns the connection to the database, for starting transactions.
    fn connection(&self) -> anyhow::Result<Arc<Connection>>;
}

/// Starts a transaction on the database.
pub async fn begin(db: &dyn Database) -> collection::Result<Transaction> {
    Transaction::start(db.connection()?).await
}

/// Implementation of a configured MongoDB database.
pub struct DatabaseImpl {
    name: EncoreName,
    conn: Arc<Connection>,
}

impl Database for DatabaseImpl {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn collection(&self, name: &str) -> anyhow::Result<Collection> {
        Ok(Collection::new(self.conn.clone(), name.to_string(), None))
    }

    fn connection(&self) -> anyhow::Result<Arc<Connection>> {
        Ok(self.conn.clone())
    }
}

/// The connection to a database, shared by all its collections.
pub struct Connection {
    /// The Encore name of the database.
    pub(super) encore_name: EncoreName,
    pub(super) tracer: Tracer,
    /// The name of the database on the MongoDB server.
    database_name: String,
    options: ClientOptions,
    /// The client is created on first use, as creating it starts
    /// background monitoring of the servers.
    client: tokio::sync::OnceCell<mongodb::Client>,
}

impl Connection {
    /// Returns the client, connecting on first use.
    pub(super) async fn client(&self) -> anyhow::Result<&mongodb::Client> {
        self.client
            .get_or_try_init(|| async {
                mongodb::Client::with_options(self.options.clone())
                    .context("failed to create MongoDB client")
            })
            .await
    }

    /// Returns the database on the server, connecting on first use.
    pub(super) async fn database(&self) -> anyhow::Result<mongodb::Database> {
        Ok(self.client().await?.database(&self.database_name))
    }
}

/// Builds database configurations from proto config.
fn databases_from_cfg(
    clusters: Vec<pb::MongoCluster>,
    creds: &pb::infrastructure::Credentials,
    secrets: &secrets::Manager,
    tracer: Tracer,
) -> anyhow::Result<HashMap<EncoreName, Arc<DatabaseImpl>>> {
    let mut result = HashMap::new();

    // Build role lookup
    let roles: HashMap<&str, &pb::MongoRole> = creds
        .mongo_roles
        .iter()
        .map(|r| (r.rid.as_str(), r))
        .collect();

    for cluster in &clusters {
        if cluster.servers.is_empty() {
            log::warn!(
                "no servers found for MongoDB cluster {}, skipping",
                cluster.rid
            );
            continue;
        }

        for db in &cluster.databases {
            // Get the read-write pool for this db
            let Some(pool) = db.conn_pools.iter().find(|p| !p.is_readonly) else {
                log::warn!(
                    "no read-write pool found for MongoDB database {}, skipping",
                    db.encore_name
                );
                continue;
            };

            // Get the role to authenticate with
            let role = roles.get(pool.role_rid.as_str()).with_context(|| {
                format!(
                    "no role found with rid {} for MongoDB database {}",
                    pool.role_rid, db.encore_name
                )
            })?;

            let options = client_options(cluster, pool, role, secrets)
                .with_context(|| format!("MongoDB database {}", db.encore_name))?;

            let name: EncoreName = db.encore_name.clone().into();
            result.insert(
                name.clone(),
                Arc::new(DatabaseImpl {
                    name,
                    conn: Arc::new(Connection {
                        encore_name: db.encore_name.clone().into(),
                        tracer: tracer.clone(),
                        database_name: db.cloud_name.clone(),
                        options,
                        client: tokio::sync::OnceCell::new(),
                    }),
                }),
            );
        }
    }

    Ok(result)
}

/// Computes the MongoDB client options for a database.
fn client_options(
    cluster: &pb::MongoCluster,
    pool: &pb::MongoConnectionPool,
    role: &pb::MongoRole,
    secrets: &secrets::Manager,
) -> anyhow::Result<ClientOptions> {
    let hosts = cluster
        .servers
        .iter()
        .map(|s| ServerAddress::parse(&s.host).context("invalid MongoDB host"))
        .collect::<anyhow::Result<Vec<_>>>()?;

    let mut opts = ClientOptions::default();
    opts.hosts = hosts;
    opts.repl_set_name = cluster.replica_set.clone();
    opts.direct_connection = Some(cluster.direct_connection);

    // Set the pool size based on the config.
    opts.max_pool_size = Some(30);
    if pool.max_connections > 0 {
        opts.max_pool_size = Some(pool.max_connections as u32);
    }
    if pool.min_connections > 0 {
        opts.min_pool_size = Some(pool.min_connections as u32);
    }

    if !role.username.is_empty() {
        let password = match &role.password {
            Some(secret_data) => {
                let password = secrets.load(secret_data.clone());
                let password = password
                    .get()
                    .context("failed to resolve MongoDB password")?;
                Some(
                    std::str::from_utf8(password)
                        .context("invalid MongoDB password")?
                        .to_string(),
                )
            }
            None => None,
        };
        let mut cred = Credential::default();
        cred.username = Some(role.username.clone());
        cred.password = password;
        cred.source = role.auth_source.clone();
        opts.credential = Some(cred);
    }

    // All servers in a cluster share the same TLS configuration.
    if let Some(tls_config) = cluster.servers[0].tls_config.as_ref() {
        let mut tls = TlsOptions::default();
        if tls_config.disable_ca_validation {
            tls.allow_invalid_certificates = Some(true);
        }
        if tls_config.disable_tls_hostname_verification && !tls_config.disable_ca_validation {
            // The driver's rustls backend cannot skip only the hostname check.
            log::warn!(
                "MongoDB cluster {}: disabling only TLS hostname verification is not supported, ignoring",
                cluster.rid
            );
        }
        if let Some(ca) = &tls_config.server_ca_cert {
            // The driver only reads the CA from a file.
            let path = std::env::temp_dir().join(format!("encore-mongodb-ca-{}.pem", cluster.rid));
            std::fs::write(&path, ca).context("failed to write MongoDB server CA cert")?;
            tls.ca_file_path = Some(path);
        }
        opts.tls = Some(Tls::Enabled(tls));
    }

    Ok(opts)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn cluster(direct: bool, hosts: &[&str]) -> pb::MongoCluster {
        pb::MongoCluster {
            rid: "cluster".into(),
            servers: hosts
                .iter()
                .map(|h| pb::MongoServer {
                    rid: format!("srv-{h}"),
                    host: h.to_string(),
                    kind: pb::ServerKind::Primary as i32,
                    tls_config: None,
                })
                .collect(),
            databases: vec![],
            replica_set: Some("rs0".into()),
            direct_connection: direct,
        }
    }

    #[tokio::test]
    async fn test_client_options() {
        let secrets = secrets::Manager::new(vec![], vec![]).await.unwrap();

        // The local MongoDB server started by the Encore daemon.
        let pool = pb::MongoConnectionPool {
            is_readonly: false,
            role_rid: "role".into(),
            min_connections: 0,
            max_connections: 0,
        };
        let role = pb::MongoRole {
            rid: "role".into(),
            ..Default::default()
        };
        let opts =
            client_options(&cluster(true, &["127.0.0.1:55086"]), &pool, &role, &secrets).unwrap();
        assert_eq!(
            opts.hosts,
            vec![ServerAddress::parse("127.0.0.1:55086").unwrap()]
        );
        assert_eq!(opts.direct_connection, Some(true));
        assert_eq!(opts.repl_set_name.as_deref(), Some("rs0"));
        assert_eq!(opts.max_pool_size, Some(30));
        assert!(opts.credential.is_none());
        assert!(opts.tls.is_none());

        // A self-hosted replica set with credentials.
        let pool = pb::MongoConnectionPool {
            max_connections: 20,
            ..pool
        };
        let role = pb::MongoRole {
            rid: "role".into(),
            username: "user".into(),
            auth_source: Some("admin".into()),
            ..Default::default()
        };
        let opts = client_options(
            &cluster(false, &["mongo-1:27017", "mongo-2:27017"]),
            &pool,
            &role,
            &secrets,
        )
        .unwrap();
        assert_eq!(opts.hosts.len(), 2);
        assert_eq!(opts.direct_connection, Some(false));
        assert_eq!(opts.max_pool_size, Some(20));
        let cred = opts.credential.unwrap();
        assert_eq!(cred.username.as_deref(), Some("user"));
        assert_eq!(cred.source.as_deref(), Some("admin"));
    }
}
