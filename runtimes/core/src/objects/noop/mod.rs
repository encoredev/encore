use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::objects;

#[derive(Debug)]
pub struct NoopCluster;

#[derive(Debug)]
pub struct NoopBucket;

impl objects::Cluster for NoopCluster {
    fn bucket(&self, _cfg: &pb::Bucket) -> Arc<dyn objects::Bucket> {
        Arc::new(NoopBucket)
    }
}

impl objects::Bucket for NoopBucket {
}
