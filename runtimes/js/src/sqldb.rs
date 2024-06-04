use crate::api::Request;
use encore_runtime_core::sqldb;
use mappable_rc::Marc;
use napi::{Env, JsUnknown};
use napi_derive::napi;
use std::collections::HashMap;
use std::fmt::Display;
use std::sync::{Arc, OnceLock};

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
    pub fn new(env: Env, params: Vec<JsUnknown>) -> napi::Result<Self> {
        use napi::ValueType;
        let values: napi::Result<Vec<_>> = params
            .into_iter()
            .map(|val| -> napi::Result<sqldb::RowValue> {
                Ok(match val.get_type()? {
                    ValueType::Null => sqldb::RowValue::Json(serde_json::Value::Null),
                    ValueType::Number => {
                        let float = val.coerce_to_number()?.get_double()?;
                        let int = float as i64;
                        if float == int as f64 {
                            sqldb::RowValue::Json(serde_json::Value::Number(int.into()))
                        } else {
                            match serde_json::Number::from_f64(float) {
                                Some(n) => sqldb::RowValue::Json(serde_json::Value::Number(n)),
                                None => {
                                    return Err(napi::Error::new(
                                        napi::Status::GenericFailure,
                                        "failed to convert float to json number".to_string(),
                                    ));
                                }
                            }
                        }
                    }
                    ValueType::Boolean => {
                        let b = val.coerce_to_bool()?.get_value()?;
                        sqldb::RowValue::Json(serde_json::Value::Bool(b))
                    }
                    ValueType::String => {
                        let s = val.coerce_to_string()?.into_utf8()?.into_owned()?;
                        sqldb::RowValue::Json(serde_json::Value::String(s))
                    }
                    ValueType::Object => {
                        let val: serde_json::Value = env.from_js_value(val)?;
                        sqldb::RowValue::Json(val)
                    }
                    ValueType::Unknown => {
                        return Err(napi::Error::new(
                            napi::Status::GenericFailure,
                            "unknown not yet supported".to_string(),
                        ));
                    }
                    ValueType::BigInt => {
                        return Err(napi::Error::new(
                            napi::Status::GenericFailure,
                            "unsupported value type".to_string(),
                        ));
                    }
                    _ => {
                        return Err(napi::Error::new(
                            napi::Status::GenericFailure,
                            "unsupported value type".to_string(),
                        ));
                    }
                })
            })
            .collect();

        let values = values?;
        Ok(Self {
            values: std::sync::Mutex::new(values),
        })
    }
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
                sqldb::RowValue::Json(val) => env.to_js_value(&val)?,
                sqldb::RowValue::Bytes(val) => {
                    env.create_arraybuffer_with_data(val)?.into_unknown()
                }
                sqldb::RowValue::Uuid(val) => env.create_string(&val.to_string())?.into_unknown(),
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
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, format!("{:#?}", e)))?;

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
}

fn to_napi_err<E: Display>(e: E) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, e.to_string())
}
