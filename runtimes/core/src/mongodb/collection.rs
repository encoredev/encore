use std::future::Future;
use std::sync::Arc;
use std::time::Duration;

use futures::TryStreamExt;
use mongodb::action::Action;
use mongodb::bson::{doc, Bson, Document};
use mongodb::options::IndexOptions;
use mongodb::{ClientSession, IndexModel};
use tokio::sync::Mutex;

use crate::model::Request;
use crate::mongodb::manager::Connection;
use crate::trace::protocol::{MongoCallEndData, MongoCallStartData};

/// The longest query recorded in a trace event, in bytes.
const MAX_TRACED_QUERY_LEN: usize = 4096;

/// A MongoDB collection.
///
/// If it was created from a [Transaction], every operation runs
/// in that transaction.
pub struct Collection {
    conn: Arc<Connection>,
    name: String,
    session: Option<Arc<Mutex<ClientSession>>>,
}

/// Options for finding documents.
#[derive(Debug, Default)]
pub struct FindOptions {
    pub sort: Option<Document>,
    pub projection: Option<Document>,
    pub skip: Option<u64>,
    pub limit: Option<i64>,
}

/// The result of an update or replace.
#[derive(Debug)]
pub struct UpdateResult {
    pub matched_count: u64,
    pub modified_count: u64,
    pub upserted_id: Option<Bson>,
}

/// Options for creating an index.
#[derive(Debug, Default)]
pub struct CreateIndexOptions {
    pub name: Option<String>,
    pub unique: Option<bool>,
    pub sparse: Option<bool>,
    pub expire_after_seconds: Option<u64>,
}

/// An error from a MongoDB operation, with the server's error code if there is one.
#[derive(Debug)]
pub struct Error {
    /// The MongoDB error code, e.g. 11000 for a duplicate key.
    pub code: Option<i32>,
    /// The MongoDB error labels, e.g. "TransientTransactionError",
    /// which tell whether a transaction can be retried.
    pub labels: Vec<String>,
    pub source: anyhow::Error,
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#}", self.source)
    }
}

impl From<anyhow::Error> for Error {
    fn from(source: anyhow::Error) -> Self {
        Self {
            code: None,
            labels: Vec::new(),
            source,
        }
    }
}

impl From<mongodb::error::Error> for Error {
    fn from(err: mongodb::error::Error) -> Self {
        let mut labels: Vec<String> = err.labels().iter().cloned().collect();
        labels.sort();
        Self {
            code: error_code(&err),
            labels,
            // The error kind's message is the readable part; the full error
            // also prints the server's raw response bytes.
            source: anyhow::anyhow!("{}", err.kind),
        }
    }
}

pub type Result<T> = std::result::Result<T, Error>;

/// Returns the MongoDB server error code of err, if any.
fn error_code(err: &mongodb::error::Error) -> Option<i32> {
    use mongodb::error::{ErrorKind, WriteFailure};
    match err.kind.as_ref() {
        ErrorKind::Command(e) => Some(e.code),
        ErrorKind::Write(WriteFailure::WriteError(e)) => Some(e.code),
        ErrorKind::Write(WriteFailure::WriteConcernError(e)) => Some(e.code),
        ErrorKind::InsertMany(e) => e
            .write_errors
            .as_ref()
            .and_then(|errs| errs.first())
            .map(|e| e.code)
            .or(e.write_concern_error.as_ref().map(|e| e.code)),
        _ => None,
    }
}

impl Collection {
    pub(super) fn new(
        conn: Arc<Connection>,
        name: String,
        session: Option<Arc<Mutex<ClientSession>>>,
    ) -> Self {
        Self {
            conn,
            name,
            session,
        }
    }

    async fn coll(&self) -> anyhow::Result<mongodb::Collection<Document>> {
        Ok(self.conn.database().await?.collection(&self.name))
    }

    /// Runs f, recording it as a MongoDB call in the trace if source is traced.
    async fn traced<T, F, Fut>(
        &self,
        source: Option<&Request>,
        operation: &'static str,
        query: Option<String>,
        f: F,
    ) -> Result<T>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<T>>,
    {
        let start = source.map(|source| {
            let query = truncate(query.unwrap_or_default());
            let start_id = self.conn.tracer.mongo_call_start(MongoCallStartData {
                source,
                database: self.conn.encore_name.as_ref(),
                collection: &self.name,
                operation,
                query: &query,
            });
            (start_id, source)
        });

        let result = f().await;

        if let Some((start_id, source)) = start {
            self.conn.tracer.mongo_call_end(MongoCallEndData {
                start_id,
                source,
                error: result.as_ref().err(),
            });
        }
        result
    }

    /// Inserts a document and returns its `_id`.
    pub async fn insert_one(&self, doc: Document, source: Option<&Request>) -> Result<Bson> {
        self.traced(
            source,
            "insertOne",
            source.map(|_| to_json(&doc)),
            || async {
                let coll = self.coll().await?;
                let res = match &self.session {
                    Some(s) => coll.insert_one(doc).session(&mut *s.lock().await).await?,
                    None => coll.insert_one(doc).await?,
                };
                Ok(res.inserted_id)
            },
        )
        .await
    }

    /// Inserts documents and returns their `_id`s, in order.
    pub async fn insert_many(
        &self,
        docs: Vec<Document>,
        source: Option<&Request>,
    ) -> Result<Vec<Bson>> {
        self.traced(
            source,
            "insertMany",
            source.map(|_| to_json_array(&docs)),
            || async {
                let n = docs.len();
                let coll = self.coll().await?;
                let res = match &self.session {
                    Some(s) => coll.insert_many(docs).session(&mut *s.lock().await).await?,
                    None => coll.insert_many(docs).await?,
                };
                let mut ids = res.inserted_ids;
                Ok((0..n)
                    .map(|i| ids.remove(&i).unwrap_or(Bson::Null))
                    .collect())
            },
        )
        .await
    }

    /// Finds the first document matching the filter.
    pub async fn find_one(
        &self,
        filter: Document,
        opts: FindOptions,
        source: Option<&Request>,
    ) -> Result<Option<Document>> {
        self.traced(
            source,
            "findOne",
            source.map(|_| to_json(&filter)),
            || async {
                let coll = self.coll().await?;
                let action = coll
                    .find_one(filter)
                    .optional(opts.sort, |a, v| a.sort(v))
                    .optional(opts.projection, |a, v| a.projection(v))
                    .optional(opts.skip, |a, v| a.skip(v));
                let doc = match &self.session {
                    Some(s) => action.session(&mut *s.lock().await).await?,
                    None => action.await?,
                };
                Ok(doc)
            },
        )
        .await
    }

    /// Finds all documents matching the filter.
    pub async fn find(
        &self,
        filter: Document,
        opts: FindOptions,
        source: Option<&Request>,
    ) -> Result<Vec<Document>> {
        self.traced(source, "find", source.map(|_| to_json(&filter)), || async {
            let coll = self.coll().await?;
            let action = coll
                .find(filter)
                .optional(opts.sort, |a, v| a.sort(v))
                .optional(opts.projection, |a, v| a.projection(v))
                .optional(opts.skip, |a, v| a.skip(v))
                .optional(opts.limit, |a, v| a.limit(v));
            let docs: Vec<Document> = match &self.session {
                Some(s) => {
                    let mut session = s.lock().await;
                    let mut cursor = action.session(&mut *session).await?;
                    cursor.stream(&mut *session).try_collect().await?
                }
                None => action.await?.try_collect().await?,
            };
            Ok(docs)
        })
        .await
    }

    /// Updates the first document matching the filter.
    pub async fn update_one(
        &self,
        filter: Document,
        update: Document,
        upsert: bool,
        source: Option<&Request>,
    ) -> Result<UpdateResult> {
        self.traced(
            source,
            "updateOne",
            source.map(|_| to_json(&doc! {"filter": &filter, "update": &update})),
            || async {
                let coll = self.coll().await?;
                let action = coll.update_one(filter, update).upsert(upsert);
                let res = match &self.session {
                    Some(s) => action.session(&mut *s.lock().await).await?,
                    None => action.await?,
                };
                Ok(update_result(res))
            },
        )
        .await
    }

    /// Updates all documents matching the filter.
    pub async fn update_many(
        &self,
        filter: Document,
        update: Document,
        upsert: bool,
        source: Option<&Request>,
    ) -> Result<UpdateResult> {
        self.traced(
            source,
            "updateMany",
            source.map(|_| to_json(&doc! {"filter": &filter, "update": &update})),
            || async {
                let coll = self.coll().await?;
                let action = coll.update_many(filter, update).upsert(upsert);
                let res = match &self.session {
                    Some(s) => action.session(&mut *s.lock().await).await?,
                    None => action.await?,
                };
                Ok(update_result(res))
            },
        )
        .await
    }

    /// Replaces the first document matching the filter.
    pub async fn replace_one(
        &self,
        filter: Document,
        replacement: Document,
        upsert: bool,
        source: Option<&Request>,
    ) -> Result<UpdateResult> {
        self.traced(
            source,
            "replaceOne",
            source.map(|_| to_json(&doc! {"filter": &filter, "replacement": &replacement})),
            || async {
                let coll = self.coll().await?;
                let action = coll.replace_one(filter, replacement).upsert(upsert);
                let res = match &self.session {
                    Some(s) => action.session(&mut *s.lock().await).await?,
                    None => action.await?,
                };
                Ok(update_result(res))
            },
        )
        .await
    }

    /// Deletes the first document matching the filter, and reports how many were deleted.
    pub async fn delete_one(&self, filter: Document, source: Option<&Request>) -> Result<u64> {
        self.traced(
            source,
            "deleteOne",
            source.map(|_| to_json(&filter)),
            || async {
                let coll = self.coll().await?;
                let res = match &self.session {
                    Some(s) => {
                        coll.delete_one(filter)
                            .session(&mut *s.lock().await)
                            .await?
                    }
                    None => coll.delete_one(filter).await?,
                };
                Ok(res.deleted_count)
            },
        )
        .await
    }

    /// Deletes all documents matching the filter, and reports how many were deleted.
    pub async fn delete_many(&self, filter: Document, source: Option<&Request>) -> Result<u64> {
        self.traced(
            source,
            "deleteMany",
            source.map(|_| to_json(&filter)),
            || async {
                let coll = self.coll().await?;
                let res = match &self.session {
                    Some(s) => {
                        coll.delete_many(filter)
                            .session(&mut *s.lock().await)
                            .await?
                    }
                    None => coll.delete_many(filter).await?,
                };
                Ok(res.deleted_count)
            },
        )
        .await
    }

    /// Counts the documents matching the filter.
    pub async fn count_documents(&self, filter: Document, source: Option<&Request>) -> Result<u64> {
        self.traced(
            source,
            "countDocuments",
            source.map(|_| to_json(&filter)),
            || async {
                let coll = self.coll().await?;
                Ok(match &self.session {
                    Some(s) => {
                        coll.count_documents(filter)
                            .session(&mut *s.lock().await)
                            .await?
                    }
                    None => coll.count_documents(filter).await?,
                })
            },
        )
        .await
    }

    /// Runs an aggregation pipeline and returns all resulting documents.
    pub async fn aggregate(
        &self,
        pipeline: Vec<Document>,
        source: Option<&Request>,
    ) -> Result<Vec<Document>> {
        self.traced(
            source,
            "aggregate",
            source.map(|_| to_json_array(&pipeline)),
            || async {
                let coll = self.coll().await?;
                let docs: Vec<Document> = match &self.session {
                    Some(s) => {
                        let mut session = s.lock().await;
                        let mut cursor = coll.aggregate(pipeline).session(&mut *session).await?;
                        cursor.stream(&mut *session).try_collect().await?
                    }
                    None => coll.aggregate(pipeline).await?.try_collect().await?,
                };
                Ok(docs)
            },
        )
        .await
    }

    /// Creates an index and returns its name.
    pub async fn create_index(
        &self,
        keys: Document,
        opts: CreateIndexOptions,
        source: Option<&Request>,
    ) -> Result<String> {
        self.traced(
            source,
            "createIndex",
            source.map(|_| to_json(&keys)),
            || async {
                let mut options = IndexOptions::default();
                options.name = opts.name;
                options.unique = opts.unique;
                options.sparse = opts.sparse;
                options.expire_after = opts.expire_after_seconds.map(Duration::from_secs);
                let mut model = IndexModel::default();
                model.keys = keys;
                model.options = Some(options);
                let res = self.coll().await?.create_index(model).await?;
                Ok(res.index_name)
            },
        )
        .await
    }

    /// Lists the indexes on the collection.
    pub async fn list_indexes(&self, source: Option<&Request>) -> Result<Vec<Document>> {
        self.traced(source, "listIndexes", None, || async {
            let cursor = self.coll().await?.list_indexes().await?;
            let indexes: Vec<IndexModel> = cursor.try_collect().await?;
            let docs = indexes
                .into_iter()
                .map(|model| {
                    let mut doc = doc! {"key": model.keys};
                    if let Some(name) = model.options.and_then(|o| o.name) {
                        doc.insert("name", name);
                    }
                    doc
                })
                .collect();
            Ok(docs)
        })
        .await
    }

    /// Drops the index with the given name.
    pub async fn drop_index(&self, name: String, source: Option<&Request>) -> Result<()> {
        self.traced(
            source,
            "dropIndex",
            source.map(|_| name.clone()),
            || async {
                self.coll().await?.drop_index(name.clone()).await?;
                Ok(())
            },
        )
        .await
    }
}

/// A MongoDB transaction.
///
/// Operations on collections from [Transaction::collection] run in the
/// transaction, until it is committed or aborted.
pub struct Transaction {
    conn: Arc<Connection>,
    session: Arc<Mutex<ClientSession>>,
}

impl Transaction {
    /// Starts a session and a transaction on it.
    pub(super) async fn start(conn: Arc<Connection>) -> Result<Self> {
        let client = conn.client().await?;
        let mut session = client.start_session().await?;
        session.start_transaction().await?;
        Ok(Self {
            conn,
            session: Arc::new(Mutex::new(session)),
        })
    }

    /// Returns the collection with the given name, bound to this transaction.
    pub fn collection(&self, name: &str) -> Collection {
        Collection::new(
            self.conn.clone(),
            name.to_string(),
            Some(self.session.clone()),
        )
    }

    pub async fn commit(&self, source: Option<&Request>) -> Result<()> {
        let coll = self.collection("");
        coll.traced(source, "commitTransaction", None, || async {
            self.session.lock().await.commit_transaction().await?;
            Ok(())
        })
        .await
    }

    pub async fn abort(&self, source: Option<&Request>) -> Result<()> {
        let coll = self.collection("");
        coll.traced(source, "abortTransaction", None, || async {
            self.session.lock().await.abort_transaction().await?;
            Ok(())
        })
        .await
    }
}

fn update_result(res: mongodb::results::UpdateResult) -> UpdateResult {
    UpdateResult {
        matched_count: res.matched_count,
        modified_count: res.modified_count,
        upserted_id: res.upserted_id,
    }
}

/// Renders a document as relaxed Extended JSON, for traces.
fn to_json(doc: &Document) -> String {
    Bson::Document(doc.clone())
        .into_relaxed_extjson()
        .to_string()
}

fn to_json_array(docs: &[Document]) -> String {
    Bson::Array(docs.iter().cloned().map(Bson::Document).collect())
        .into_relaxed_extjson()
        .to_string()
}

fn truncate(mut s: String) -> String {
    if s.len() > MAX_TRACED_QUERY_LEN {
        let mut end = MAX_TRACED_QUERY_LEN;
        while !s.is_char_boundary(end) {
            end -= 1;
        }
        s.truncate(end);
        s.push_str("...");
    }
    s
}
