use anyhow::Result;
use std::future::Future;
use std::sync::Arc;
use std::{fmt::Debug, pin::Pin};

pub use manager::{Bucket, Manager, Object};

use crate::encore::runtime::v1 as pb;

mod gcs;
mod manager;
mod noop;
mod s3;

trait ClusterImpl: Debug + Send + Sync {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn BucketImpl + 'static>;
}

trait BucketImpl: Debug + Send + Sync {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn ObjectImpl + 'static>;
}

trait ObjectImpl: Debug + Send + Sync {
    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<bool>> + Send>>;
}
