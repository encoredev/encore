use crate::cache::manager::Cluster;
use crate::cache::pool::Pool;
use crate::names::EncoreName;

/// NoopCluster is returned when a cache cluster is not configured.
/// All operations on it will return an error immediately.
pub struct NoopCluster {
    name: EncoreName,
}

impl NoopCluster {
    pub fn new(name: EncoreName) -> Self {
        Self { name }
    }
}

impl Cluster for NoopCluster {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn pool(&self) -> anyhow::Result<Pool> {
        anyhow::bail!("cache: this service is not configured to use this cache cluster")
    }
}
