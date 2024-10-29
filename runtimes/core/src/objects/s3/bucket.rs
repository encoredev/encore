use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::AsyncRead;

use crate::encore::runtime::v1 as pb;
use crate::objects::{self, Error, ObjectAttrs};

#[derive(Debug)]
pub struct Bucket {
    client: Arc<s3::Bucket>,
    key_prefix: Option<String>,
}

impl Bucket {
    pub(super) fn new(region: s3::Region, creds: s3::creds::Credentials, cfg: &pb::Bucket) -> Self {
        let client = s3::Bucket::new(&cfg.cloud_name, region, creds)
            .expect("unable to construct bucket client")
            .with_path_style();
        let client = Arc::from(client);
        Self {
            client,
            key_prefix: cfg.key_prefix.clone(),
        }
    }

    /// Computes the object name, including the key prefix if present.
    fn obj_name<'a>(&'_ self, name: Cow<'a, str>) -> Cow<'a, str> {
        match &self.key_prefix {
            Some(prefix) => {
                let mut key = prefix.to_owned();
                key.push_str(&name);
                Cow::Owned(key)
            }
            None => name,
        }
    }

    /// Returns the name with the key prefix stripped, if present.
    fn _strip_prefix<'a>(&'_ self, name: Cow<'a, str>) -> Cow<'a, str> {
        match &self.key_prefix {
            Some(prefix) => name
                .as_ref()
                .strip_prefix(prefix)
                .map(|s| Cow::Owned(s.to_string()))
                .unwrap_or(name),
            None => name,
        }
    }
}

impl objects::BucketImpl for Bucket {
    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            bkt: self.clone(),
            name,
        })
    }

    fn list(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, objects::Error>> + Send + 'static>>
    {
        Box::pin(async move {
            Err(objects::Error::Internal(anyhow::anyhow!(
                "not yet implemented"
            )))
        })
    }
}

#[derive(Debug)]
struct Object {
    bkt: Arc<Bucket>,
    name: String,
}

impl objects::ObjectImpl for Object {
    fn attrs(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = self.bkt.client.head_object(&cloud_name).await;
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
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = self.bkt.client.head_object(&cloud_name).await;
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

    fn delete(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>> {
        Box::pin(async move {
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = self.bkt.client.delete_object(&cloud_name).await;
            match res {
                Ok(_) => Ok(()),
                Err(s3::error::S3Error::HttpFailWithBody(404, _)) => Ok(()),
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }
}
