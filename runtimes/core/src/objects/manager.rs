use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::names::EncoreName;
use crate::objects::noop::NoopCluster;
use crate::objects::{
    noop, Cluster, Bucket,
};
use crate::trace::{Tracer};

pub struct Manager {
    tracer: Tracer,
    bucket_cfg: HashMap<EncoreName, (Arc<dyn Cluster>, pb::Bucket)>,

    buckets: Arc<RwLock<HashMap<EncoreName, Arc<dyn Bucket>>>>,
}

#[derive(Debug)]
pub struct BucketObj {
    name: EncoreName,
    tracer: Tracer,
    inner: Arc<dyn Bucket>,
}

impl BucketObj {
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

    pub fn bucket(&self, name: EncoreName) -> Option<BucketObj> {
        let inner = self.bucket_impl(name.clone())?;
        Some(BucketObj {
            name,
            inner,
            tracer: self.tracer.clone(),
        })
    }

    fn bucket_impl(&self, name: EncoreName) -> Option<Arc<dyn Bucket>> {
        if let Some(bkt) = self.buckets.read().unwrap().get(&name) {
            return Some(bkt.clone());
        }

        let bkt = {
            if let Some((cluster, bucket_cfg)) = self.bucket_cfg.get(&name) {
                cluster.bucket(bucket_cfg)
            } else {
                Arc::new(noop::NoopBucket)
            }
        };

        self.buckets.write().unwrap().insert(name, bkt.clone());
        Some(bkt)
    }
}

fn make_cfg_maps(
    clusters: Vec<pb::BucketCluster>,
    md: &meta::Data,
) ->
    HashMap<EncoreName, (Arc<dyn Cluster>, pb::Bucket)>
{
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

fn new_cluster(cluster: &pb::BucketCluster) -> Arc<dyn Cluster> {
    // let Some(provider) = &cluster.provider else {
    //     log::error!("missing PubSub cluster provider: {}", cluster.rid);
    //     return Arc::new(NoopCluster);
    // };

    // match provider {
    //     pb::pub_sub_cluster::Provider::Gcp(_) => return Arc::new(gcp::Cluster::new()),
    //     pb::pub_sub_cluster::Provider::Nsq(cfg) => {
    //         return Arc::new(nsq::Cluster::new(cfg.hosts[0].clone()));
    //     }
    //     pb::pub_sub_cluster::Provider::Aws(_) => return Arc::new(sqs_sns::Cluster::new()),
    //     pb::pub_sub_cluster::Provider::Encore(_) => {
    //         log::error!("Encore Cloud Pub/Sub not yet supported: {}", cluster.rid);
    //     }
    //     pb::pub_sub_cluster::Provider::Azure(_) => {
    //         log::error!("Azure Pub/Sub not yet supported: {}", cluster.rid);
    //     }
    // }

    Arc::new(NoopCluster)
}
