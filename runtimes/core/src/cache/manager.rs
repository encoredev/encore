use std::collections::HashMap;
use std::sync::Arc;

use anyhow::Context;
use bb8_redis::redis;
use redis::{ConnectionAddr, IntoConnectionInfo, RedisConnectionInfo, TlsCertificates};

use crate::cache::memcluster::MemoryCluster;
use crate::cache::noop::NoopCluster;
use crate::cache::pool::Pool;
use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::secrets;
use crate::trace::Tracer;

/// Manager manages cache cluster connections.
pub struct Manager {
    clusters: Arc<HashMap<EncoreName, Arc<ClusterImpl>>>,
    /// Memory cluster for Encore Cloud fallback.
    memory_cluster: Option<Arc<MemoryCluster>>,
}

/// Configuration for creating a Manager.
pub struct ManagerConfig<'a> {
    pub clusters: Vec<pb::RedisCluster>,
    pub creds: &'a pb::infrastructure::Credentials,
    pub secrets: &'a secrets::Manager,
    pub tracer: Tracer,
    pub cloud: pb::environment::Cloud,
    pub testing: bool,
}

impl ManagerConfig<'_> {
    pub fn build(self) -> anyhow::Result<Manager> {
        let clusters =
            clusters_from_cfg(self.clusters, self.creds, self.secrets, self.tracer.clone())
                .context("failed to parse Redis clusters")?;

        // Use in-memory cache for testing and Encore Cloud.
        let memory_cluster = if self.testing || self.cloud == pb::environment::Cloud::Encore {
            log::debug!("cache: enabling in-memory cache");
            Some(Arc::new(MemoryCluster::new(self.tracer.clone())))
        } else {
            None
        };

        Ok(Manager {
            clusters: Arc::new(clusters),
            memory_cluster,
        })
    }
}

impl Manager {
    /// Returns a cluster by name.
    /// If the cluster is not configured and running in Encore Cloud,
    /// returns an in-memory cache cluster. Otherwise, returns a NoopCluster
    /// that errors on all operations.
    pub fn cluster(&self, name: &EncoreName) -> Arc<dyn Cluster> {
        match self.clusters.get(name) {
            Some(cluster) => cluster.clone(),
            None => {
                // If we're running in Encore Cloud, use the in-memory cluster fallback.
                // This matches the Go runtime behavior where miniredis is used
                // when a cluster isn't explicitly configured.
                if let Some(mem_cluster) = &self.memory_cluster {
                    log::debug!(
                        "cache: using in-memory fallback for unconfigured cluster {}",
                        name
                    );
                    mem_cluster.clone()
                } else {
                    Arc::new(NoopCluster::new(name.clone()))
                }
            }
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
            let Some(pool) = db.conn_pools.iter().find(|p| !p.is_readonly) else {
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

            // Build connection info and client
            let client = build_redis_client(server, db, role, secrets)?;

            let name: EncoreName = db.encore_name.clone().into();
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

    Ok(result)
}

/// Builds a Redis client with proper TLS configuration.
fn build_redis_client(
    server: &pb::RedisServer,
    db: &pb::RedisDatabase,
    role: &pb::RedisRole,
    secrets: &secrets::Manager,
) -> anyhow::Result<redis::Client> {
    use pb::redis_role::Auth;

    // Parse host and port
    let (host, port) = if server.host.starts_with('/') {
        // Unix socket - use URL-based connection
        let url = build_unix_socket_url(&server.host, db.database_idx, role, secrets)?;
        return redis::Client::open(url).context("failed to create Redis client");
    } else if let Some((h, p)) = server.host.split_once(':') {
        (h.to_string(), p.parse::<u16>().context("invalid port")?)
    } else {
        (server.host.clone(), 6379)
    };

    let (username, password) = match &role.auth {
        Some(Auth::AuthString(secret_data)) => {
            let password = secrets.load(secret_data.clone());
            let password = password
                .get()
                .context("failed to resolve Redis auth string")?;
            let password_str = std::str::from_utf8(password).context("invalid auth string")?;
            // Trim whitespace/newlines that might be in the secret
            let password_str = password_str.trim().to_string();
            (None, Some(password_str))
        }
        Some(Auth::Acl(acl)) => {
            let password = acl
                .password
                .as_ref()
                .context("ACL auth requires password")?;
            let password = secrets.load(password.clone());
            let password = password.get().context("failed to resolve Redis password")?;
            let password_str = std::str::from_utf8(password).context("invalid password")?;
            let password_str = password_str.trim().to_string();
            // If username is empty, treat it as password-only auth (like AuthString)
            let username = if acl.username.is_empty() {
                None
            } else {
                Some(acl.username.clone())
            };
            (username, Some(password_str))
        }
        None => (None, None),
    };

    // Build connection address based on TLS config
    let mut addr = if let Some(tls_config) = &server.tls_config {
        // TLS enabled - check for insecure mode
        let insecure = tls_config.disable_ca_validation;
        ConnectionAddr::TcpTls {
            host,
            port,
            insecure,
            tls_params: None, // TLS params will be set via build_with_tls
        }
    } else {
        // No TLS
        ConnectionAddr::Tcp(host, port)
    };

    // Handle hostname verification separately from CA validation
    if let Some(tls_config) = &server.tls_config {
        if tls_config.disable_tls_hostname_verification {
            addr.set_danger_accept_invalid_hostnames(true);
        }
    }

    let mut redis_info = RedisConnectionInfo::default().set_db(db.database_idx as i64);
    if let Some(user) = username {
        redis_info = redis_info.set_username(user);
    }
    if let Some(pass) = password {
        redis_info = redis_info.set_password(pass);
    }

    // Build connection info using builder pattern
    let conn_info = addr
        .into_connection_info()
        .context("failed to create connection info")?
        .set_redis_settings(redis_info);

    // Create client with or without TLS certificates
    if let Some(tls_config) = &server.tls_config {
        // Build TLS certificates config
        let root_cert = tls_config
            .server_ca_cert
            .as_ref()
            .map(|cert| cert.as_bytes().to_vec());

        let tls_certs = TlsCertificates {
            client_tls: None, // No client cert support yet
            root_cert,
        };

        redis::Client::build_with_tls(conn_info, tls_certs)
            .context("failed to create Redis client with TLS")
    } else {
        redis::Client::open(conn_info).context("failed to create Redis client")
    }
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
