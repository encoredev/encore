use std::collections::HashMap;
use std::fmt::Write;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex, OnceLock, Weak};

use bb8::{ErrorSink, PooledConnection, RunError};
use futures_util::StreamExt;

use tokio_postgres::config::Host;
use tokio_postgres::types::BorrowToSql;

use crate::sqldb::val::RowValue;
use crate::trace::{protocol, Tracer};
use crate::{model, sqldb};

use super::auth::Auth;
use super::manager::PoolConfig;
use super::transaction::Transaction;

/// Opens a database's connections, like `bb8_postgres`'s manager, with the
/// credentials its [`Auth`] gives each one.
pub(crate) struct Mgr {
    config: Arc<tokio_postgres::Config>,
    tls: postgres_native_tls::MakeTlsConnector,
    auth: Auth,
}

impl bb8::ManageConnection for Mgr {
    type Connection = tokio_postgres::Client;
    type Error = Error;

    async fn connect(&self) -> Result<Self::Connection, Self::Error> {
        let config = self
            .auth
            .connect_config(&self.config)
            .await
            .map_err(Error::Credentials)?;
        let (client, connection) = config.connect(self.tls.clone()).await?;
        tokio::spawn(async move { connection.await.map(|_| ()) });
        Ok(client)
    }

    async fn is_valid(&self, conn: &mut Self::Connection) -> Result<(), Self::Error> {
        conn.simple_query("").await?;
        Ok(())
    }

    fn has_broken(&self, conn: &mut Self::Connection) -> bool {
        conn.is_closed()
    }
}

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
    gcp_iam: bool,
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
            .field("gcp_iam", &self.gcp_iam)
            .field("min_conns", &self.min_conns)
            .field("max_conns", &self.max_conns)
            .finish()
    }
}

impl PoolKey {
    fn new(config: &tokio_postgres::Config, auth: &Auth, pool_cfg: &PoolConfig) -> Self {
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
            gcp_iam: auth.is_gcp_iam(),
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
        let key = PoolKey::new(db.config()?, db.auth()?, &pool_cfg);

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
        let mgr = Mgr {
            config: db.config()?.clone(),
            tls: db.tls()?.clone(),
            auth: db.auth()?.clone(),
        };

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

impl ErrorSink<Error> for RustLoggerSink {
    fn sink(&self, err: Error) {
        let Error::DB(err) = err else {
            log::error!("database {}: connection pool error: {}", self.db_name, err);
            return;
        };
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

    fn boxed_clone(&self) -> Box<dyn ErrorSink<Error>> {
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
                    RunError::User(err) => err,
                    RunError::TimedOut => Error::ConnectTimeout,
                })?;
                conn.query_raw(query, params).await.map_err(Error::from)
            })
            .await
    }

    pub async fn acquire(&self) -> Result<Connection, Error> {
        let conn = self.pool.get_owned().await.map_err(|e| match e {
            RunError::User(err) => err,
            RunError::TimedOut => Error::ConnectTimeout,
        })?;
        Ok(Connection {
            conn: tokio::sync::RwLock::new(Some(conn)),
            tracer: self.tracer.clone(),
        })
    }

    pub async fn begin(&self, source: Option<&model::Request>) -> Result<Transaction, Error> {
        let conn = self.pool.get_owned().await.map_err(|e| match e {
            RunError::User(err) => err,
            RunError::TimedOut => Error::ConnectTimeout,
        })?;
        Transaction::begin(conn, self.tracer.clone(), source).await
    }
}

pub struct Cursor {
    /// `None` once the cursor is closed.
    stream: Option<Pin<Box<tokio_postgres::RowStream>>>,
}

impl Cursor {
    pub async fn next(&mut self) -> Option<Result<Row, tokio_postgres::Error>> {
        match self.stream.as_mut()?.next().await {
            Some(Ok(row)) => Some(Ok(Row { row })),
            Some(Err(err)) => Some(Err(err)),
            None => None,
        }
    }

    /// Discards the rows not yet read.
    ///
    /// A connection hands each query's result to its caller in order, a few
    /// messages at a time, and reads nothing further from the server until that
    /// caller takes them. A cursor left open with rows unread therefore stalls
    /// every later query on its connection, for as long as the cursor lives —
    /// which, for one held by JavaScript, is until it is garbage collected.
    /// Closing it lets the connection skip past the rest of the result.
    pub fn close(&mut self) {
        self.stream = None;
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

pub(crate) type PooledConn = PooledConnection<'static, Mgr>;

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
    /// The credentials to open a connection with could not be had.
    Credentials(anyhow::Error),
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::DB(err) => <tokio_postgres::Error as std::fmt::Display>::fmt(err, f),
            Error::Closed => f.write_str("connection_closed"),
            Error::ConnectTimeout => f.write_str("timeout establishing connection"),
            Error::Credentials(err) => write!(f, "failed to get database credentials: {err:#}"),
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
            stream: Some(Box::pin(stream)),
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
            &Auth::Password,
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

#[cfg(test)]
mod cursor_test {
    use std::collections::HashMap;
    use std::time::Duration;

    use bytes::{Buf, BufMut, BytesMut};
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::{TcpListener, TcpStream};

    use super::*;

    /// Rows in the result that is left unread. Each is sent on its own, so the
    /// client reads them one at a time, as it would a large result.
    const MANY: i32 = 20;

    /// A cursor left open with rows unread used to hang every later query on
    /// its connection until the cursor was garbage collected — in a
    /// transaction, the very next statement. Closing it must free the
    /// connection.
    #[tokio::test]
    async fn closing_a_cursor_frees_its_connection() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        tokio::spawn(async move {
            let (socket, _) = listener.accept().await.unwrap();
            serve(socket).await;
        });

        let mut config = tokio_postgres::Config::new();
        config
            .host("127.0.0.1")
            .port(port)
            .user("encore")
            .dbname("db");
        let (client, connection) = config.connect(tokio_postgres::NoTls).await.unwrap();
        tokio::spawn(connection);
        let client = Arc::new(client);

        let tracer = QueryTracer(crate::trace::Tracer::noop());
        let query = |sql: &'static str| {
            let client = client.as_ref();
            tracer.trace(None, sql, move || async move {
                client
                    .query_raw(sql, Vec::<i32>::new())
                    .await
                    .map_err(Error::from)
            })
        };

        // Like `queryRow`: read the first row and leave the rest.
        let mut cursor = query("SELECT many").await.unwrap();
        let first = cursor.next().await.unwrap().unwrap();
        assert_eq!(first.row.get::<_, i32>(0), 1);

        let next = tokio::spawn({
            let client = client.clone();
            async move { client.query_one("SELECT one", &[]).await }
        });

        // The hazard being guarded against: while the cursor is open, the
        // connection delivers nothing else.
        tokio::time::sleep(Duration::from_millis(500)).await;
        assert!(
            !next.is_finished(),
            "the next query completed while the cursor was open; \
             this test no longer exercises the stall"
        );

        cursor.close();
        assert!(cursor.next().await.is_none(), "a closed cursor yields rows");

        let row = tokio::time::timeout(Duration::from_secs(5), next)
            .await
            .expect("the next query hung after the cursor was closed")
            .unwrap()
            .unwrap();
        assert_eq!(row.get::<_, i32>(0), 1);
    }

    /// Plays a Postgres server well enough for `prepare` and `query`: every
    /// statement returns one int4 column, counting up from 1, with [`MANY`]
    /// rows if its text contains "many" and one row otherwise.
    async fn serve(mut socket: TcpStream) {
        let len = socket.read_i32().await.unwrap();
        let mut startup = vec![0; len as usize - 4];
        socket.read_exact(&mut startup).await.unwrap();

        let mut out = BytesMut::new();
        message(&mut out, b'R', |b| b.put_i32(0)); // AuthenticationOk
        ready_for_query(&mut out);
        socket.write_all(&out).await.unwrap();

        let mut rows_by_statement = HashMap::new();
        let mut rows = 0;
        loop {
            let Ok(tag) = socket.read_u8().await else {
                return;
            };
            let len = socket.read_i32().await.unwrap();
            let mut body = vec![0; len as usize - 4];
            socket.read_exact(&mut body).await.unwrap();
            let mut body = &body[..];

            let mut out = BytesMut::new();
            match tag {
                b'P' => {
                    let name = cstr(&mut body);
                    let sql = cstr(&mut body);
                    let n = if sql.contains("many") { MANY } else { 1 };
                    rows_by_statement.insert(name, n);
                    message(&mut out, b'1', |_| {}); // ParseComplete
                }
                b'D' => {
                    message(&mut out, b't', |b| b.put_i16(0)); // ParameterDescription
                    message(&mut out, b'T', |b| {
                        // RowDescription: one int4 column.
                        b.put_i16(1);
                        b.put_slice(b"n\0");
                        b.put_i32(0);
                        b.put_i16(0);
                        b.put_i32(23);
                        b.put_i16(4);
                        b.put_i32(-1);
                        b.put_i16(0);
                    });
                }
                b'B' => {
                    let _portal = cstr(&mut body);
                    rows = rows_by_statement[&cstr(&mut body)];
                    message(&mut out, b'2', |_| {}); // BindComplete
                }
                b'E' => {
                    for i in 1..=rows {
                        message(&mut out, b'D', |b| {
                            b.put_i16(1);
                            b.put_i32(4);
                            b.put_i32(i);
                        });
                        // Send each row on its own, so the client reads it
                        // as its own chunk.
                        socket.write_all(&out).await.unwrap();
                        out.clear();
                        tokio::time::sleep(Duration::from_millis(5)).await;
                    }
                    message(&mut out, b'C', |b| b.put_slice(b"SELECT\0"));
                }
                b'C' => message(&mut out, b'3', |_| {}), // CloseComplete
                b'S' => ready_for_query(&mut out),
                b'X' => return,
                other => panic!("unexpected message {:?}", other as char),
            }
            socket.write_all(&out).await.unwrap();
        }
    }

    fn message(out: &mut BytesMut, tag: u8, body: impl FnOnce(&mut BytesMut)) {
        let mut b = BytesMut::new();
        body(&mut b);
        out.put_u8(tag);
        out.put_i32(b.len() as i32 + 4);
        out.put_slice(&b);
    }

    fn ready_for_query(out: &mut BytesMut) {
        message(out, b'Z', |b| b.put_u8(b'I'));
    }

    fn cstr(body: &mut &[u8]) -> String {
        let end = body.iter().position(|&b| b == 0).unwrap();
        let s = String::from_utf8(body[..end].to_vec()).unwrap();
        body.advance(end + 1);
        s
    }
}
