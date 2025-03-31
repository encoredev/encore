use tokio_postgres::types::BorrowToSql;

use crate::model;

use super::{
    client::{Error, PooledConn, QueryTracer},
    Cursor,
};

pub struct Transaction {
    conn: PooledConn,
    tracer: QueryTracer,
    done: bool,
}

impl Transaction {
    pub(crate) async fn begin(
        conn: PooledConn,
        tracer: QueryTracer,
    ) -> Result<Self, tokio_postgres::Error> {
        struct RollbackIfNotDone<'me> {
            client: &'me tokio_postgres::Client,
            done: bool,
        }

        impl Drop for RollbackIfNotDone<'_> {
            fn drop(&mut self) {
                if self.done {
                    return;
                }

                self.client.__private_api_rollback();
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
            conn.batch_execute("BEGIN").await?;
            cleaner.done = true;
        }

        Ok(Transaction {
            conn,
            tracer,
            done: false,
        })
    }

    pub async fn commit(mut self) -> Result<(), tokio_postgres::Error> {
        self.done = true;
        // TODO trace
        // TODO savepoint
        self.conn.batch_execute("COMMIT").await
    }

    pub async fn rollback(mut self) -> Result<(), tokio_postgres::Error> {
        self.done = true;
        // TODO trace
        // TODO savepoint
        self.conn.batch_execute("ROLLBACK").await
    }

    // TODO: nested transactions via savepoints
    pub async fn transaction(&mut self) -> Result<Transaction, tokio_postgres::Error> {
        todo!()
    }
    pub async fn savepoint(&mut self, name: &str) -> Result<Transaction, tokio_postgres::Error> {
        todo!()
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

        // TODO savepoint
        self.conn.__private_api_rollback();
    }
}
