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
mod s3old;

trait ClusterImpl: Debug + Send + Sync {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn BucketImpl + 'static>;
}

trait BucketImpl: Debug + Send + Sync {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn ObjectImpl + 'static>;

    fn list(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<ListStream, Error>> + Send + 'static>>;
}

pub type ListStream = Box<dyn Stream<Item = Result<ListEntry, Error>> + Send>;

trait ObjectImpl: Debug + Send + Sync {
    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send>>;

    fn upload(
        self: Arc<Self>,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send>>;

    fn download(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<DownloadStream, DownloadError>> + Send>>;

    fn attrs(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>>;

    fn delete(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>>;
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

    pub async fn list(&self) -> Result<ListStream, Error> {
        self.imp.clone().list().await
    }
}

#[derive(Debug)]
pub struct Object {
    _tracer: Tracer,
    imp: Arc<dyn ObjectImpl>,
}

impl Object {
    pub async fn exists(&self) -> anyhow::Result<bool> {
        self.imp.clone().exists().await
    }

    pub fn upload(
        &self,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
    ) -> impl Future<Output = anyhow::Result<()>> + Send + 'static {
        self.imp.clone().upload(data, options)
    }

    pub fn download_stream(
        &self,
    ) -> impl Future<Output = Result<DownloadStream, DownloadError>> + Send + 'static {
        self.imp.clone().download()
    }

    pub fn download_all(
        &self,
    ) -> impl Future<Output = Result<Vec<u8>, DownloadError>> + Send + 'static {
        let stream = self.imp.clone().download();
        async move {
            let mut bytes = Vec::new();
            let mut stream = stream.await?;

            while let Some(chunk) = stream.next().await {
                bytes.extend_from_slice(&chunk?);
            }
            Ok(bytes)
        }
    }

    pub async fn attrs(&self) -> Result<ObjectAttrs, Error> {
        self.imp.clone().attrs().await
    }

    pub async fn delete(&self) -> Result<(), Error> {
        self.imp.clone().delete().await
    }
}

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("object not found")]
    NotFound,

    #[error("internal error: {0:?}")]
    Internal(anyhow::Error),

    #[error("{0:?}")]
    Other(anyhow::Error),
}

#[derive(thiserror::Error, Debug)]
pub enum DownloadError {
    #[error("object not found")]
    NotFound,

    #[error("internal error: {0:?}")]
    Internal(anyhow::Error),

    #[error("{0:?}")]
    Other(anyhow::Error),
}

pub type DownloadStream = Pin<Box<dyn Stream<Item = Result<Bytes, DownloadError>> + Send>>;

pub struct ObjectAttrs {
    pub name: String,
    pub version: String,
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
