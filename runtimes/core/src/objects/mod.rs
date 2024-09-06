use std::fmt::Debug;
use std::sync::Arc;

pub use manager::{Manager};

use crate::encore::runtime::v1 as pb;

mod s3;
mod noop;
mod manager;

trait Cluster: Debug + Send + Sync {
    fn bucket(&self, cfg: &pb::Bucket) -> Arc<dyn Bucket + 'static>;
}

trait Bucket: Debug + Send + Sync {}
