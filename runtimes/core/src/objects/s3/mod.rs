use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::objects;
use crate::objects::s3::bucket::Bucket;

mod bucket;

#[derive(Debug)]
pub struct Cluster {
    region: s3::Region,
    creds: s3::creds::Credentials,
}

impl Cluster {
    pub fn new(cfg: &pb::BucketCluster) -> Self {
        let region = s3::Region::Custom {
            region: cfg.region.clone(),
            endpoint: cfg.endpoint.clone(),
        };

        // TODO(andre): does this work?
        let creds = s3::creds::Credentials::default().unwrap();

        Self { region, creds }
    }
}

impl objects::Cluster for Cluster {
    fn bucket(&self, cfg: &pb::Bucket) -> Arc<dyn objects::Bucket + 'static> {
        Arc::new(Bucket::new(self.region.clone(), self.creds.clone(), cfg))
    }
}
