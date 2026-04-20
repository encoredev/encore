use crate::api::Request;
use crate::pvalue::{parse_pvalue, pvalue_to_js};
use bytes::BytesMut;
use encore_runtime_core::sqldb;
use mappable_rc::Marc;
use napi::bindgen_prelude::{Buffer, Either3, Null};
use napi::{Env, JsUnknown};
use napi_derive::napi;
use std::collections::HashMap;
use std::fmt::Display;
use std::sync::{Arc, OnceLock};
use tokio_postgres::types::{to_sql_checked, Format, IsNull, ToSql, Type};

/// A single driver query parameter: string (text format), Buffer (binary format), or null.
type DriverParamValue = Either3<String, Buffer, Null>;

/// A query parameter that matches how the pg Node.js library sends values:
/// text format for strings (PostgreSQL handles type coercion server-side),
/// binary format for raw byte data (bytea).
#[derive(Debug)]
enum DriverParam {
    Text(String),
    Binary(Vec<u8>),
    Null,
}

impl ToSql for DriverParam {
    fn to_sql(
        &self,
        _ty: &Type,
        out: &mut BytesMut,
    ) -> Result<IsNull, Box<dyn std::error::Error + Sync + Send>> {
        match self {
            DriverParam::Text(s) => {
                out.extend_from_slice(s.as_bytes());
                Ok(IsNull::No)
            }
            DriverParam::Binary(b) => {
                out.extend_from_slice(b);
                Ok(IsNull::No)
            }
            DriverParam::Null => Ok(IsNull::Yes),
        }
    }

    fn accepts(_ty: &Type) -> bool {
        true
    }

    fn encode_format(&self, _ty: &Type) -> Format {
        match self {
            DriverParam::Binary(_) => Format::Binary,
            _ => Format::Text,
        }
    }

    to_sql_checked!();
}

async fn collect_driver_result(mut cursor: sqldb::TextCursor) -> napi::Result<DriverQueryResult> {
    let columns: Vec<DriverColumnInfo> = cursor
        .columns()
        .into_iter()
        .map(|c| DriverColumnInfo {
            name: c.name,
            type_oid: c.type_oid,
            table_oid: c.table_oid,
            column_id: c.column_id,
        })
        .collect();

    let num_cols = columns.len();
    let mut rows = Vec::new();
    while let Some(row) = cursor.next().await {
        let row = row.map_err(to_napi_err)?;
        let mut text_row = Vec::with_capacity(num_cols);
        for i in 0..num_cols {
            let value = row
                .col_text(i)
                .map(|bytes| String::from_utf8_lossy(bytes).into_owned());
            text_row.push(value);
        }
        rows.push(text_row);
    }

    Ok(DriverQueryResult {
        columns,
        rows,
        affected_rows: cursor.rows_affected().unwrap_or(0) as i64,
        command: cursor.command_tag().unwrap_or("").to_string(),
    })
}

fn convert_driver_params(params: Vec<DriverParamValue>) -> Vec<DriverParam> {
    params
        .into_iter()
        .map(|val| match val {
            Either3::A(s) => DriverParam::Text(s),
            Either3::B(b) => DriverParam::Binary(b.to_vec()),
            Either3::C(_) => DriverParam::Null,
        })
        .collect()
}

/// Query result formatted to match the pg Node.js library's QueryResult interface.
/// This enables ORM adapters (Drizzle, Prisma, etc.) to apply their own type parsers
/// to the raw text values, exactly as they would with a real pg.Pool connection.
///
/// See: https://node-postgres.com/apis/result
#[napi(object)]
pub struct DriverColumnInfo {
    pub name: String,
    pub type_oid: u32,
    pub table_oid: Option<u32>,
    pub column_id: Option<i16>,
}

/// See: https://node-postgres.com/apis/result
#[napi(object)]
pub struct DriverQueryResult {
    /// Column metadata (name and type OID).
    pub columns: Vec<DriverColumnInfo>,
    /// Rows as arrays of text values. Null means SQL NULL.
    pub rows: Vec<Vec<Option<String>>>,
    /// Number of rows affected (for INSERT/UPDATE/DELETE).
    pub affected_rows: i64,
    /// The command tag (e.g. "SELECT", "INSERT").
    pub command: String,
}

#[napi]
pub struct SQLDatabase {
    db: Arc<dyn sqldb::Database>,
    pool: OnceLock<Marc<napi::Result<sqldb::Pool>>>,
}

#[napi]
pub struct QueryArgs {
    values: std::sync::Mutex<Vec<sqldb::RowValue>>,
}

#[napi]
impl QueryArgs {
    #[napi(constructor)]
    pub fn new(params: Vec<JsUnknown>) -> napi::Result<Self> {
        let values = convert_row_values(params)?;
        Ok(Self {
            values: std::sync::Mutex::new(values),
        })
    }
}

fn convert_row_values(params: Vec<JsUnknown>) -> napi::Result<Vec<sqldb::RowValue>> {
    use napi::JsBuffer;
    params
        .into_iter()
        .map(|val| -> napi::Result<sqldb::RowValue> {
            if val.is_buffer()? {
                let buf: JsBuffer = val.try_into()?;
                let buf = buf.into_value()?;
                return Ok(sqldb::RowValue::Bytes(buf.to_vec()));
            }
            let pval = parse_pvalue(val)?;
            Ok(sqldb::RowValue::PVal(pval))
        })
        .collect()
}

#[napi]
impl SQLDatabase {
    pub(crate) fn new(db: Arc<dyn sqldb::Database>) -> Self {
        Self {
            db,
            pool: OnceLock::new(),
        }
    }

    /// Reports the connection string to connect to this database.
    #[napi]
    pub fn conn_string(&self) -> &str {
        self.db.proxy_conn_string()
    }

    /// Executes a query returning text-format results with column metadata.
    /// This is designed for building pg-compatible driver adapters — the results
    /// match what the pg Node.js library returns, enabling ORM type parsers to work.
    /// Queries are automatically traced via the Encore runtime.
    ///
    /// Params should be pre-processed by pg's prepareValue: each value is
    /// either a string (text format), Buffer (binary format), or null.
    #[napi]
    pub async fn driver_query(
        &self,
        query: String,
        params: Vec<DriverParamValue>,
        source: Option<&Request>,
    ) -> napi::Result<DriverQueryResult> {
        let values = convert_driver_params(params);
        let source = source.map(|s| s.inner.as_ref());
        let cursor = self
            .pool()?
            .query_raw_text(&query, values, source)
            .await
            .map_err(to_napi_err)?;
        collect_driver_result(cursor).await
    }

    /// Begins a transaction
    #[napi]
    pub async fn begin(&self, source: Option<&Request>) -> napi::Result<Transaction> {
        let source = source.map(|s| s.inner.as_ref());
        let tx = self
            .pool()?
            .begin(source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;

        Ok(Transaction {
            tx: tokio::sync::Mutex::new(Some(tx)),
        })
    }

    #[napi]
    pub async fn query(
        &self,
        query: String,
        args: &QueryArgs,
        source: Option<&Request>,
    ) -> napi::Result<Cursor> {
        let values: Vec<_> = args.values.lock().unwrap().drain(..).collect();
        let source = source.map(|s| s.inner.as_ref());
        let stream = self
            .pool()?
            .query_raw(&query, values, source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        Ok(Cursor {
            stream: tokio::sync::Mutex::new(stream),
        })
    }

    #[napi]
    pub async fn query_row(
        &self,
        query: String,
        args: &QueryArgs,
        source: Option<&Request>,
    ) -> napi::Result<Option<Row>> {
        let values: Vec<_> = args.values.lock().unwrap().drain(..).collect();
        let source = source.map(|s| s.inner.as_ref());
        let mut stream = self
            .pool()?
            .query_raw(&query, values, source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        let row = stream
            .next()
            .await
            .transpose()
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        Ok(row.map(|row| Row { row }))
    }

    fn pool(&self) -> napi::Result<&sqldb::Pool> {
        match self.pool_marc().as_ref() {
            Ok(pool) => Ok(pool),
            Err(e) => Err(e.clone()),
        }
    }

    fn pool_marc(&self) -> &Marc<napi::Result<sqldb::Pool>> {
        self.pool.get_or_init(|| {
            let pool = self
                .db
                .new_pool()
                .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e));
            Marc::new(pool)
        })
    }
}

#[napi]
pub struct Transaction {
    tx: tokio::sync::Mutex<Option<sqldb::Transaction>>,
}

#[napi]
impl Transaction {
    #[napi]
    pub async fn commit(&self, source: Option<&Request>) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        let tx = self.tx.lock().await.take().ok_or(napi::Error::new(
            napi::Status::GenericFailure,
            "transaction closed",
        ))?;
        tx.commit(source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))
    }

    #[napi]
    pub async fn rollback(&self, source: Option<&Request>) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        let tx = self.tx.lock().await.take().ok_or(napi::Error::new(
            napi::Status::GenericFailure,
            "transaction closed",
        ))?;
        tx.rollback(source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))
    }

    #[napi]
    pub async fn query(
        &self,
        query: String,
        args: &QueryArgs,
        source: Option<&Request>,
    ) -> napi::Result<Cursor> {
        let values: Vec<_> = args.values.lock().unwrap().drain(..).collect();
        let source = source.map(|s| s.inner.as_ref());
        let tx = self.tx.lock().await;
        let stream = tx
            .as_ref()
            .ok_or(napi::Error::new(
                napi::Status::GenericFailure,
                "transaction closed",
            ))?
            .query_raw(&query, values, source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        Ok(Cursor {
            stream: tokio::sync::Mutex::new(stream),
        })
    }

    #[napi]
    pub async fn driver_query(
        &self,
        query: String,
        params: Vec<DriverParamValue>,
        source: Option<&Request>,
    ) -> napi::Result<DriverQueryResult> {
        let values = convert_driver_params(params);
        let source = source.map(|s| s.inner.as_ref());
        let tx = self.tx.lock().await;
        let cursor = tx
            .as_ref()
            .ok_or(napi::Error::new(
                napi::Status::GenericFailure,
                "transaction closed",
            ))?
            .query_raw_text(&query, values, source)
            .await
            .map_err(to_napi_err)?;
        collect_driver_result(cursor).await
    }
}

#[napi]
pub struct Cursor {
    stream: tokio::sync::Mutex<sqldb::Cursor>,
}

#[napi]
pub struct Row {
    row: sqldb::Row,
}

#[napi]
impl Row {
    #[napi]
    pub fn values(&self, env: Env) -> napi::Result<HashMap<String, JsUnknown>> {
        let vals = self.row.values()?;
        let mut map = HashMap::with_capacity(vals.len());
        for (key, val) in vals {
            let val: JsUnknown = match val {
                sqldb::RowValue::PVal(val) => pvalue_to_js(env, &val)?,
                sqldb::RowValue::Bytes(val) => {
                    env.create_arraybuffer_with_data(val)?.into_unknown()
                }
                sqldb::RowValue::Uuid(val) => env.create_string(&val.to_string())?.into_unknown(),
                sqldb::RowValue::Cidr(val) => env.create_string(&val.to_string())?.into_unknown(),
                sqldb::RowValue::Inet(val) => env.create_string(&val.to_string())?.into_unknown(),
            };
            map.insert(key, val);
        }
        Ok(map)
    }
}

#[napi]
impl Cursor {
    #[napi]
    pub async fn next(&self) -> napi::Result<Option<Row>> {
        let mut stream = self.stream.lock().await;
        let row = stream
            .next()
            .await
            .transpose()
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, format!("{e:#?}")))?;

        Ok(row.map(|row| Row { row }))
    }
}

#[napi]
impl SQLDatabase {
    #[napi]
    pub async fn acquire(&self) -> napi::Result<SQLConn> {
        let conn = self.pool()?.acquire().await.map_err(to_napi_err)?;
        log::info!("acquired connection");
        Ok(SQLConn {
            inner: Arc::new(conn),
        })
    }
}

#[napi]
pub struct SQLConn {
    inner: Arc<sqldb::Connection>,
}

#[napi]
impl SQLConn {
    #[napi]
    pub async fn close(&self) {
        self.inner.close().await
    }

    #[napi]
    pub async fn query(
        &self,
        query: String,
        args: &QueryArgs,
        source: Option<&Request>,
    ) -> napi::Result<Cursor> {
        let values: Vec<_> = args.values.lock().unwrap().drain(..).collect();
        let source = source.map(|s| s.inner.as_ref());
        let stream = self
            .inner
            .query_raw(&query, values, source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        Ok(Cursor {
            stream: tokio::sync::Mutex::new(stream),
        })
    }

    #[napi]
    pub async fn query_row(
        &self,
        query: String,
        args: &QueryArgs,
        source: Option<&Request>,
    ) -> napi::Result<Option<Row>> {
        let values: Vec<_> = args.values.lock().unwrap().drain(..).collect();
        let source = source.map(|s| s.inner.as_ref());
        let mut stream = self
            .inner
            .query_raw(&query, values, source)
            .await
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        let row = stream
            .next()
            .await
            .transpose()
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, e.to_string()))?;
        Ok(row.map(|row| Row { row }))
    }

    #[napi]
    pub async fn driver_query(
        &self,
        query: String,
        params: Vec<DriverParamValue>,
        source: Option<&Request>,
    ) -> napi::Result<DriverQueryResult> {
        let values = convert_driver_params(params);
        let source = source.map(|s| s.inner.as_ref());
        let cursor = self
            .inner
            .query_raw_text(&query, values, source)
            .await
            .map_err(to_napi_err)?;
        collect_driver_result(cursor).await
    }
}

fn to_napi_err<E: Display>(e: E) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, e.to_string())
}
