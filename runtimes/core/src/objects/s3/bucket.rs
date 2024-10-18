use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use futures::future;

use crate::encore::runtime::v1 as pb;
use crate::objects;

#[derive(Debug)]
pub struct Bucket {
    client: Arc<s3::Bucket>,
}

impl Bucket {
    pub(super) fn new(region: s3::Region, creds: s3::creds::Credentials, cfg: &pb::Bucket) -> Self {
        let client = s3::Bucket::new(&cfg.cloud_name, region, creds)
            .expect("unable to construct bucket client");
        let client = Arc::from(client);
        Self { client }
    }
}

impl objects::BucketImpl for Bucket {
    fn object(&self, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            client: self.client.clone(),
            name,
        })
    }
}

#[derive(Debug)]
struct Object {
    client: Arc<s3::Bucket>,
    name: String,
}

impl objects::ObjectImpl for Object {
    fn exists(&self) -> Pin<Box<dyn Future<Output = bool> + Send + 'static>> {
        Box::pin(future::ready(true))
    }
}
