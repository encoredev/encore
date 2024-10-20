use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use futures::future;

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
}

impl objects::ObjectImpl for Object {
    fn exists(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send + 'static>> {
        Box::pin(future::ready(Ok(false)))
    }
}
