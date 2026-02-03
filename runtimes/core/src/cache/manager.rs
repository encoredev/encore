use std::collections::HashMap;
use std::sync::Arc;

use anyhow::Context;

use crate::cache::noop::NoopCluster;
use crate::cache::pool::Pool;
use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::secrets;
use crate::trace::Tracer;

/// Manager manages cache cluster connections.
pub struct Manager {
    clusters: Arc<HashMap<EncoreName, Arc<ClusterImpl>>>,
    #[allow(dead_code)]
    tracer: Tracer,
}

/// Configuration for creating a Manager.
pub struct ManagerConfig<'a> {
    pub clusters: Vec<pb::RedisCluster>,
    pub creds: &'a pb::infrastructure::Credentials,
    pub secrets: &'a secrets::Manager,
    pub tracer: Tracer,
}

impl ManagerConfig<'_> {
    pub fn build(self) -> anyhow::Result<Manager> {
        let clusters =
            clusters_from_cfg(self.clusters, self.creds, self.secrets, self.tracer.clone())
                .context("failed to parse Redis clusters")?;

        Ok(Manager {
            clusters: Arc::new(clusters),
            tracer: self.tracer,
        })
    }
}

impl Manager {
    /// Returns a cluster by name.
    /// If the cluster is not configured, returns a NoopCluster that errors on all operations.
    pub fn cluster(&self, name: &EncoreName) -> Arc<dyn Cluster> {
        match self.clusters.get(name) {
            Some(cluster) => cluster.clone(),
            None => Arc::new(NoopCluster::new(name.clone())),
        }
    }
}

/// Trait representing a cache cluster.
pub trait Cluster: Send + Sync {
    /// Returns the name of the cluster.
    fn name(&self) -> &EncoreName;

    /// Creates a new connection pool to this cluster.
    fn pool(&self) -> anyhow::Result<Pool>;
}

/// Implementation of a configured cache cluster.
pub struct ClusterImpl {
    name: EncoreName,
    client: redis::Client,
    key_prefix: Option<String>,
    tracer: Tracer,
    min_conns: u32,
    max_conns: u32,
}

impl ClusterImpl {
    fn new(
        name: EncoreName,
        client: redis::Client,
        key_prefix: Option<String>,
        tracer: Tracer,
        min_conns: u32,
        max_conns: u32,
    ) -> Self {
        Self {
            name,
            client,
            key_prefix,
            tracer,
            min_conns,
            max_conns,
        }
    }
}

impl Cluster for ClusterImpl {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn pool(&self) -> anyhow::Result<Pool> {
        Pool::new(
            self.client.clone(),
            self.key_prefix.clone(),
            self.tracer.clone(),
            self.min_conns,
            self.max_conns,
        )
    }
}

/// Builds cluster configurations from proto config.
fn clusters_from_cfg(
    clusters: Vec<pb::RedisCluster>,
    creds: &pb::infrastructure::Credentials,
    secrets: &secrets::Manager,
    tracer: Tracer,
) -> anyhow::Result<HashMap<EncoreName, Arc<ClusterImpl>>> {
    let mut result = HashMap::new();

    log::debug!("cache: configuring {} Redis clusters", clusters.len());

    // Build role lookup
    let roles: HashMap<&str, &pb::RedisRole> = creds
        .redis_roles
        .iter()
        .map(|r| (r.rid.as_str(), r))
        .collect();

    for cluster in clusters {
        // Get the primary server
        let server = cluster
            .servers
            .iter()
            .find(|s| s.kind() == pb::ServerKind::Primary);

        let Some(server) = server else {
            log::warn!(
                "no primary server found for Redis cluster {}, skipping",
                cluster.rid
            );
            continue;
        };

        // Process each database in the cluster
        for db in &cluster.databases {
            // Get the read-write pool for this db
            let pool = db.conn_pools.iter().find(|p| !p.is_readonly);
            let Some(pool) = pool else {
                log::warn!(
                    "no read-write pool found for Redis database {}, skipping",
                    db.encore_name
                );
                continue;
            };

            // Get the role to authenticate with
            let role = roles.get(pool.role_rid.as_str()).with_context(|| {
                format!(
                    "no role found with rid {} for Redis database {}",
                    pool.role_rid, db.encore_name
                )
            })?;

            // Build connection URL
            let conn_url = build_connection_url(server, db, role, secrets)?;
            let client = redis::Client::open(conn_url).context("failed to create Redis client")?;

            let name: EncoreName = db.encore_name.clone().into();
            log::debug!("cache: configured Redis database {}", db.encore_name);
            result.insert(
                name.clone(),
                Arc::new(ClusterImpl::new(
                    name,
                    client,
                    db.key_prefix.clone(),
                    tracer.clone(),
                    pool.min_connections as u32,
                    pool.max_connections as u32,
                )),
            );
        }
    }

    log::debug!("cache: configured {} Redis databases total", result.len());
    Ok(result)
}

/// Builds a Redis connection URL from configuration.
fn build_connection_url(
    server: &pb::RedisServer,
    db: &pb::RedisDatabase,
    role: &pb::RedisRole,
    secrets: &secrets::Manager,
) -> anyhow::Result<String> {
    use pb::redis_role::Auth;

    // Parse host and port
    let (host, port) = if server.host.starts_with('/') {
        // Unix socket
        return build_unix_socket_url(&server.host, db.database_idx, role, secrets);
    } else if let Some((h, p)) = server.host.split_once(':') {
        (h.to_string(), p.parse::<u16>().context("invalid port")?)
    } else {
        (server.host.clone(), 6379)
    };

    // Determine if TLS is enabled
    let use_tls = server.tls_config.is_some();
    let scheme = if use_tls { "rediss" } else { "redis" };

    // Build auth portion
    let auth = match &role.auth {
        Some(Auth::AuthString(secret_data)) => {
            let password = secrets.load(secret_data.clone());
            let password = password
                .get()
                .context("failed to resolve Redis auth string")?;
            let password_str = std::str::from_utf8(password).context("invalid auth string")?;
            format!(":{password_str}@")
        }
        Some(Auth::Acl(acl)) => {
            let password = acl
                .password
                .as_ref()
                .context("ACL auth requires password")?;
            let password = secrets.load(password.clone());
            let password = password.get().context("failed to resolve Redis password")?;
            let password_str = std::str::from_utf8(password).context("invalid password")?;
            format!("{}:{password_str}@", acl.username)
        }
        None => String::new(),
    };

    Ok(format!(
        "{scheme}://{auth}{host}:{port}/{db_idx}",
        db_idx = db.database_idx
    ))
}

/// Builds a Unix socket connection URL.
fn build_unix_socket_url(
    socket_path: &str,
    db_idx: i32,
    role: &pb::RedisRole,
    secrets: &secrets::Manager,
) -> anyhow::Result<String> {
    use pb::redis_role::Auth;

    // Build auth portion for query string
    let auth_params = match &role.auth {
        Some(Auth::AuthString(secret_data)) => {
            let password = secrets.load(secret_data.clone());
            let password = password
                .get()
                .context("failed to resolve Redis auth string")?;
            let password_str = std::str::from_utf8(password).context("invalid auth string")?;
            format!("&password={}", urlencoding::encode(password_str))
        }
        Some(Auth::Acl(acl)) => {
            let password = acl
                .password
                .as_ref()
                .context("ACL auth requires password")?;
            let password = secrets.load(password.clone());
            let password = password.get().context("failed to resolve Redis password")?;
            let password_str = std::str::from_utf8(password).context("invalid password")?;
            format!(
                "&username={}&password={}",
                urlencoding::encode(&acl.username),
                urlencoding::encode(password_str)
            )
        }
        None => String::new(),
    };

    Ok(format!(
        "redis+unix://{}?db={db_idx}{auth_params}",
        socket_path
    ))
}
