use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::objects;
use crate::objects::s3::bucket::Bucket;
use aws_sdk_s3 as s3;

mod bucket;

#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyS3Client>,
}

impl Cluster {
    pub fn new(cfg: &pb::bucket_cluster::S3) -> Self {
        let client = Arc::new(LazyS3Client::new(cfg.clone()));
        Self { client }
    }
}

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl + 'static> {
        Arc::new(Bucket::new(self.client.clone(), cfg))
    }
}

struct LazyS3Client {
    cfg: pb::bucket_cluster::S3,
    cell: tokio::sync::OnceCell<Arc<s3::Client>>,
}

impl std::fmt::Debug for LazyS3Client {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("LazyS3Client").finish()
    }
}

impl LazyS3Client {
    fn new(cfg: pb::bucket_cluster::S3) -> Self {
        Self {
            cfg,
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get(&self) -> &Arc<s3::Client> {
        self.cell
            .get_or_init(|| async {
                let region = aws_config::Region::new(self.cfg.region.clone());
                let mut builder =
                    aws_config::defaults(aws_config::BehaviorVersion::v2024_03_28()).region(region);
                if let Some(endpoint) = self.cfg.endpoint.as_ref() {
                    builder = builder.endpoint_url(endpoint.clone());
                }

                let cfg = builder.load().await;
                Arc::new(s3::Client::new(&cfg))
            })
            .await
    }
}
