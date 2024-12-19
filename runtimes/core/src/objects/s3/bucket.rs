use async_stream::{stream, try_stream};
use aws_sdk_s3 as s3;
use aws_sdk_s3::error::SdkError;
use aws_sdk_s3::presigning::PresigningConfig;
use aws_sdk_s3::presigning::PresigningConfigError;
use aws_smithy_types::byte_stream::ByteStream;
use base64::Engine;
use bytes::{Bytes, BytesMut};
use futures::Stream;
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::task::Poll;
use std::time::Duration;
use tokio::io::{AsyncRead, AsyncReadExt};

use crate::encore::runtime::v1 as pb;
use crate::objects::{
    self, AttrsOptions, DeleteOptions, DownloadOptions, Error, ExistsOptions, ListEntry,
    ListOptions, ObjectAttrs, PublicUrlError, UploadUrlOptions,
};
use crate::{CloudName, EncoreName};

use super::LazyS3Client;

const CHUNK_SIZE: usize = 8_388_608; // 8 Mebibytes, min is 5 (5_242_880);

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyS3Client>,
    encore_name: EncoreName,
    cloud_name: CloudName,
    public_base_url: Option<String>,
    key_prefix: Option<String>,
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyS3Client>, cfg: &pb::Bucket) -> Self {
        Self {
            client,
            encore_name: cfg.encore_name.clone().into(),
            cloud_name: cfg.cloud_name.clone().into(),
            public_base_url: cfg.public_base_url.clone(),
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
    fn name(&self) -> &EncoreName {
        &self.encore_name
    }

    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            bkt: self.clone(),
            name,
        })
    }

    fn list(
        self: Arc<Self>,
        options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, objects::Error>> + Send + 'static>>
    {
        Box::pin(async move {
            let mut total_seen = 0;
            let client = self.client.get().await.clone();
            let s: objects::ListStream = Box::new(try_stream! {
                let mut req = client.list_objects_v2()
                    .bucket(&self.cloud_name);

                if let Some(key_prefix) = self.key_prefix.clone() {
                    req = req.prefix(key_prefix);
                }

                let page_size = if let Some(limit) = options.limit {
                    limit.min(1000) as i32
                } else {
                    1000
                };

                let mut stream = req.into_paginator()
                    .page_size(page_size)
                    .send();

                'PageLoop:
                while let Some(resp) = stream.try_next().await.map_err(|e| Error::Other(e.into()))? {
                    for obj in resp.contents.unwrap_or_default() {
                        total_seen += 1;
                        if let Some(limit) = options.limit {
                            if total_seen > limit {
                                // We've reached the limit, stop the stream.
                                break 'PageLoop;
                            }
                        }

                        let entry = ListEntry {
                            name: self.strip_prefix(Cow::Owned(obj.key.unwrap_or_default())).into_owned(),
                            size: obj.size.unwrap_or_default() as u64,
                            etag: parse_etag(obj.e_tag),
                        };
                        yield entry;
                    }
                }
            });

            Ok(s)
        })
    }
}

#[derive(Debug)]
struct Object {
    bkt: Arc<Bucket>,
    name: String,
}

impl objects::ObjectImpl for Object {
    fn bucket_name(&self) -> &EncoreName {
        &self.bkt.encore_name
    }

    fn key(&self) -> &str {
        &self.name
    }

    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = client
                .head_object()
                .bucket(&self.bkt.cloud_name)
                .key(cloud_name)
                .set_version_id(options.version)
                .send()
                .await;

            match res {
                Ok(obj) => Ok(ObjectAttrs {
                    name: self.name.clone(),
                    version: obj.version_id,
                    size: obj.content_length.unwrap_or_default() as u64,
                    content_type: obj.content_type,
                    etag: parse_etag(obj.e_tag),
                }),
                Err(SdkError::ServiceError(err)) if err.err().is_not_found() => {
                    Err(Error::NotFound)
                }
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn signed_upload_url(
        self: Arc<Self>,
        options: UploadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let obj_name = self.bkt.obj_name(Cow::Borrowed(&self.name));

            let ttl = Duration::from_secs(options.ttl);

            let res = client
                .put_object()
                .bucket(&self.bkt.cloud_name)
                .key(obj_name)
                .presigned(PresigningConfig::expires_in(ttl).map_err(map_sign_config_err)?)
                .await;
            match res {
                Ok(req) => Ok(String::from(req.uri())),
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn exists(
        self: Arc<Self>,
        options: ExistsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = client
                .head_object()
                .bucket(&self.bkt.cloud_name)
                .key(cloud_name)
                .set_version_id(options.version)
                .send()
                .await;
            match res {
                Ok(_) => Ok(true),
                Err(SdkError::ServiceError(err)) if err.err().is_not_found() => Ok(false),
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn upload(
        self: Arc<Self>,
        mut data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let first_chunk = read_chunk_async(&mut data).await.map_err(|e| {
                Error::Other(anyhow::anyhow!("uable to read from data source: {}", e))
            })?;

            match first_chunk {
                Chunk::Complete(chunk) => {
                    // The file is small; do a regular upload.
                    let chunk = chunk.freeze();
                    let total_size = chunk.len();
                    let content_md5 = base64::engine::general_purpose::STANDARD
                        .encode(md5::compute(&chunk).as_ref());

                    let mut req = client
                        .put_object()
                        .bucket(&self.bkt.cloud_name)
                        .key(cloud_name)
                        .content_length(total_size as i64)
                        .content_md5(content_md5)
                        .set_content_type(options.content_type.clone())
                        .body(ByteStream::from(chunk));

                    if let Some(precond) = options.preconditions {
                        if precond.not_exists == Some(true) {
                            req = req.if_none_match("*");
                        }
                    }

                    let resp = req.send().await.map_err(map_upload_err)?;
                    Ok(ObjectAttrs {
                        name: self.name.clone(),
                        version: resp.version_id,
                        size: total_size as u64,
                        content_type: options.content_type,
                        etag: resp.e_tag.unwrap_or_default(),
                    })
                }

                Chunk::Part(chunk) => {
                    // Large file; do a multipart upload.
                    let upload = client
                        .create_multipart_upload()
                        .bucket(&self.bkt.cloud_name)
                        .key(cloud_name.to_string())
                        .set_content_type(options.content_type.clone())
                        .send()
                        .await
                        .map_err(|err| {
                            Error::Other(anyhow::anyhow!(
                                "unable to begin streaming upload: {}",
                                err
                            ))
                        })?;

                    let Some(upload_id) = upload.upload_id else {
                        return Err(Error::Other(anyhow::anyhow!(
                            "missing upload_id in streaming_upload"
                        )));
                    };

                    let res = upload_multipart_chunks(
                        &client,
                        &mut data,
                        chunk.freeze(),
                        &upload_id,
                        &self.bkt.cloud_name,
                        &cloud_name,
                        &options,
                    )
                    .await;

                    if let UploadMultipartResult::CompleteSuccess { total_size, output } = res {
                        return Ok(ObjectAttrs {
                            name: self.name.clone(),
                            version: output.version_id,
                            size: total_size,
                            content_type: options.content_type,
                            etag: parse_etag(output.e_tag),
                        });
                    }

                    // Abort the upload.
                    let fut = client
                        .abort_multipart_upload()
                        .bucket(&self.bkt.cloud_name)
                        .key(cloud_name)
                        .upload_id(upload_id)
                        .send();
                    tokio::spawn(async move {
                        let _ = fut.await;
                    });

                    Err(match res {
                        UploadMultipartResult::CompleteSuccess { .. } => unreachable!(),
                        UploadMultipartResult::UploadError(err) => Error::Other(err.into()),
                        UploadMultipartResult::CompleteError(err) => map_upload_err(err),
                        UploadMultipartResult::ReadContents(err) => Error::Other(anyhow::anyhow!(
                            "unable to read from data source: {}",
                            err
                        )),
                    })
                }
            }
        })
    }

    fn download(
        self: Arc<Self>,
        options: DownloadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::DownloadStream, objects::Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = client
                .get_object()
                .bucket(&self.bkt.cloud_name)
                .key(cloud_name.into_owned())
                .set_version_id(options.version)
                .send()
                .await;

            match res {
                Ok(mut resp) => {
                    let result = stream! {
                        while let Some(chunk) = resp.body.next().await {
                            yield chunk.map_err(|e| objects::Error::Other(e.into()));
                        }
                    };
                    let result: objects::DownloadStream = Box::pin(result);
                    Ok(result)
                }
                Err(SdkError::ServiceError(err)) if err.err().is_no_such_key() => {
                    Err(objects::Error::NotFound)
                }
                Err(err) => Err(objects::Error::Other(err.into())),
            }
        })
    }

    fn delete(
        self: Arc<Self>,
        options: DeleteOptions,
    ) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
            let res = client
                .delete_object()
                .bucket(&self.bkt.cloud_name)
                .key(cloud_name.into_owned())
                .set_version_id(options.version)
                .send()
                .await;
            match res {
                Ok(_) => Ok(()),
                Err(SdkError::ServiceError(err)) if err.raw().status().as_u16() == 404 => {
                    Err(Error::NotFound)
                }
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn public_url(&self) -> Result<String, PublicUrlError> {
        let Some(base_url) = self.bkt.public_base_url.clone() else {
            return Err(PublicUrlError::PrivateBucket);
        };

        let url = objects::public_url(base_url, &self.name);
        Ok(url)
    }
}

enum Chunk {
    Part(BytesMut),
    Complete(BytesMut),
}

impl Chunk {
    fn into_bytes(self) -> BytesMut {
        match self {
            Chunk::Part(buf) => buf,
            Chunk::Complete(buf) => buf,
        }
    }
}

async fn read_chunk_async<R: AsyncRead + Unpin + ?Sized>(reader: &mut R) -> std::io::Result<Chunk> {
    // Use an initial capacity of 10KiB.
    let mut buf = BytesMut::with_capacity(10 * 1024);
    while buf.len() < CHUNK_SIZE {
        // If the buf has no available capacity, we need to allocate more.
        if buf.len() == buf.capacity() {
            buf.reserve(buf.capacity());
        }

        let n = reader.read_buf(&mut buf).await?;
        if n == 0 {
            // We've reached the end of the stream.
            // This is guaranteed to be the case since in the case
            // where the buffer was full we reserved
            // additional capacity in the buffer before reading.
            return Ok(Chunk::Complete(buf));
        }
    }

    Ok(Chunk::Part(buf))
}

#[allow(clippy::large_enum_variant)]
enum UploadMultipartResult {
    CompleteSuccess {
        total_size: u64,
        output: s3::operation::complete_multipart_upload::CompleteMultipartUploadOutput,
    },
    CompleteError(
        s3::error::SdkError<s3::operation::complete_multipart_upload::CompleteMultipartUploadError>,
    ),
    UploadError(s3::error::SdkError<s3::operation::upload_part::UploadPartError>),
    ReadContents(std::io::Error),
}

async fn upload_multipart_chunks<R: AsyncRead + Unpin + ?Sized>(
    client: &s3::Client,
    reader: &mut R,
    first_chunk: Bytes,
    upload_id: &str,
    bucket: &CloudName,
    key: &str,
    options: &objects::UploadOptions,
) -> UploadMultipartResult {
    let mut handles = Vec::new();
    let mut part_number = 0;
    let mut total_size = 0;
    let mut upload_part = |chunk: Bytes| {
        part_number += 1;
        let content_md5 =
            base64::engine::general_purpose::STANDARD.encode(md5::compute(&chunk).as_ref());
        total_size += chunk.len() as u64;
        let handle = client
            .upload_part()
            .bucket(bucket)
            .key(key)
            .upload_id(upload_id)
            .part_number(part_number)
            .content_length(chunk.len() as i64)
            .content_md5(content_md5)
            .body(ByteStream::from(chunk))
            .send();
        handles.push(handle);
    };

    upload_part(first_chunk);
    loop {
        let bytes = match read_chunk_async(reader).await {
            Ok(chunk) => chunk.into_bytes(),
            Err(err) => return UploadMultipartResult::ReadContents(err),
        };
        if bytes.is_empty() {
            break;
        }
        upload_part(bytes.freeze());
    }

    // Wait for all the parts to finish uploading.
    let responses = futures::future::join_all(handles).await;

    // Check for errors.
    for res in responses {
        if let Err(err) = res {
            return UploadMultipartResult::UploadError(err);
        }
    }

    let mut req = client
        .complete_multipart_upload()
        .bucket(bucket)
        .key(key)
        .upload_id(upload_id);

    if let Some(precond) = &options.preconditions {
        if precond.not_exists == Some(true) {
            req = req.if_none_match("*");
        }
    }

    let resp = req.send().await;
    match resp {
        Ok(output) => UploadMultipartResult::CompleteSuccess { total_size, output },
        Err(err) => UploadMultipartResult::CompleteError(err),
    }
}

struct ObjectStream {
    inner: ByteStream,
}

impl Stream for ObjectStream {
    type Item = Result<Bytes, objects::Error>;

    fn poll_next(
        self: Pin<&mut Self>,
        cx: &mut std::task::Context,
    ) -> std::task::Poll<Option<Self::Item>> {
        let stream = Pin::new(&mut self.get_mut().inner);
        match stream.poll_next(cx) {
            Poll::Ready(Some(Err(err))) => {
                Poll::Ready(Some(Err(objects::Error::Other(err.into()))))
            }
            Poll::Ready(Some(Ok(data))) => Poll::Ready(Some(Ok(data))),
            Poll::Ready(None) => Poll::Ready(None),
            Poll::Pending => Poll::Pending,
        }
    }
}

fn parse_etag(s: Option<String>) -> String {
    match s {
        Some(s) => {
            if s.starts_with('"') && s.ends_with('"') {
                s[1..s.len() - 1].to_string()
            } else {
                s
            }
        }
        None => "".to_string(),
    }
}

fn map_upload_err<E>(err: s3::error::SdkError<E>) -> objects::Error
where
    E: std::fmt::Debug,
{
    if err
        .raw_response()
        .is_some_and(|r| r.status().as_u16() == 412)
    {
        Error::PreconditionFailed
    } else {
        Error::Other(anyhow::anyhow!("failed to upload: {:?}", err))
    }
}

fn map_sign_config_err(_err: PresigningConfigError) -> objects::Error {
    // We can't access the kind of error, unfortunately, but currently all
    // possible error kinds are related to expiration time.
    Error::PreconditionFailed
}
