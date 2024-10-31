use bytes::Bytes;
use futures::{Stream, StreamExt};
use std::future::Future;
use std::sync::Arc;
use std::{fmt::Debug, pin::Pin};
use tokio::io::AsyncRead;

pub use manager::Manager;

use crate::encore::runtime::v1 as pb;
use crate::trace::Tracer;

mod gcs;
mod manager;
mod noop;
mod s3;

trait ClusterImpl: Debug + Send + Sync {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn BucketImpl + 'static>;
}

trait BucketImpl: Debug + Send + Sync {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn ObjectImpl + 'static>;

    fn list(
        self: Arc<Self>,
        options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ListStream, Error>> + Send + 'static>>;
}

pub type ListStream = Box<dyn Stream<Item = Result<ListEntry, Error>> + Send>;

trait ObjectImpl: Debug + Send + Sync {
    fn exists(
        self: Arc<Self>,
        version: Option<String>,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>>;

    fn upload(
        self: Arc<Self>,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>>;

    fn download(
        self: Arc<Self>,
        options: DownloadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<DownloadStream, Error>> + Send>>;

    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>>;

    fn delete(
        self: Arc<Self>,
        options: DeleteOptions,
    ) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>>;
}

#[derive(Debug)]
pub struct Bucket {
    tracer: Tracer,
    imp: Arc<dyn BucketImpl>,
}

impl Bucket {
    pub fn object(&self, name: String) -> Object {
        Object {
            imp: self.imp.clone().object(name),
            _tracer: self.tracer.clone(),
        }
    }

    pub async fn list(&self, options: ListOptions) -> Result<ListStream, Error> {
        self.imp.clone().list(options).await
    }
}

#[derive(Debug)]
pub struct Object {
    _tracer: Tracer,
    imp: Arc<dyn ObjectImpl>,
}

impl Object {
    pub async fn exists(&self, version: Option<String>) -> Result<bool, Error> {
        self.imp.clone().exists(version).await
    }

    pub fn upload(
        &self,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
    ) -> impl Future<Output = Result<ObjectAttrs, Error>> + Send + 'static {
        self.imp.clone().upload(data, options)
    }

    pub fn download_stream(
        &self,
        options: DownloadOptions,
    ) -> impl Future<Output = Result<DownloadStream, Error>> + Send + 'static {
        self.imp.clone().download(options)
    }

    pub fn download_all(
        &self,
        options: DownloadOptions,
    ) -> impl Future<Output = Result<Vec<u8>, Error>> + Send + 'static {
        let stream = self.imp.clone().download(options);
        async move {
            let mut bytes = Vec::new();
            let mut stream = stream.await?;

            while let Some(chunk) = stream.next().await {
                bytes.extend_from_slice(&chunk?);
            }
            Ok(bytes)
        }
    }

    pub async fn attrs(&self, options: AttrsOptions) -> Result<ObjectAttrs, Error> {
        self.imp.clone().attrs(options).await
    }

    pub async fn delete(&self, options: DeleteOptions) -> Result<(), Error> {
        self.imp.clone().delete(options).await
    }
}

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("object not found")]
    NotFound,

    #[error("precondition failed")]
    PreconditionFailed,

    #[error("internal error: {0:?}")]
    Internal(anyhow::Error),

    #[error("{0:?}")]
    Other(anyhow::Error),
}

pub type DownloadStream = Pin<Box<dyn Stream<Item = Result<Bytes, Error>> + Send>>;

pub struct ObjectAttrs {
    pub name: String,
    pub version: Option<String>,
    pub size: u64,
    pub content_type: Option<String>,
    pub etag: String,
}

pub struct ListEntry {
    pub name: String,
    pub size: u64,
    pub etag: String,
}

#[derive(Debug, Default)]
pub struct UploadOptions {
    pub content_type: Option<String>,
    pub preconditions: Option<UploadPreconditions>,
}

#[derive(Debug, Default)]
pub struct UploadPreconditions {
    pub not_exists: Option<bool>,
}

#[derive(Debug, Default)]
pub struct DownloadOptions {
    pub version: Option<String>,
}

#[derive(Debug, Default)]
pub struct AttrsOptions {
    pub version: Option<String>,
}

#[derive(Debug, Default)]
pub struct DeleteOptions {
    pub version: Option<String>,
}

#[derive(Debug, Default)]
pub struct ListOptions {
    pub prefix: Option<String>,
    pub limit: Option<u64>,
}
