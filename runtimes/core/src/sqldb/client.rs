use std::collections::HashMap;
use std::fmt::Write;
use std::future::Future;
use std::pin::Pin;

use bb8::{ErrorSink, PooledConnection, RunError};
use bb8_postgres::PostgresConnectionManager;
use futures_util::StreamExt;

use tokio_postgres::types::BorrowToSql;
use tokio_postgres::ResultFormat;

use crate::sqldb::val::RowValue;
use crate::trace::{protocol, Tracer};
use crate::{model, sqldb};

use super::transaction::Transaction;

/// Column metadata from a query result.
pub struct ColumnInfo {
    /// The column name.
    pub name: String,
    /// The PostgreSQL type OID.
    pub type_oid: u32,
    /// The OID of the table this column belongs to, if applicable.
    pub table_oid: Option<u32>,
    /// The attribute number of the column within its table, if applicable.
    pub column_id: Option<i16>,
}

/// A cursor over text-format query results.
/// Values are raw UTF-8 text as sent by PostgreSQL's text protocol,
/// matching what the pg Node.js library receives over the wire.
pub struct TextCursor {
    stream: Pin<Box<tokio_postgres::RowStream>>,
}

impl TextCursor {
    /// Returns column metadata for the result set.
    pub fn columns(&self) -> Vec<ColumnInfo> {
        self.stream
            .columns()
            .iter()
            .map(|col| ColumnInfo {
                name: col.name().to_string(),
                type_oid: col.type_().oid(),
                table_oid: col.table_oid(),
                column_id: col.column_id(),
            })
            .collect()
    }

    /// Returns the next row, or None when the stream is exhausted.
    pub async fn next(&mut self) -> Option<Result<TextRow, tokio_postgres::Error>> {
        self.stream.next().await.map(|r| r.map(TextRow))
    }

    /// Returns the number of rows affected by the query.
    /// Available after the stream has been fully consumed.
    pub fn rows_affected(&self) -> Option<u64> {
        self.stream.rows_affected()
    }

    /// Returns the command tag (e.g. "SELECT", "INSERT").
    /// Available after the stream has been fully consumed.
    pub fn command_tag(&self) -> Option<&str> {
        self.stream.command_tag()
    }
}

/// A single row from a text-format query.
/// Values are raw UTF-8 bytes as sent by PostgreSQL.
pub struct TextRow(tokio_postgres::Row);

impl TextRow {
    /// Returns the text value at the given column index, or None for SQL NULL.
    pub fn col_text(&self, idx: usize) -> Option<&[u8]> {
        self.0.col_buffer(idx)
    }

    /// Returns the number of columns.
    pub fn len(&self) -> usize {
        self.0.len()
    }

    /// Determines if the row contains no values.
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

type Mgr = PostgresConnectionManager<postgres_native_tls::MakeTlsConnector>;

pub struct Pool {
    pool: bb8::Pool<Mgr>,
    tracer: QueryTracer,
}

impl Pool {
    pub fn new<DB: sqldb::Database>(db: &DB, tracer: Tracer) -> anyhow::Result<Self> {
        let tls = db.tls()?.clone();
        let mgr = Mgr::new(db.config()?.clone(), tls);

        let pool_cfg = db.pool_config()?;
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

        let pool = pool.build_unchecked(mgr);
        Ok(Self {
            pool,
            tracer: QueryTracer(tracer),
        })
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

    /// Executes a query with text-format results, returning a cursor over raw text values.
    /// The trace covers query initiation; row fetching happens as the cursor is consumed.
    pub async fn query_raw_text<P, I>(
        &self,
        query: &str,
        params: I,
        source: Option<&model::Request>,
    ) -> Result<TextCursor, Error>
    where
        P: BorrowToSql,
        I: IntoIterator<Item = P>,
        I::IntoIter: ExactSizeIterator,
    {
        self.tracer
            .trace_text_query(source, query, || async {
                let conn = self.pool.get().await.map_err(|e| match e {
                    RunError::User(err) => Error::DB(err),
                    RunError::TimedOut => Error::ConnectTimeout,
                })?;
                conn.query_raw_with_format(query, params, ResultFormat::Text)
                    .await
                    .map_err(Error::from)
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

    pub async fn query_raw_text<P, I>(
        &self,
        query: &str,
        params: I,
        source: Option<&model::Request>,
    ) -> Result<TextCursor, Error>
    where
        P: BorrowToSql,
        I: IntoIterator<Item = P>,
        I::IntoIter: ExactSizeIterator,
    {
        self.tracer
            .trace_text_query(source, query, || async {
                let guard = self.conn.read().await;
                let Some(conn) = guard.as_ref() else {
                    return Err(Error::Closed);
                };
                conn.query_raw_with_format(query, params, ResultFormat::Text)
                    .await
                    .map_err(Error::from)
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

    pub(crate) async fn trace_text_query<F, Fut>(
        &self,
        source: Option<&model::Request>,
        query: &str,
        exec: F,
    ) -> Result<TextCursor, Error>
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
        Ok(TextCursor {
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
