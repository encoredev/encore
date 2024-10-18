use std::future::Future;
use std::sync::Arc;
use std::{fmt::Debug, pin::Pin};

pub use manager::{Bucket, Manager, Object};

use crate::encore::runtime::v1 as pb;

mod manager;
mod noop;
mod s3;

trait ClusterImpl: Debug + Send + Sync {
    fn bucket(&self, cfg: &pb::Bucket) -> Arc<dyn BucketImpl + 'static>;
}

trait BucketImpl: Debug + Send + Sync {
    fn object(&self, name: String) -> Arc<dyn ObjectImpl + 'static>;
}

trait ObjectImpl: Debug + Send + Sync {
    fn exists(&self) -> Pin<Box<dyn Future<Output = bool> + Send + 'static>>;
}
