use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::objects;
use crate::objects::s3::bucket::Bucket;
use crate::secrets::Secret;
use aws_sdk_s3 as s3;

mod bucket;

#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyS3Client>,
}

impl Cluster {
    pub fn new(cfg: pb::bucket_cluster::S3, secret_access_key: Option<Secret>) -> Self {
        let client = Arc::new(LazyS3Client::new(cfg, secret_access_key));
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
    secret_access_key: Option<Secret>,
    cell: tokio::sync::OnceCell<Arc<s3::Client>>,
}

impl std::fmt::Debug for LazyS3Client {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("LazyS3Client").finish()
    }
}

impl LazyS3Client {
    fn new(cfg: pb::bucket_cluster::S3, secret_access_key: Option<Secret>) -> Self {
        Self {
            cfg,
            secret_access_key,
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

                if let (Some(access_key_id), Some(secret_access_key)) = (
                    self.cfg.access_key_id.as_ref(),
                    self.secret_access_key.as_ref(),
                ) {
                    use aws_credential_types::Credentials;
                    let secret_access_key = secret_access_key
                        .get()
                        .expect("unable to resolve s3 secret access key");
                    let secret_access_key = std::str::from_utf8(secret_access_key)
                        .expect("unable to parse s3 secret access key as utf-8");

                    builder = builder.credentials_provider(Credentials::new(
                        access_key_id,
                        secret_access_key,
                        None,
                        None,
                        "encore-runtime",
                    ));
                }

                let cfg = builder.load().await;
                Arc::new(s3::Client::new(&cfg))
            })
            .await
    }
}
