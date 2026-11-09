use std::collections::HashMap;
use std::fmt::Write;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex, OnceLock, Weak};

use bb8::{ErrorSink, PooledConnection, RunError};
use bb8_postgres::PostgresConnectionManager;
use futures_util::StreamExt;

use tokio_postgres::config::Host;
use tokio_postgres::types::BorrowToSql;

use crate::sqldb::val::RowValue;
use crate::trace::{protocol, Tracer};
use crate::{model, sqldb};

use super::manager::PoolConfig;
use super::transaction::Transaction;

type Mgr = PostgresConnectionManager<postgres_native_tls::MakeTlsConnector>;

/// Identifies a set of interchangeable connections.
///
/// Two databases with equal keys are the same database reached the same way, so
/// a single set of connections serves both. The fields are therefore everything
/// that makes two connections *not* interchangeable — including the password,
/// not as an authorization check (nothing outside the runtime config can choose
/// one; see `databases_from_cfg`) but so that a credential which differs for any
/// reason cannot be served a pool that was opened with another.
///
/// TLS is deliberately absent. It is configured per server, and a server is
/// identified by its host and port, so equal keys already imply equal TLS.
#[derive(Clone, PartialEq, Eq, Hash)]
struct PoolKey {
    hosts: Vec<String>,
    ports: Vec<u16>,
    dbname: Option<String>,
    user: Option<String>,
    password: Option<Vec<u8>>,
    min_conns: u32,
    max_conns: u32,
}

/// Compared in full, but never printed: `tokio_postgres::Config` redacts its own
/// password for this reason, and a derived `Debug` here would undo that.
impl std::fmt::Debug for PoolKey {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PoolKey")
            .field("hosts", &self.hosts)
            .field("ports", &self.ports)
            .field("dbname", &self.dbname)
            .field("user", &self.user)
            .field("password", &self.password.as_ref().map(|_| "_"))
            .field("min_conns", &self.min_conns)
            .field("max_conns", &self.max_conns)
            .finish()
    }
}

impl PoolKey {
    fn new(config: &tokio_postgres::Config, pool_cfg: &PoolConfig) -> Self {
        Self {
            hosts: config
                .get_hosts()
                .iter()
                .map(|host| match host {
                    Host::Tcp(host) => host.clone(),
                    #[cfg(unix)]
                    Host::Unix(path) => path.to_string_lossy().into_owned(),
                })
                .collect(),
            ports: config.get_ports().to_vec(),
            dbname: config.get_dbname().map(str::to_owned),
            user: config.get_user().map(str::to_owned),
            password: config.get_password().map(<[u8]>::to_vec),
            min_conns: pool_cfg.min_conns,
            max_conns: pool_cfg.max_conns,
        }
    }
}

/// The connection pools in use by this process, shared across runtimes.
///
/// A process can hold more than one runtime: in test mode every `Runtime`
/// construction builds a fresh one (see the JS runtime's `test_mode` branch), and
/// a test runner that re-evaluates the module declaring a database does exactly
/// that. Keying the pools on the connection itself rather than on the runtime
/// that asked for it bounds a process to one pool per database however many
/// runtimes it accumulates.
///
/// Held weakly so a pool is released once the last handle to it goes away. A dead
/// entry is simply overwritten the next time its database is opened.
static POOLS: OnceLock<Mutex<HashMap<PoolKey, Weak<bb8::Pool<Mgr>>>>> = OnceLock::new();

pub struct Pool {
    /// Shared with every other handle to the same database in this process.
    pool: Arc<bb8::Pool<Mgr>>,
    /// Not shared: traces belong to the runtime that issued the query.
    tracer: QueryTracer,
}

impl Pool {
    pub fn new<DB: sqldb::Database>(db: &DB, tracer: Tracer) -> anyhow::Result<Self> {
        let pool_cfg = db.pool_config()?;
        let key = PoolKey::new(db.config()?, &pool_cfg);

        let mut pools = POOLS
            .get_or_init(Default::default)
            .lock()
            .unwrap_or_else(|err| err.into_inner());

        let pool = match pools.get(&key).and_then(Weak::upgrade) {
            Some(pool) => pool,
            None => {
                let pool = Arc::new(Self::build_conn_pool(db, &pool_cfg)?);
                pools.insert(key, Arc::downgrade(&pool));
                pool
            }
        };

        Ok(Self {
            pool,
            tracer: QueryTracer(tracer),
        })
    }

    fn build_conn_pool<DB: sqldb::Database>(
        db: &DB,
        pool_cfg: &PoolConfig,
    ) -> anyhow::Result<bb8::Pool<Mgr>> {
        let tls = db.tls()?.clone();
        let mgr = Mgr::new(db.config()?.clone(), tls);

        let mut pool = bb8::Pool::builder()
            .error_sink(Box::new(RustLoggerSink {
                db_name: db.name().to_string(),
            }))
            .max_size(if pool_cfg.max_conns > 0 {
                pool_cfg.max_conns
            } else {
                30
            });

        if pool_cfg.min_conns > 0 {
            pool = pool.min_idle(Some(pool_cfg.min_conns));
        }

        Ok(pool.build_unchecked(mgr))
    }

    /// Reports whether two handles draw on the same connections.
    #[cfg(test)]
    pub(crate) fn shares_connections_with(&self, other: &Pool) -> bool {
        Arc::ptr_eq(&self.pool, &other.pool)
    }
}

#[derive(Debug, Clone)]
struct RustLoggerSink {
    db_name: String,
}

impl ErrorSink<tokio_postgres::Error> for RustLoggerSink {
    fn sink(&self, err: tokio_postgres::Error) {
        let mut msg = format!(
            "database {}: connection pool error: {:?}",
            self.db_name, err
        );
        let mut source = std::error::Error::source(&err);
        while let Some(cause) = source {
            let _ = write!(msg, "\n  caused by: {cause}");
            source = std::error::Error::source(cause);
        }
        log::error!("{msg}");
    }

    fn boxed_clone(&self) -> Box<dyn ErrorSink<tokio_postgres::Error>> {
        Box::new(self.clone())
    }
}

impl Pool {
    pub async fn query_raw<P, I>(
        &self,
        query: &str,
        params: I,
        source: Option<&model::Request>,
    ) -> Result<Cursor, Error>
    where
        P: BorrowToSql,
        I: IntoIterator<Item = P>,
        I::IntoIter: ExactSizeIterator,
    {
        self.tracer
            .trace(source, query, || async {
                let conn = self.pool.get().await.map_err(|e| match e {
                    RunError::User(err) => Error::DB(err),
                    RunError::TimedOut => Error::ConnectTimeout,
                })?;
                conn.query_raw(query, params).await.map_err(Error::from)
            })
            .await
    }

    pub async fn acquire(&self) -> Result<Connection, tokio_postgres::Error> {
        let conn = self.pool.get_owned().await.map_err(|e| match e {
            RunError::User(err) => err,
            RunError::TimedOut => tokio_postgres::Error::__private_api_timeout(),
        })?;
        Ok(Connection {
            conn: tokio::sync::RwLock::new(Some(conn)),
            tracer: self.tracer.clone(),
        })
    }

    pub async fn begin(&self, source: Option<&model::Request>) -> Result<Transaction, Error> {
        let conn = self.pool.get_owned().await.map_err(|e| match e {
            RunError::User(err) => err,
            RunError::TimedOut => tokio_postgres::Error::__private_api_timeout(),
        })?;
        Transaction::begin(conn, self.tracer.clone(), source).await
    }
}

pub struct Cursor {
    stream: Pin<Box<tokio_postgres::RowStream>>,
}

impl Cursor {
    pub async fn next(&mut self) -> Option<Result<Row, tokio_postgres::Error>> {
        match self.stream.next().await {
            Some(Ok(row)) => Some(Ok(Row { row })),
            Some(Err(err)) => Some(Err(err)),
            None => None,
        }
    }
}

pub struct Row {
    row: tokio_postgres::Row,
}

impl Row {
    pub fn values(&self) -> anyhow::Result<HashMap<String, RowValue>> {
        let cols = self.row.columns();
        let mut map = HashMap::with_capacity(cols.len());
        for (i, col) in cols.iter().enumerate() {
            let name = col.name().to_string();
            let value: RowValue = self
                .row
                .try_get(i)
                .map_err(|e| anyhow::anyhow!("unable to parse column {}: {:#?}", name, e))?;
            map.insert(name, value);
        }
        Ok(map)
    }
}

pub(crate) type PooledConn =
    PooledConnection<'static, PostgresConnectionManager<postgres_native_tls::MakeTlsConnector>>;

pub struct Connection {
    conn: tokio::sync::RwLock<Option<PooledConn>>,
    tracer: QueryTracer,
}

impl Connection {
    pub async fn close(&self) {
        let mut guard = self.conn.write().await;
        if let Some(conn) = guard.take() {
            drop(conn);
        }
    }

    pub async fn query_raw<P, I>(
        &self,
        query: &str,
        params: I,
        source: Option<&model::Request>,
    ) -> Result<Cursor, Error>
    where
        P: BorrowToSql,
        I: IntoIterator<Item = P>,
        I::IntoIter: ExactSizeIterator,
    {
        self.tracer
            .trace(source, query, || async {
                let guard = self.conn.read().await;
                let Some(conn) = guard.as_ref() else {
                    return Err(Error::Closed);
                };
                conn.query_raw(query, params).await.map_err(Error::from)
            })
            .await
    }
}

#[derive(Debug)]
pub enum Error {
    DB(tokio_postgres::Error),
    Closed,
    ConnectTimeout,
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::DB(err) => <tokio_postgres::Error as std::fmt::Display>::fmt(err, f),
            Error::Closed => f.write_str("connection_closed"),
            Error::ConnectTimeout => f.write_str("timeout establishing connection"),
        }
    }
}

impl From<tokio_postgres::Error> for Error {
    fn from(err: tokio_postgres::Error) -> Self {
        Error::DB(err)
    }
}

#[derive(Debug, Clone)]
pub(crate) struct QueryTracer(Tracer);

impl QueryTracer {
    pub(crate) async fn trace<F, Fut>(
        &self,
        source: Option<&model::Request>,
        query: &str,
        exec: F,
    ) -> Result<Cursor, Error>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<tokio_postgres::RowStream, Error>>,
    {
        let start_id = source.and_then(|source| {
            self.0
                .db_query_start(protocol::DBQueryStartData { source, query })
        });

        let result = exec().await;

        if let Some(source) = source {
            self.0.db_query_end(protocol::DBQueryEndData {
                start_id,
                source,
                error: result.as_ref().err(),
            });
        }

        let stream = result?;
        Ok(Cursor {
            stream: Box::pin(stream),
        })
    }

    pub(crate) async fn trace_batch_execute<F, Fut>(
        &self,
        source: Option<&model::Request>,
        query: &str,
        exec: F,
    ) -> Result<(), Error>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<(), Error>>,
    {
        let start_id = source.and_then(|source| {
            self.0
                .db_query_start(protocol::DBQueryStartData { source, query })
        });

        let result = exec().await;

        if let Some(source) = source {
            self.0.db_query_end(protocol::DBQueryEndData {
                start_id,
                source,
                error: result.as_ref().err(),
            });
        }

        result
    }
}

#[cfg(test)]
mod key_test {
    use super::*;

    /// The password is part of the key but must not be printable: it would
    /// otherwise leak through any log line or panic message that formats a key,
    /// undoing the redaction `tokio_postgres::Config` does for the same reason.
    #[test]
    fn debug_does_not_print_the_password() {
        let mut config = tokio_postgres::Config::new();
        config.host("localhost").user("encore").password("hunter2");
        let key = PoolKey::new(
            &config,
            &PoolConfig {
                min_conns: 0,
                max_conns: 30,
            },
        );

        let rendered = format!("{key:?}");
        assert!(!rendered.contains("hunter2"), "{rendered}");
        // The bytes must not leak in numeric form either.
        assert!(!rendered.contains("104"), "{rendered}");
    }
}
