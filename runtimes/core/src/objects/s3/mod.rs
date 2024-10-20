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
    pub fn new(cfg: &pb::bucket_cluster::S3) -> Self {
        let region = match cfg.endpoint.as_ref() {
            Some(ep) => s3::Region::Custom {
                region: cfg.region.clone(),
                endpoint: ep.clone(),
            },
            None => {
                let region: s3::Region = cfg.region.parse().expect("unable to resolve S3 region");
                region
            }
        };

        let creds = s3::creds::Credentials::default()
            .or_else(|_| s3::creds::Credentials::anonymous())
            .expect("unable to resolve S3 credentials");

        Self { region, creds }
    }
}

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl + 'static> {
        Arc::new(Bucket::new(self.region.clone(), self.creds.clone(), cfg))
    }
}
