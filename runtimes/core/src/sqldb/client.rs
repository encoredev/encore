use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;

use bb8::{ErrorSink, PooledConnection, RunError};
use bb8_postgres::PostgresConnectionManager;
use futures_util::StreamExt;

use tokio_postgres::types::BorrowToSql;

use crate::sqldb::val::RowValue;
use crate::trace::{protocol, Tracer};
use crate::{model, sqldb};

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
        log::error!(
            "database {}: connection pool error: {:?}",
            self.db_name,
            err
        );
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

type PooledConn =
    PooledConnection<'static, PostgresConnectionManager<postgres_native_tls::MakeTlsConnector>>;

pub struct Connection {
    conn: tokio::sync::RwLock<Option<PooledConn>>,
    tracer: QueryTracer,
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

#[derive(Debug, Clone)]
struct QueryTracer(Tracer);

impl QueryTracer {
    async fn trace<F, Fut>(
        &self,
        source: Option<&model::Request>,
        query: &str,
        exec: F,
    ) -> Result<Cursor, Error>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<tokio_postgres::RowStream, Error>>,
    {
        let start_id = if let Some(source) = source {
            let id = self
                .0
                .db_query_start(protocol::DBQueryStartData { source, query });
            Some(id)
        } else {
            None
        };

        let result = exec().await;

        if let Some(start_id) = start_id {
            self.0.db_query_end(protocol::DBQueryEndData {
                start_id,
                source: source.unwrap(),
                error: result.as_ref().err(),
            });
        }

        let stream = result?;
        Ok(Cursor {
            stream: Box::pin(stream),
        })
    }
}
