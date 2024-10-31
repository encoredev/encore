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
use crate::objects::{
    AttrsOptions, DeleteOptions, DownloadOptions, DownloadStream, Error, ListEntry, ListOptions,
    ObjectAttrs, UploadOptions,
};
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
        options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, objects::Error>> + Send + 'static>>
    {
        Box::pin(async move {
            match self.client.get().await {
                Ok(client) => {
                    let client = client.clone();

                    let mut total_seen = 0;
                    const DEFAULT_MAX_RESULTS: u64 = 1000;
                    let s: objects::ListStream = Box::new(try_stream! {
                        let max_results = if let Some(limit) = options.limit {
                            limit.min(DEFAULT_MAX_RESULTS) as i32
                        } else {
                            DEFAULT_MAX_RESULTS as i32
                        };

                        let mut req = gcs::http::objects::list::ListObjectsRequest {
                            bucket: self.name.to_string(),
                            max_results: Some(max_results),
                            ..Default::default()
                        };


                        // Filter by key prefix, if provided.
                        if let Some(key_prefix) = &self.key_prefix {
                            req.prefix = Some(key_prefix.clone());
                        }

                        'PageLoop:
                        loop {
                            let resp = client.list_objects(&req).await.map_err(|e| Error::Other(e.into()))?;
                            if let Some(items) = resp.items {
                                for obj in items {
                                    total_seen += 1;
                                    if let Some(limit) = options.limit {
                                        if total_seen > limit {
                                            break 'PageLoop;
                                        }
                                    }

                                    let entry = ListEntry {
                                        name: self.strip_prefix(Cow::Owned(obj.name)).into_owned(),
                                        size: obj.size as u64,
                                        etag: obj.etag,
                                    };
                                    yield entry;
                                }
                            }

                            req.page_token = resp.next_page_token;

                            // Are we close to being done? If so, adjust the max_results
                            // to avoid over-fetching.
                            if let Some(limit) = options.limit {
                                let remaining = (limit - total_seen).max(0);
                                if remaining == 0 {
                                    break 'PageLoop;
                                }
                                req.max_results = Some(remaining.min(DEFAULT_MAX_RESULTS) as i32);
                            }

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
    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = options.version {
                        req.generation = Some(parse_version(version)?);
                    }

                    let obj = client.get_object(&req).await.map_err(map_err)?;
                    Ok(ObjectAttrs {
                        name: obj.name,
                        version: Some(obj.generation.to_string()),
                        size: obj.size as u64,
                        content_type: obj.content_type,
                        etag: obj.etag,
                    })
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn exists(
        self: Arc<Self>,
        version: Option<String>,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = version {
                        req.generation = Some(parse_version(version)?);
                    }

                    match client.get_object(&req).await.map_err(map_err) {
                        Ok(_obj) => Ok(true),
                        Err(Error::NotFound) => Ok(false),
                        Err(err) => Err(err),
                    }
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn upload(
        self: Arc<Self>,
        data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        opts: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
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
                            name: obj.name,
                            version: Some(obj.generation.to_string()),
                            size: obj.size as u64,
                            content_type: obj.content_type,
                            etag: obj.etag,
                        }),
                        Err(err) => Err(map_err(err)),
                    }
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn download(
        self: Arc<Self>,
        options: DownloadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<DownloadStream, Error>> + Send>> {
        fn convert_err(err: gcs::http::Error) -> Error {
            use gcs::http::error::ErrorResponse;
            match err {
                gcs::http::Error::Response(ErrorResponse { code: 404, .. }) => Error::NotFound,
                gcs::http::Error::HttpClient(err)
                    if err.status().map(|s| s.as_u16()) == Some(404) =>
                {
                    Error::NotFound
                }
                err => Error::Other(err.into()),
            }
        }

        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = GetObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = options.version {
                        req.generation = Some(parse_version(version)?);
                    }

                    let resp = client
                        .download_streamed_object(&req, &Range::default())
                        .await;

                    let stream = resp.map_err(convert_err)?;
                    let stream: DownloadStream = Box::pin(stream.map_err(convert_err));
                    Ok(stream)
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn delete(
        self: Arc<Self>,
        options: DeleteOptions,
    ) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = gcs::http::objects::delete::DeleteObjectRequest {
                        bucket: self.bkt.name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.name)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = options.version {
                        req.generation = Some(parse_version(version)?);
                    }

                    match client.delete_object(&req).await.map_err(map_err) {
                        Ok(_) => Ok(()),
                        Err(Error::NotFound) => Ok(()),
                        Err(err) => Err(err),
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

fn parse_version(version: String) -> Result<i64, Error> {
    version.parse().map_err(|err| {
        Error::Other(anyhow::anyhow!(
            "invalid version number {}: {}",
            version,
            err
        ))
    })
}

fn map_err(err: gcs::http::Error) -> Error {
    use gcs::http::error::ErrorResponse;
    match err {
        gcs::http::Error::Response(ErrorResponse { code, .. }) => match code {
            404 => Error::NotFound,
            412 => Error::PreconditionFailed,
            _ => Error::Other(err.into()),
        },
        gcs::http::Error::HttpClient(err) => {
            let status = err.status().map(|s| s.as_u16());
            match status {
                Some(404) => Error::NotFound,
                Some(412) => Error::PreconditionFailed,
                _ => Error::Other(err.into()),
            }
        }
        err => Error::Other(err.into()),
    }
}
