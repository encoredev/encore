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
    fn attrs(self: Arc<Self>) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    use gcs::http::{error::ErrorResponse, Error as GCSError};
                    let req = &gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.name.clone(),
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
                    let mut media = Media::new(self.name.clone());

                    apply_upload_opts(opts, &mut req, &mut media);

                    let upload_type = UploadType::Simple(media);
                    let stream = tokio_util::io::ReaderStream::new(data);

                    match client
                        .upload_streamed_object(&req, stream, &upload_type)
                        .await
                    {
                        Ok(obj) => Ok(ObjectAttrs {
                            name: obj.name,
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
                        object: self.name.clone(),
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
