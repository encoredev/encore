use async_stream::try_stream;
use futures::TryStreamExt;
use google_cloud_storage::http::objects::download::Range;
use google_cloud_storage::http::objects::get::GetObjectRequest;
use google_cloud_storage::http::objects::upload::{Media, UploadObjectRequest, UploadType};
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::AsyncRead;

use crate::encore::runtime::v1 as pb;
use crate::objects::{DownloadError, DownloadStream, Error, ObjectAttrs, UploadOptions};
use crate::{objects, CloudName};
use google_cloud_storage as gcs;

use super::LazyGCSClient;

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyGCSClient>,
    name: CloudName,
    key_prefix: Option<String>,
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyGCSClient>, cfg: &pb::Bucket) -> Self {
        Self {
            client,
            name: cfg.cloud_name.clone().into(),
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
    fn strip_prefix<'a>(&'_ self, name: Cow<'a, str>) -> Cow<'a, str> {
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
        Arc::new(Object { bkt: self, name })
    }

    fn list(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, objects::Error>> + Send + 'static>>
    {
        Box::pin(async move {
            match self.client.get().await {
                Ok(client) => {
                    let client = client.clone();
                    let s: objects::ListStream = Box::new(try_stream! {
                        let mut req = gcs::http::objects::list::ListObjectsRequest {
                            bucket: self.name.to_string(),
                            ..Default::default()
                        };

                        // Filter by key prefix, if provided.
                        if let Some(key_prefix) = &self.key_prefix {
                            req.prefix = Some(key_prefix.clone());
                        }

                        loop {
                            let resp = client.list_objects(&req).await.map_err(|e| Error::Other(e.into()))?;
                            if let Some(items) = resp.items {
                                for obj in items {
                                    let attrs = ObjectAttrs {
                                        name: self.strip_prefix(Cow::Owned(obj.name)).into_owned(),
                                        version: obj.generation.to_string(),
                                        size: obj.size as u64,
                                        content_type: obj.content_type,
                                        etag: obj.etag,
                                    };
                                    yield attrs;
                                }
                            }

                            req.page_token = resp.next_page_token;
                            if req.page_token.is_none() {
                                break;
                            }
                        }
                    });

                    Ok(s)
                }

                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
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
            match self.bkt.client.get().await {
                Ok(client) => {
                    use gcs::http::{error::ErrorResponse, Error as GCSError};
                    let req = &gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    match client.get_object(req).await {
                        Ok(obj) => Ok(ObjectAttrs {
                            name: obj.name,
                            version: obj.generation.to_string(),
                            size: obj.size as u64,
                            content_type: obj.content_type,
                            etag: obj.etag,
                        }),
                        Err(GCSError::Response(ErrorResponse { code: 404, .. })) => {
                            Err(Error::NotFound)
                        }
                        Err(GCSError::HttpClient(err))
                            if err.status().is_some_and(|v| v.as_u16() == 404) =>
                        {
                            Err(Error::NotFound)
                        }

                        Err(err) => Err(Error::Other(err.into())),
                    }
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn exists(self: Arc<Self>) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    use gcs::http::{error::ErrorResponse, Error};
                    let req = &gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
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

    fn upload(
        self: Arc<Self>,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        opts: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<ObjectAttrs>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = UploadObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        ..Default::default()
                    };

                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let mut media = Media::new(cloud_name.into_owned());

                    apply_upload_opts(opts, &mut req, &mut media);

                    let upload_type = UploadType::Simple(media);
                    let stream = tokio_util::io::ReaderStream::new(data);

                    match client
                        .upload_streamed_object(&req, stream, &upload_type)
                        .await
                    {
                        Ok(obj) => Ok(ObjectAttrs {
                            name: self.name.clone(),
                            version: obj.generation.to_string(),
                            size: obj.size as u64,
                            content_type: obj.content_type,
                            etag: obj.etag,
                        }),
                        Err(err) => Err(err.into()),
                    }
                }
                Err(err) => Err(anyhow::anyhow!("unable to resolve client: {}", err)),
            }
        })
    }

    fn download(
        self: Arc<Self>,
    ) -> Pin<Box<dyn Future<Output = Result<DownloadStream, DownloadError>> + Send>> {
        fn convert_err(err: gcs::http::Error) -> DownloadError {
            use gcs::http::error::ErrorResponse;
            use gcs::http::Error;
            match err {
                Error::Response(ErrorResponse { code: 404, .. }) => DownloadError::NotFound,
                Error::HttpClient(err) if err.status().map(|s| s.as_u16()) == Some(404) => {
                    DownloadError::NotFound
                }
                err => DownloadError::Other(err.into()),
            }
        }

        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let req = GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };
                    let resp = client
                        .download_streamed_object(&req, &Range::default())
                        .await;

                    let stream = resp.map_err(convert_err)?;
                    let stream: DownloadStream = Box::new(stream.map_err(convert_err));
                    Ok(stream)
                }
                Err(err) => Err(DownloadError::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn delete(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    use gcs::http::{error::ErrorResponse, Error as GCSError};
                    let req = &gcs::http::objects::delete::DeleteObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    match client.delete_object(req).await {
                        Ok(_) => Ok(()),
                        Err(GCSError::Response(ErrorResponse { code: 404, .. })) => Ok(()),
                        Err(GCSError::HttpClient(err))
                            if err.status().is_some_and(|v| v.as_u16() == 404) =>
                        {
                            Ok(())
                        }

                        Err(err) => Err(Error::Other(err.into())),
                    }
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }
}

fn apply_upload_opts(opts: UploadOptions, req: &mut UploadObjectRequest, media: &mut Media) {
    if let Some(content_type) = opts.content_type {
        media.content_type = Cow::Owned(content_type);
    }
    if let Some(pre) = opts.preconditions {
        if pre.not_exists == Some(true) {
            req.if_generation_match = Some(0);
        }
    }
}
