use std::fmt::Debug;
use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::objects;
use crate::objects::gcs::bucket::Bucket;
use anyhow::Context;
use google_cloud_storage as gcs;

mod bucket;

#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyGCSClient>,
}

impl Cluster {
    pub fn new(cfg: pb::bucket_cluster::Gcs) -> Self {
        let client = Arc::new(LazyGCSClient::new(cfg));

        // Begin initializing the client in the background.
        tokio::spawn(client.clone().begin_initialize());

        Self { client }
    }
}

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl + 'static> {
        Arc::new(Bucket::new(self.client.clone(), cfg))
    }
}

struct LazyGCSClient {
    cfg: pb::bucket_cluster::Gcs,
    cell: tokio::sync::OnceCell<anyhow::Result<Arc<gcs::client::Client>>>,
}

impl Debug for LazyGCSClient {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("LazyGCPClient").finish()
    }
}

impl LazyGCSClient {
    fn new(cfg: pb::bucket_cluster::Gcs) -> Self {
        Self {
            cfg,
            cell: tokio::sync::OnceCell::new(),
        }
    }

    async fn get(&self) -> &anyhow::Result<Arc<gcs::client::Client>> {
        self.cell.get_or_init(|| initialize(&self.cfg)).await
    }

    async fn begin_initialize(self: Arc<Self>) {
        self.get().await;
    }
}

async fn initialize(cfg: &pb::bucket_cluster::Gcs) -> anyhow::Result<Arc<gcs::client::Client>> {
    let mut config = gcs::client::ClientConfig::default();
    if let Some(endpoint) = &cfg.endpoint {
        config.storage_endpoint.clone_from(endpoint);
    }

    if cfg.anonymous {
        config = config.anonymous();
    } else {
        config = config
            .with_auth()
            .await
            .context("unable to resolve client config")?;
    }

    Ok(Arc::new(gcs::client::Client::new(config)))
}
