use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use futures::future;
use tokio::io::AsyncRead;

use crate::encore::runtime::v1 as pb;
use crate::objects;

#[derive(Debug)]
pub struct Cluster;

#[derive(Debug)]
pub struct Bucket;

#[derive(Debug)]
pub struct Object;

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, _cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl> {
        Arc::new(Bucket)
    }
}

impl objects::BucketImpl for Bucket {
    fn object(self: Arc<Self>, _name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object)
    }

    fn list(
        self: Arc<Self>,
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
    fn attrs(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ObjectAttrs, objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support attrs"),
        ))))
    }

    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send>> {
        Box::pin(future::ready(Err(anyhow::anyhow!(
            "noop bucket does not support exists"
        ))))
    }

    fn upload(
        self: Arc<Self>,
        _data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        _options: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send>> {
        Box::pin(future::ready(Err(anyhow::anyhow!(
            "noop bucket does not support upload"
        ))))
    }

    fn download(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<objects::DownloadStream, objects::DownloadError>> + Send>>
    {
        Box::pin(async move {
            Err(objects::DownloadError::Internal(anyhow::anyhow!(
                "noop bucket does not support download"
            )))
        })
    }

    fn delete(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<(), objects::Error>> + Send>> {
        Box::pin(future::ready(Err(objects::Error::Internal(
            anyhow::anyhow!("noop bucket does not support delete"),
        ))))
    }
}
