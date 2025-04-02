use std::sync::Arc;

use tokio_postgres::types::BorrowToSql;

use crate::model;

use super::{
    client::{Error, PooledConn, QueryTracer},
    Cursor,
};

pub struct Transaction {
    conn: Arc<PooledConn>,
    tracer: QueryTracer,
    done: bool,
    savepoint: Option<Savepoint>,
}

pub struct Savepoint {
    name: String,
    depth: u32,
}

impl Transaction {
    pub(crate) async fn begin(
        conn: PooledConn,
        tracer: QueryTracer,
        source: Option<&model::Request>,
    ) -> Result<Self, Error> {
        struct RollbackIfNotDone<'me> {
            client: &'me tokio_postgres::Client,
            done: bool,
        }

        impl Drop for RollbackIfNotDone<'_> {
            fn drop(&mut self) {
                if self.done {
                    return;
                }

                self.client.__private_api_rollback(None);
            }
        }

        // This is done, as `Future` created by this method can be dropped after
        // `RequestMessages` is synchronously send to the `Connection` by
        // `batch_execute()`, but before `Responses` is asynchronously polled to
        // completion. In that case `Transaction` won't be created and thus
        // won't be rolled back.
        {
            let mut cleaner = RollbackIfNotDone {
                client: &conn,
                done: false,
            };

            tracer
                .trace_batch_execute(source, "BEGIN", || async {
                    conn.batch_execute("BEGIN").await.map_err(Error::from)
                })
                .await?;

            cleaner.done = true;
        }

        Ok(Transaction {
            conn: Arc::new(conn),
            tracer,
            done: false,
            savepoint: None,
        })
    }

    pub async fn commit(mut self, source: Option<&model::Request>) -> Result<(), Error> {
        self.done = true;
        if let Some(sp) = self.savepoint.as_ref() {
            self.batch_execute(&format!("RELEASE {}", sp.name), source)
                .await
        } else {
            self.batch_execute("COMMIT", source).await
        }
    }

    pub async fn rollback(mut self, source: Option<&model::Request>) -> Result<(), Error> {
        self.done = true;
        if let Some(sp) = self.savepoint.as_ref() {
            self.batch_execute(&format!("ROLLBACK TO {}", sp.name), source)
                .await
        } else {
            self.batch_execute("ROLLBACK", source).await
        }
    }

    async fn batch_execute(
        &self,
        query: &str,
        source: Option<&model::Request>,
    ) -> Result<(), Error> {
        self.tracer
            .trace_batch_execute(source, query, || async {
                self.conn.batch_execute(query).await.map_err(Error::from)
            })
            .await
    }

    pub async fn savepoint(
        &mut self,
        name: Option<&str>,
        source: Option<&model::Request>,
    ) -> Result<Transaction, Error> {
        let depth = self.savepoint.as_ref().map_or(0, |sp| sp.depth) + 1;
        let name = name.unwrap_or(&format!("sp_{depth}")).to_string();
        let query = &format!("SAVEPOINT {}", &name);

        self.batch_execute(query, source).await?;

        Ok(Transaction {
            conn: self.conn.clone(),
            tracer: self.tracer.clone(),
            done: false,
            savepoint: Some(Savepoint { name, depth }),
        })
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
                self.conn
                    .query_raw(query, params)
                    .await
                    .map_err(Error::from)
            })
            .await
    }
}

impl Drop for Transaction {
    fn drop(&mut self) {
        if self.done {
            return;
        }

        if self.savepoint.is_none() {
            log::warn!("transaction was not finished, rolling back");
        }

        let name = self.savepoint.as_ref().map(|sp| sp.name.as_str());
        self.conn.__private_api_rollback(name);
    }
}
