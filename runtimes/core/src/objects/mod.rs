use bytes::Bytes;
use futures::{Stream, StreamExt};
use std::borrow::Cow;
use std::future::Future;
use std::sync::Arc;
use std::{fmt::Debug, pin::Pin};
use thiserror::Error;
use tokio::io::AsyncRead;

pub use manager::Manager;

use crate::encore::runtime::v1 as pb;
use crate::trace::{protocol, Tracer};
use crate::{model, EncoreName};

mod gcs;
mod manager;
mod noop;
mod s3;

trait ClusterImpl: Debug + Send + Sync {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn BucketImpl + 'static>;
}

trait BucketImpl: Debug + Send + Sync {
    #[allow(dead_code)]
    fn name(&self) -> &EncoreName;

    fn object(self: Arc<Self>, name: String) -> Arc<dyn ObjectImpl + 'static>;

    fn list(
        self: Arc<Self>,
        options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ListStream, Error>> + Send + 'static>>;
}

type ListStream = Box<dyn Stream<Item = Result<ListEntry, Error>> + Send>;

trait ObjectImpl: Debug + Send + Sync {
    fn bucket_name(&self) -> &EncoreName;
    fn key(&self) -> &str;

    fn exists(
        self: Arc<Self>,
        options: ExistsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>>;

    fn upload(
        self: Arc<Self>,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>>;

    fn signed_upload_url(
        self: Arc<Self>,
        options: UploadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, Error>> + Send>>;

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

    fn public_url(&self) -> Result<String, PublicUrlError>;
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
            tracer: self.tracer.clone(),
        }
    }

    pub async fn list(
        &self,
        options: ListOptions,
        source: Option<Arc<model::Request>>,
    ) -> Result<ListIterator, Error> {
        let (stream, start_id) = if let Some(source) = source.as_deref() {
            let start_id =
                self.tracer
                    .bucket_list_objects_start(protocol::BucketListObjectsStart {
                        source,
                        bucket: self.imp.name(),
                        prefix: options.prefix.as_deref(),
                    });

            let res = self.imp.clone().list(options).await;

            match res {
                Ok(stream) => (stream, Some(start_id)),
                Err(err) => {
                    self.tracer
                        .bucket_list_objects_end(protocol::BucketListObjectsEnd {
                            source,
                            start_id,
                            result: protocol::BucketListObjectsEndResult::Err(&err),
                        });
                    return Err(err);
                }
            }
        } else {
            let stream = self.imp.clone().list(options).await?;
            (stream, None)
        };

        Ok(ListIterator {
            stream: stream.into(),
            source,
            start_id,
            tracer: self.tracer.clone(),

            yielded_entries: 0,
            seen_end: false,
            err: None,
        })
    }
}

#[derive(Debug)]
pub struct Object {
    tracer: Tracer,
    imp: Arc<dyn ObjectImpl>,
}

#[derive(Debug, Error)]
pub enum PublicUrlError {
    #[error("bucket is not public")]
    PrivateBucket,
    #[error("invalid object name")]
    InvalidObjectName,
    #[error("public url not supported in noop bucket")]
    NoopBucket,
}

impl Object {
    pub async fn exists(
        &self,
        options: ExistsOptions,
        source: Option<Arc<model::Request>>,
    ) -> Result<bool, Error> {
        if let Some(source) = source.as_deref() {
            let start_id =
                self.tracer
                    .bucket_object_get_attrs_start(protocol::BucketObjectGetAttrsStart {
                        source,
                        bucket: self.imp.bucket_name(),
                        object: self.imp.key(),
                        version: options.version.as_deref(),
                    });
            let res = self.imp.clone().exists(options).await;

            self.tracer
                .bucket_object_get_attrs_end(protocol::BucketObjectGetAttrsEnd {
                    start_id,
                    source,
                    result: match &res {
                        Ok(true) => {
                            protocol::BucketObjectGetAttrsEndResult::Success(Default::default())
                        }
                        Ok(false) => protocol::BucketObjectGetAttrsEndResult::Err(&Error::NotFound),
                        Err(err) => protocol::BucketObjectGetAttrsEndResult::Err(err),
                    },
                });
            res
        } else {
            self.imp.clone().exists(options).await
        }
    }

    pub fn upload(
        &self,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: UploadOptions,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = Result<ObjectAttrs, Error>> + Send + 'static {
        let tracer = self.tracer.clone();
        let imp = self.imp.clone();

        async move {
            if let Some(source) = source.as_deref() {
                let start_id =
                    tracer.bucket_object_upload_start(protocol::BucketObjectUploadStart {
                        source,
                        bucket: imp.bucket_name(),
                        object: imp.key(),
                        attrs: protocol::BucketObjectAttributes {
                            content_type: options.content_type.as_deref(),
                            ..Default::default()
                        },
                    });

                let res = imp.upload(data, options).await;

                tracer.bucket_object_upload_end(protocol::BucketObjectUploadEnd {
                    start_id,
                    source,
                    result: match &res {
                        Ok(attrs) => protocol::BucketObjectUploadEndResult::Success {
                            size: attrs.size,
                            version: attrs.version.as_deref(),
                        },
                        Err(err) => protocol::BucketObjectUploadEndResult::Err(err),
                    },
                });

                res
            } else {
                imp.upload(data, options).await
            }
        }
    }

    pub async fn signed_upload_url(
        &self,
        options: UploadUrlOptions,
        _source: Option<Arc<model::Request>>,
    ) -> Result<String, Error> {
        self.imp.clone().signed_upload_url(options).await
    }

    pub fn download_stream(
        &self,
        options: DownloadOptions,
        _source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = Result<DownloadStream, Error>> + Send + 'static {
        self.imp.clone().download(options)
    }

    pub fn download_all(
        &self,
        options: DownloadOptions,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = Result<Vec<u8>, Error>> + Send + 'static {
        let tracer = self.tracer.clone();
        let imp = self.imp.clone();
        let start_id = if let Some(source) = source.as_deref() {
            Some(
                tracer.bucket_object_download_start(protocol::BucketObjectDownloadStart {
                    source,
                    bucket: imp.bucket_name(),
                    object: imp.key(),
                    version: options.version.as_deref(),
                }),
            )
        } else {
            None
        };

        let fut = self.do_download_all(options);
        async move {
            let res = fut.await;

            if let (Some(start_id), Some(source)) = (start_id, source.as_deref()) {
                tracer.bucket_object_download_end(protocol::BucketObjectDownloadEnd {
                    start_id,
                    source,
                    result: match &res {
                        Ok(bytes) => protocol::BucketObjectDownloadEndResult::Success {
                            size: bytes.len() as u64,
                        },
                        Err(err) => protocol::BucketObjectDownloadEndResult::Err(err),
                    },
                });
            }

            res
        }
    }

    fn do_download_all(
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

    pub async fn attrs(
        &self,
        options: AttrsOptions,
        source: Option<Arc<model::Request>>,
    ) -> Result<ObjectAttrs, Error> {
        if let Some(source) = source.as_deref() {
            let start_id =
                self.tracer
                    .bucket_object_get_attrs_start(protocol::BucketObjectGetAttrsStart {
                        source,
                        bucket: self.imp.bucket_name(),
                        object: self.imp.key(),
                        version: options.version.as_deref(),
                    });
            let res = self.imp.clone().attrs(options).await;

            self.tracer
                .bucket_object_get_attrs_end(protocol::BucketObjectGetAttrsEnd {
                    start_id,
                    source,
                    result: match &res {
                        Ok(attrs) => protocol::BucketObjectGetAttrsEndResult::Success(attrs.into()),
                        Err(err) => protocol::BucketObjectGetAttrsEndResult::Err(err),
                    },
                });
            res
        } else {
            self.imp.clone().attrs(options).await
        }
    }

    pub async fn delete(
        &self,
        options: DeleteOptions,
        source: Option<Arc<model::Request>>,
    ) -> Result<(), Error> {
        if let Some(source) = source.as_deref() {
            let start_id =
                self.tracer
                    .bucket_delete_objects_start(protocol::BucketDeleteObjectsStart {
                        source,
                        bucket: self.imp.bucket_name(),
                        objects: [protocol::BucketDeleteObjectEntry {
                            object: self.imp.key(),
                            version: options.version.as_deref(),
                        }]
                        .into_iter(),
                    });

            let res = self.imp.clone().delete(options).await;

            self.tracer
                .bucket_delete_objects_end(protocol::BucketDeleteObjectsEnd {
                    start_id,
                    source,
                    result: match &res {
                        Ok(()) => protocol::BucketDeleteObjectsEndResult::Success,
                        Err(err) => protocol::BucketDeleteObjectsEndResult::Err(err),
                    },
                });

            res
        } else {
            self.imp.clone().delete(options).await
        }
    }

    /// Returns the public URL of the object, if available.
    /// If the bucket is not public, it reports None.
    pub fn public_url(&self) -> Result<String, PublicUrlError> {
        self.imp.public_url()
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
pub struct ExistsOptions {
    pub version: Option<String>,
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
pub struct UploadUrlOptions {
    pub ttl: u64,
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

pub struct ListIterator {
    stream: Pin<Box<dyn Stream<Item = Result<ListEntry, Error>> + Send>>,
    tracer: Tracer,
    start_id: Option<model::TraceEventId>,
    source: Option<Arc<model::Request>>,
    err: Option<String>,

    yielded_entries: u64,
    seen_end: bool,
}

impl ListIterator {
    pub async fn next(&mut self) -> Option<Result<ListEntry, Error>> {
        let res = self.stream.next().await;

        match &res {
            None => {
                self.seen_end = true;
            }
            Some(Ok(_)) => {
                self.yielded_entries += 1;
            }
            Some(Err(err)) => {
                if self.err.is_none() {
                    self.err = Some(err.to_string());
                }
            }
        }

        res
    }
}

impl Drop for ListIterator {
    fn drop(&mut self) {
        if let (Some(start_id), Some(source)) = (self.start_id, self.source.as_deref()) {
            self.tracer
                .bucket_list_objects_end(protocol::BucketListObjectsEnd {
                    start_id,
                    source,
                    result: match self.err {
                        Some(ref err) => protocol::BucketListObjectsEndResult::Err(err),
                        None => protocol::BucketListObjectsEndResult::Success {
                            observed: self.yielded_entries,
                            has_more: !self.seen_end,
                        },
                    },
                });
        }
    }
}

use percent_encoding::{AsciiSet, CONTROLS};

// From https://url.spec.whatwg.org/#c0-control-percent-encode-set

const QUERY: &AsciiSet = &CONTROLS.add(b' ').add(b'"').add(b'#').add(b'<').add(b'>');
const PATH: &AsciiSet = &QUERY.add(b'?').add(b'`').add(b'{').add(b'}');

fn escape_path(s: &str) -> Cow<'_, str> {
    percent_encoding::percent_encode(s.as_bytes(), PATH).into()
}

/// Computes the public url given a base url and object name.
fn public_url(base_url: String, name: &str) -> String {
    let mut url = base_url;

    if !url.ends_with('/') {
        url.push('/');
    }
    url.push_str(&escape_path(name));
    url
}
