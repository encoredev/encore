use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use futures::future;

use crate::encore::runtime::v1 as pb;
use crate::objects;

#[derive(Debug)]
pub struct NoopCluster;

#[derive(Debug)]
pub struct NoopBucket;

#[derive(Debug)]
pub struct NoopObject;

impl objects::ClusterImpl for NoopCluster {
    fn bucket(&self, _cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl> {
        Arc::new(NoopBucket)
    }
}

impl objects::BucketImpl for NoopBucket {
    fn object(&self, _name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(NoopObject)
    }
}

impl objects::ObjectImpl for NoopObject {
    fn exists(&self) -> Pin<Box<dyn Future<Output = bool> + Send + 'static>> {
        Box::pin(future::ready(false))
    }
}
