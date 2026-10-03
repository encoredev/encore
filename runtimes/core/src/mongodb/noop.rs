use crate::mongodb::collection::Collection;
use std::sync::Arc;

use crate::mongodb::manager::{Connection, Database};
use crate::names::EncoreName;

const NOT_CONFIGURED: &str = "mongodb: this service is not configured to use this database. Use MongoDatabase.named in this service to get a reference and access to the database from this service";

/// NoopDatabase is returned when a MongoDB database is not configured.
/// All operations on it will return an error immediately.
pub struct NoopDatabase {
    name: EncoreName,
}

impl NoopDatabase {
    pub fn new(name: EncoreName) -> Self {
        Self { name }
    }
}

impl Database for NoopDatabase {
    fn name(&self) -> &EncoreName {
        &self.name
    }

    fn collection(&self, _name: &str) -> anyhow::Result<Collection> {
        anyhow::bail!(NOT_CONFIGURED)
    }

    fn connection(&self) -> anyhow::Result<Arc<Connection>> {
        anyhow::bail!(NOT_CONFIGURED)
    }
}
