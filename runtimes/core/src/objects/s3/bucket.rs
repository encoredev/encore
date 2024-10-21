use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::AsyncRead;

use crate::encore::runtime::v1 as pb;
use crate::objects::{self, Error, ObjectAttrs};

#[derive(Debug)]
pub struct Bucket {
    client: Arc<s3::Bucket>,
}

impl Bucket {
    pub(super) fn new(region: s3::Region, creds: s3::creds::Credentials, cfg: &pb::Bucket) -> Self {
        let client = s3::Bucket::new(&cfg.cloud_name, region, creds)
            .expect("unable to construct bucket client")
            .with_path_style();
        let client = Arc::from(client);
        Self { client }
    }
}

impl objects::BucketImpl for Bucket {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
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
    fn attrs(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            let res = self.client.head_object(&self.name).await;
            match res {
                Ok((obj, _)) => Ok(ObjectAttrs {
                    name: self.name.clone(),
                    version: obj.version_id.unwrap_or_default(),
                    size: obj.content_length.unwrap_or_default() as u64,
                    content_type: obj.content_type,
                    etag: obj.e_tag.unwrap_or_default(),
                }),
                Err(s3::error::S3Error::HttpFailWithBody(404, _)) => Err(Error::NotFound),
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send>> {
        Box::pin(async move {
            let res = self.client.head_object(&self.name).await;
            match res {
                Ok(_) => Ok(true),
                Err(s3::error::S3Error::HttpFailWithBody(404, _)) => Ok(false),
                Err(err) => Err(err.into()),
            }
        })
    }

    fn upload(
        self: Arc<Self>,
        _data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        _options: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<objects::ObjectAttrs>> + Send>> {
        Box::pin(async move { Err(anyhow::anyhow!("not yet implemented")) })
    }

    fn download(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<objects::DownloadStream, objects::DownloadError>> + Send>>
    {
        Box::pin(async move {
            Err(objects::DownloadError::Internal(anyhow::anyhow!(
                "not yet implemented"
            )))
        })
    }
}
