use anyhow::Result;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use crate::encore::runtime::v1 as pb;
use crate::{objects, CloudName};
use google_cloud_storage as gcs;

use super::LazyGCSClient;

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyGCSClient>,
    name: CloudName,
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyGCSClient>, cfg: &pb::Bucket) -> Self {
        Self {
            client,
            name: cfg.cloud_name.clone().into(),
        }
    }
}

impl objects::BucketImpl for Bucket {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object { bkt: self, name })
    }
}

#[derive(Debug)]
struct Object {
    bkt: Arc<Bucket>,
    name: String,
}

impl objects::ObjectImpl for Object {
    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<bool>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    use gcs::http::{error::ErrorResponse, Error};
                    let req = &gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.name.clone(),
                        ..Default::default()
                    };

                    match client.get_object(req).await {
                        Ok(_obj) => Ok(true),
                        Err(Error::Response(ErrorResponse { code: 404, .. })) => Ok(false),
                        Err(Error::HttpClient(err))
                            if err.status().is_some_and(|v| v.as_u16() == 404) =>
                        {
                            Ok(false)
                        }

                        Err(err) => Err(err.into()),
                    }
                }
                Err(err) => Err(anyhow::anyhow!("unable to resolve client: {}", err)),
            }
        })
    }
}
