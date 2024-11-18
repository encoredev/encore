use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::objects::{gcs, noop, s3, BucketImpl, ClusterImpl};
use crate::trace::Tracer;

use super::Bucket;

pub struct Manager {
    tracer: Tracer,
    bucket_cfg: HashMap<EncoreName, (Arc<dyn ClusterImpl>, pb::Bucket)>,

    buckets: Arc<RwLock<HashMap<EncoreName, Arc<dyn BucketImpl>>>>,
}

impl Manager {
    pub fn new(tracer: Tracer, clusters: Vec<pb::BucketCluster>, md: &meta::Data) -> Self {
        let bucket_cfg = make_cfg_maps(clusters, md);

        Self {
            tracer,
            bucket_cfg,
            buckets: Arc::default(),
        }
    }

    pub fn bucket(&self, name: EncoreName) -> Option<Bucket> {
        let imp = self.bucket_impl(name)?;
        Some(Bucket {
            imp,
            tracer: self.tracer.clone(),
        })
    }

    fn bucket_impl(&self, name: EncoreName) -> Option<Arc<dyn BucketImpl>> {
        if let Some(bkt) = self.buckets.read().unwrap().get(&name) {
            return Some(bkt.clone());
        }

        let bkt = {
            if let Some((cluster, bucket_cfg)) = self.bucket_cfg.get(&name) {
                cluster.clone().bucket(bucket_cfg)
            } else {
                Arc::new(noop::Bucket::new(name.clone()))
            }
        };

        self.buckets.write().unwrap().insert(name, bkt.clone());
        Some(bkt)
    }
}

fn make_cfg_maps(
    clusters: Vec<pb::BucketCluster>,
    _md: &meta::Data,
) -> HashMap<EncoreName, (Arc<dyn ClusterImpl>, pb::Bucket)> {
    let mut bucket_map = HashMap::new();

    for cluster_cfg in clusters {
        let cluster = new_cluster(&cluster_cfg);

        for bucket_cfg in cluster_cfg.buckets {
            bucket_map.insert(
                bucket_cfg.encore_name.clone().into(),
                (cluster.clone(), bucket_cfg),
            );
        }
    }

    bucket_map
}

fn new_cluster(cluster: &pb::BucketCluster) -> Arc<dyn ClusterImpl> {
    let Some(provider) = &cluster.provider else {
        log::error!("missing bucket cluster provider: {}", cluster.rid);
        return Arc::new(noop::Cluster);
    };

    match provider {
        pb::bucket_cluster::Provider::S3(s3cfg) => Arc::new(s3::Cluster::new(s3cfg)),
        pb::bucket_cluster::Provider::Gcs(gcscfg) => Arc::new(gcs::Cluster::new(gcscfg.clone())),
    }
}
