use async_stream::try_stream;
use futures::TryStreamExt;
use google_cloud_storage::http::objects::download::Range;
use google_cloud_storage::http::objects::get::GetObjectRequest;
use google_cloud_storage::http::objects::upload::{Media, UploadObjectRequest, UploadType};
use google_cloud_storage::sign::SignBy;
use google_cloud_storage::sign::SignedURLOptions;
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::time::{Duration, SystemTime};
use tokio::io::AsyncRead;
use url::Url;

use crate::encore::runtime::v1 as pb;
use crate::objects::{
    AttrsOptions, DeleteOptions, DownloadOptions, DownloadStream, Error, ExistsOptions, ListEntry,
    ListOptions, ObjectAttrs, PublicUrlError, UploadOptions, UploadUrlOptions,
};
use crate::{objects, CloudName, EncoreName};
use google_cloud_storage as gcs;

use super::LazyGCSClient;

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyGCSClient>,
    encore_name: EncoreName,
    cloud_name: CloudName,
    public_base_url: Option<String>,
    key_prefix: Option<String>,
    local_sign: Option<LocalSignOptions>,
}

#[derive(Debug)]
pub struct LocalSignOptions {
    base_url: String,
    access_id: String,
    private_key: String,
}

fn local_sign_config_from_client(client: Arc<LazyGCSClient>) -> Option<LocalSignOptions> {
    client.cfg.local_sign.as_ref().map(|cfg| LocalSignOptions {
        base_url: cfg.base_url.clone(),
        access_id: cfg.access_id.clone(),
        private_key: cfg.private_key.clone(),
    })
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyGCSClient>, cfg: &pb::Bucket) -> Self {
        Self {
            client: client.clone(),
            encore_name: cfg.encore_name.clone().into(),
            cloud_name: cfg.cloud_name.clone().into(),
            public_base_url: cfg.public_base_url.clone(),
            key_prefix: cfg.key_prefix.clone(),
            local_sign: local_sign_config_from_client(client.clone()),
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
    fn name(&self) -> &EncoreName {
        &self.encore_name
    }

    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            bkt: self,
            key: name,
        })
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
                            bucket: self.cloud_name.to_string(),
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
                                if total_seen >= limit {
                                    break 'PageLoop;
                                }
                                let remaining = limit - total_seen;
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
    key: String,
}

impl objects::ObjectImpl for Object {
    fn bucket_name(&self) -> &EncoreName {
        &self.bkt.encore_name
    }

    fn key(&self) -> &str {
        &self.key
    }

    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.cloud_name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.key)).into_owned(),
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

    fn signed_upload_url(
        self: Arc<Self>,
        options: UploadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let gcs_opts = SignedURLOptions {
                        method: gcs::sign::SignedURLMethod::PUT,
                        expires: Duration::from_secs(options.ttl),
                        start_time: Some(SystemTime::now()),
                        ..Default::default()
                    };

                    // We use a fake GCS service for local development. Ideally, the runtime
                    // code would be oblivious to this once the GCS client is set up. But that
                    // turns out to be difficult for URL signing, so we add a special case
                    // here.
                    let local_sign = &self.bkt.local_sign;
                    let access_id: Option<String>;
                    let sign_by: Option<SignBy>;
                    if let Some(opt) = local_sign {
                        access_id = Some(opt.access_id.clone());
                        sign_by = Some(SignBy::PrivateKey(opt.private_key.as_bytes().to_vec()));
                    } else {
                        access_id = None;
                        sign_by = None;
                    }

                    let name = self.bkt.obj_name(Cow::Borrowed(&self.key)).into_owned();
                    let mut url = client
                        .signed_url(&self.bkt.cloud_name, &name, access_id, sign_by, gcs_opts)
                        .await
                        .map_err(map_sign_err)?;

                    // More special handling for the local dev case.
                    if let Some(cfg) = local_sign {
                        url = replace_url_prefix(url, cfg.base_url.clone());
                    }

                    Ok(url)
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
        options: ExistsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(client) => {
                    let mut req = gcs::http::objects::get::GetObjectRequest {
                        bucket: self.bkt.cloud_name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.key)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = options.version {
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
                        bucket: self.bkt.cloud_name.to_string(),
                        ..Default::default()
                    };

                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.key));
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
                        bucket: self.bkt.cloud_name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.key)).into_owned(),
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
                        bucket: self.bkt.cloud_name.to_string(),
                        object: self.bkt.obj_name(Cow::Borrowed(&self.key)).into_owned(),
                        ..Default::default()
                    };

                    if let Some(version) = options.version {
                        req.generation = Some(parse_version(version)?);
                    }

                    match client.delete_object(&req).await.map_err(map_err) {
                        Ok(_) => Ok(()),
                        Err(Error::NotFound) => Err(Error::NotFound),
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

    fn public_url(&self) -> Result<String, PublicUrlError> {
        let Some(base_url) = self.bkt.public_base_url.clone() else {
            return Err(PublicUrlError::PrivateBucket);
        };

        let url = objects::public_url(base_url, &self.key);
        Ok(url)
    }
}

fn replace_url_prefix(orig_url: String, base: String) -> String {
    match Url::parse(&orig_url) {
        Ok(url) => {
            let mut out = format!(
                "{}/{}",
                base.trim_end_matches('/'),
                url.path().trim_start_matches("/")
            );
            if let Some(query) = url.query() {
                out.push('?');
                out.push_str(query);
            }
            out
        }
        Err(_) => {
            // If the input URL fails parsing, just don't do the replace
            orig_url
        }
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

fn map_sign_err(err: gcs::sign::SignedURLError) -> Error {
    match err {
        gcs::sign::SignedURLError::InvalidOption(_e) => Error::PreconditionFailed,
        err => Error::Internal(anyhow::anyhow!(err)),
    }
}
