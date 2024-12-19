use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use futures::future;
use tokio::io::AsyncRead;

use crate::objects;
use crate::{encore::runtime::v1 as pb, EncoreName};

use super::{
    AttrsOptions, DeleteOptions, DownloadOptions, ExistsOptions, ListOptions, PublicUrlError,
    UploadUrlOptions,
};

#[derive(Debug)]
pub struct Cluster;

#[derive(Debug)]
pub struct Bucket {
    name: EncoreName,
}

impl Bucket {
    pub fn new(name: EncoreName) -> Self {
        Self { name }
    }
}

#[derive(Debug)]
pub struct Object {
    bkt: Arc<Bucket>,
    name: String,
}

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl> {
        Arc::new(Bucket {
            name: cfg.encore_name.clone().into(),
        })
    }
}

impl objects::BucketImpl for Bucket {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            name,
            bkt: self.clone(),
        })
    }

    fn list(
        self: Arc<Self>,
        _options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, objects::Error>> + Send + 'static>>
    {
        Box::pin(async move {
            Err(objects::Error::Internal(anyhow::anyhow!(
                "noop bucket does not support list"
            )))
        })
    }
}

impl objects::ObjectImpl for Object {
    fn bucket_name(&self) -> &EncoreName {
        &self.bkt.name
    }

    fn key(&self) -> &str {
        &self.name
    }

    fn attrs(
        self: Arc<Self>,
        _options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ObjectAttrs, objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support attrs"),
        ))))
    }

    fn signed_upload_url(
        self: Arc<Self>,
        _options: UploadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support getting upload URL"),
        ))))
    }

    fn exists(
        self: Arc<Self>,
        _options: ExistsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<bool, objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support exists"),
        ))))
    }

    fn upload(
        self: Arc<Self>,
        _data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        _options: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ObjectAttrs, objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Other(anyhow::anyhow!(
            "noop bucket does not support upload"
        )))))
    }

    fn download(
        self: Arc<Self>,
        _options: DownloadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::DownloadStream, objects::Error>> + Send>> {
        Box::pin(async move {
            Err(objects::Error::Internal(anyhow::anyhow!(
                "noop bucket does not support download"
            )))
        })
    }

    fn delete(
        self: Arc<Self>,
        _options: DeleteOptions,
    ) -> Pin<Box<dyn Future<Output = Result<(), objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support delete"),
        ))))
    }

    fn public_url(&self) -> Result<String, PublicUrlError> {
        Err(PublicUrlError::NoopBucket)
    }
}
