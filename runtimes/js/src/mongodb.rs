use std::sync::Arc;

use encore_runtime_core::mongodb;
use napi::{Error, Status};
use napi_derive::napi;

use crate::api::Request;
use crate::mongodb_bson::{BsonDoc, BsonOut};
use encore_runtime_core::mongodb::bson::Bson;

#[napi]
pub struct MongoDatabase {
    inner: Arc<dyn mongodb::Database>,
}

#[napi]
impl MongoDatabase {
    pub fn new(inner: Arc<dyn mongodb::Database>) -> Self {
        Self { inner }
    }

    /// Returns a handle to the collection with the given name.
    /// If the database is not configured for this service,
    /// every operation on the collection reports an error.
    #[napi]
    pub fn collection(&self, name: String) -> MongoCollection {
        MongoCollection {
            inner: self.inner.collection(&name).map_err(|e| format!("{e:#}")),
        }
    }

    /// Starts a transaction.
    #[napi]
    pub async fn begin(&self) -> napi::Result<MongoTransaction> {
        let tx = mongodb::begin(self.inner.as_ref())
            .await
            .map_err(to_napi_err)?;
        Ok(MongoTransaction { tx: Arc::new(tx) })
    }
}

#[napi]
pub struct MongoTransaction {
    tx: Arc<mongodb::Transaction>,
}

#[napi]
impl MongoTransaction {
    /// Returns the collection with the given name, bound to this transaction.
    #[napi]
    pub fn collection(&self, name: String) -> MongoCollection {
        MongoCollection {
            inner: Ok(self.tx.collection(&name)),
        }
    }

    #[napi]
    pub async fn commit(&self, source: Option<&Request>) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.tx.commit(source).await.map_err(to_napi_err)
    }

    #[napi]
    pub async fn abort(&self, source: Option<&Request>) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.tx.abort(source).await.map_err(to_napi_err)
    }
}

#[napi(object)]
pub struct MongoUpdateResult {
    pub matched_count: i64,
    pub modified_count: i64,
    #[napi(ts_type = "any")]
    pub upserted_id: Option<BsonOut>,
}

#[napi(object)]
pub struct MongoCreateIndexOptions {
    pub name: Option<String>,
    pub unique: Option<bool>,
    pub sparse: Option<bool>,
    pub expire_after_seconds: Option<i64>,
}

#[napi]
pub struct MongoCollection {
    inner: Result<mongodb::Collection, String>,
}

#[napi]
impl MongoCollection {
    fn coll(&self) -> napi::Result<&mongodb::Collection> {
        self.inner
            .as_ref()
            .map_err(|e| Error::new(Status::GenericFailure, e.clone()))
    }

    #[napi(
        ts_args_type = "doc: Record<string, any>, source?: Request | null",
        ts_return_type = "Promise<any>"
    )]
    pub async fn insert_one(
        &self,
        doc: BsonDoc,
        source: Option<&Request>,
    ) -> napi::Result<BsonOut> {
        let source = source.map(|s| s.inner.as_ref());
        let doc = into_doc(doc)?;
        let id = self
            .coll()?
            .insert_one(doc, source)
            .await
            .map_err(to_napi_err)?;
        Ok(BsonOut(id))
    }

    #[napi(
        ts_args_type = "docs: Array<Record<string, any>>, source?: Request | null",
        ts_return_type = "Promise<any[]>"
    )]
    pub async fn insert_many(
        &self,
        docs: Vec<BsonDoc>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<BsonOut>> {
        let source = source.map(|s| s.inner.as_ref());
        let docs = docs.into_iter().map(|d| d.0).collect::<Vec<_>>();
        let ids = self
            .coll()?
            .insert_many(docs, source)
            .await
            .map_err(to_napi_err)?;
        Ok(ids.into_iter().map(BsonOut).collect())
    }

    #[napi(
        ts_args_type = "filter: Record<string, any>, sort?: Record<string, any> | null, projection?: Record<string, any> | null, skip?: number | null, source?: Request | null",
        ts_return_type = "Promise<Record<string, any> | null>"
    )]
    pub async fn find_one(
        &self,
        filter: BsonDoc,
        sort: Option<BsonDoc>,
        projection: Option<BsonDoc>,
        skip: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<BsonOut>> {
        let source = source.map(|s| s.inner.as_ref());
        let opts = find_options(sort, projection, skip, None)?;
        let doc = self
            .coll()?
            .find_one(into_doc(filter)?, opts, source)
            .await
            .map_err(to_napi_err)?;
        Ok(doc.map(|d| BsonOut(Bson::Document(d))))
    }

    #[napi(
        ts_args_type = "filter: Record<string, any>, sort?: Record<string, any> | null, projection?: Record<string, any> | null, skip?: number | null, limit?: number | null, source?: Request | null",
        ts_return_type = "Promise<Record<string, any>[]>"
    )]
    pub async fn find(
        &self,
        filter: BsonDoc,
        sort: Option<BsonDoc>,
        projection: Option<BsonDoc>,
        skip: Option<i64>,
        limit: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<BsonOut>> {
        let source = source.map(|s| s.inner.as_ref());
        let opts = find_options(sort, projection, skip, limit)?;
        let docs = self
            .coll()?
            .find(into_doc(filter)?, opts, source)
            .await
            .map_err(to_napi_err)?;
        Ok(docs
            .into_iter()
            .map(|d| BsonOut(Bson::Document(d)))
            .collect())
    }

    #[napi(
        ts_args_type = "filter: Record<string, any>, update: Record<string, any>, upsert?: boolean | null, source?: Request | null"
    )]
    pub async fn update_one(
        &self,
        filter: BsonDoc,
        update: BsonDoc,
        upsert: Option<bool>,
        source: Option<&Request>,
    ) -> napi::Result<MongoUpdateResult> {
        let source = source.map(|s| s.inner.as_ref());
        let res = self
            .coll()?
            .update_one(
                into_doc(filter)?,
                into_doc(update)?,
                upsert.unwrap_or(false),
                source,
            )
            .await
            .map_err(to_napi_err)?;
        Ok(update_result(res))
    }

    #[napi(
        ts_args_type = "filter: Record<string, any>, update: Record<string, any>, upsert?: boolean | null, source?: Request | null"
    )]
    pub async fn update_many(
        &self,
        filter: BsonDoc,
        update: BsonDoc,
        upsert: Option<bool>,
        source: Option<&Request>,
    ) -> napi::Result<MongoUpdateResult> {
        let source = source.map(|s| s.inner.as_ref());
        let res = self
            .coll()?
            .update_many(
                into_doc(filter)?,
                into_doc(update)?,
                upsert.unwrap_or(false),
                source,
            )
            .await
            .map_err(to_napi_err)?;
        Ok(update_result(res))
    }

    #[napi(
        ts_args_type = "filter: Record<string, any>, replacement: Record<string, any>, upsert?: boolean | null, source?: Request | null"
    )]
    pub async fn replace_one(
        &self,
        filter: BsonDoc,
        replacement: BsonDoc,
        upsert: Option<bool>,
        source: Option<&Request>,
    ) -> napi::Result<MongoUpdateResult> {
        let source = source.map(|s| s.inner.as_ref());
        let res = self
            .coll()?
            .replace_one(
                into_doc(filter)?,
                into_doc(replacement)?,
                upsert.unwrap_or(false),
                source,
            )
            .await
            .map_err(to_napi_err)?;
        Ok(update_result(res))
    }

    #[napi(ts_args_type = "filter: Record<string, any>, source?: Request | null")]
    pub async fn delete_one(&self, filter: BsonDoc, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let n = self
            .coll()?
            .delete_one(into_doc(filter)?, source)
            .await
            .map_err(to_napi_err)?;
        Ok(n as i64)
    }

    #[napi(ts_args_type = "filter: Record<string, any>, source?: Request | null")]
    pub async fn delete_many(
        &self,
        filter: BsonDoc,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let n = self
            .coll()?
            .delete_many(into_doc(filter)?, source)
            .await
            .map_err(to_napi_err)?;
        Ok(n as i64)
    }

    #[napi(ts_args_type = "filter: Record<string, any>, source?: Request | null")]
    pub async fn count_documents(
        &self,
        filter: BsonDoc,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let n = self
            .coll()?
            .count_documents(into_doc(filter)?, source)
            .await
            .map_err(to_napi_err)?;
        Ok(n as i64)
    }

    #[napi(
        ts_args_type = "pipeline: Array<Record<string, any>>, source?: Request | null",
        ts_return_type = "Promise<Record<string, any>[]>"
    )]
    pub async fn aggregate(
        &self,
        pipeline: Vec<BsonDoc>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<BsonOut>> {
        let source = source.map(|s| s.inner.as_ref());
        let pipeline = pipeline.into_iter().map(|d| d.0).collect::<Vec<_>>();
        let docs = self
            .coll()?
            .aggregate(pipeline, source)
            .await
            .map_err(to_napi_err)?;
        Ok(docs
            .into_iter()
            .map(|d| BsonOut(Bson::Document(d)))
            .collect())
    }

    #[napi(
        ts_args_type = "keys: Record<string, any>, opts?: MongoCreateIndexOptions | null, source?: Request | null"
    )]
    pub async fn create_index(
        &self,
        keys: BsonDoc,
        opts: Option<MongoCreateIndexOptions>,
        source: Option<&Request>,
    ) -> napi::Result<String> {
        let source = source.map(|s| s.inner.as_ref());
        let opts = opts.map_or_else(mongodb::CreateIndexOptions::default, |o| {
            mongodb::CreateIndexOptions {
                name: o.name,
                unique: o.unique,
                sparse: o.sparse,
                expire_after_seconds: o.expire_after_seconds.map(|n| n.max(0) as u64),
            }
        });
        self.coll()?
            .create_index(into_doc(keys)?, opts, source)
            .await
            .map_err(to_napi_err)
    }

    #[napi(
        ts_args_type = "source?: Request | null",
        ts_return_type = "Promise<Record<string, any>[]>"
    )]
    pub async fn list_indexes(&self, source: Option<&Request>) -> napi::Result<Vec<BsonOut>> {
        let source = source.map(|s| s.inner.as_ref());
        let indexes = self
            .coll()?
            .list_indexes(source)
            .await
            .map_err(to_napi_err)?;
        Ok(indexes
            .into_iter()
            .map(|d| BsonOut(Bson::Document(d)))
            .collect())
    }

    #[napi]
    pub async fn drop_index(&self, name: String, source: Option<&Request>) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.coll()?
            .drop_index(name, source)
            .await
            .map_err(to_napi_err)
    }
}

fn into_doc(val: BsonDoc) -> napi::Result<mongodb::Document> {
    Ok(val.0)
}

fn find_options(
    sort: Option<BsonDoc>,
    projection: Option<BsonDoc>,
    skip: Option<i64>,
    limit: Option<i64>,
) -> napi::Result<mongodb::FindOptions> {
    Ok(mongodb::FindOptions {
        sort: sort.map(|d| d.0),
        projection: projection.map(|d| d.0),
        skip: skip.map(|n| n.max(0) as u64),
        limit,
    })
}

fn update_result(res: mongodb::UpdateResult) -> MongoUpdateResult {
    MongoUpdateResult {
        matched_count: res.matched_count as i64,
        modified_count: res.modified_count as i64,
        upserted_id: res.upserted_id.map(BsonOut),
    }
}

/// Converts an error to a napi error. If MongoDB reported an error code
/// or error labels, the message starts with
/// "mongodb error [code=<code>] [labels=<label>,<label>]: ", which the
/// JavaScript SDK turns into a MongoError with that code and those labels.
fn to_napi_err(e: mongodb::Error) -> Error {
    let mut prefix = String::new();
    if let Some(code) = e.code {
        prefix.push_str(&format!(" code={code}"));
    }
    if !e.labels.is_empty() {
        prefix.push_str(&format!(" labels={}", e.labels.join(",")));
    }
    let msg = if prefix.is_empty() {
        e.to_string()
    } else {
        format!("mongodb error{prefix}: {e}")
    };
    Error::new(Status::GenericFailure, msg)
}
