use anyhow::Context;
use async_stream::{stream, try_stream};
use aws_sdk_s3 as s3;
use aws_sdk_s3::error::SdkError;
use aws_smithy_types::byte_stream::ByteStream;
use base64::Engine;
use bytes::{Bytes, BytesMut};
use futures::Stream;
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::task::Poll;
use tokio::io::{AsyncRead, AsyncReadExt};

use crate::encore::runtime::v1 as pb;
use crate::objects::{
    self, AttrsOptions, DeleteOptions, DownloadOptions, Error, ListEntry, ListOptions, ObjectAttrs,
};

use super::LazyS3Client;

const CHUNK_SIZE: usize = 8_388_608; // 8 Mebibytes, min is 5 (5_242_880);

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyS3Client>,
    name: String,
    key_prefix: Option<String>,
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyS3Client>, cfg: &pb::Bucket) -> Self {
        Self {
            client,
            name: cfg.cloud_name.clone(),
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
        Arc::new(Object {
            bkt: self.clone(),
            cloud_name: name,
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
                    .bucket(self.name.clone());

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
                            etag: obj.e_tag.unwrap_or_default(),
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
    cloud_name: String,
}

impl objects::ObjectImpl for Object {
    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.cloud_name));
            let res = client
                .head_object()
                .bucket(self.bkt.name.clone())
                .key(cloud_name)
                .set_version_id(options.version)
                .send()
                .await;

            match res {
                Ok(obj) => Ok(ObjectAttrs {
                    name: self.cloud_name.clone(),
                    version: obj.version_id.unwrap_or_default(),
                    size: obj.content_length.unwrap_or_default() as u64,
                    content_type: obj.content_type,
                    etag: obj.e_tag.unwrap_or_default(),
                }),
                Err(SdkError::ServiceError(err)) if err.err().is_not_found() => {
                    Err(Error::NotFound)
                }
                Err(err) => Err(Error::Other(err.into())),
            }
        })
    }

    fn exists(
        self: Arc<Self>,
        version: Option<String>,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.cloud_name));
            let res = client
                .head_object()
                .bucket(&self.bkt.name)
                .key(cloud_name)
                .set_version_id(version)
                .send()
                .await;
            match res {
                Ok(_) => Ok(true),
                Err(SdkError::ServiceError(err)) if err.err().is_not_found() => Ok(false),
                Err(err) => Err(err.into()),
            }
        })
    }

    fn upload(
        self: Arc<Self>,
        mut data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        options: objects::UploadOptions,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send>> {
        Box::pin(async move {
            let client = self.bkt.client.get().await.clone();
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.cloud_name));
            let first_chunk = read_chunk_async(&mut data)
                .await
                .context("unable to read from data source")?;

            match first_chunk {
                Chunk::Complete(chunk) => {
                    // The file is small; do a regular upload.
                    let chunk = chunk.freeze();
                    let total_size = chunk.len();
                    let content_md5 = base64::engine::general_purpose::STANDARD
                        .encode(md5::compute(&chunk).as_ref());

                    let mut req = client
                        .put_object()
                        .bucket(&self.bkt.name)
                        .key(cloud_name)
                        .content_length(total_size as i64)
                        .content_md5(content_md5)
                        .set_content_type(options.content_type)
                        .body(ByteStream::from(chunk));

                    if let Some(precond) = options.preconditions {
                        if precond.not_exists == Some(true) {
                            req = req.if_none_match("*");
                        }
                    }

                    let _ = req.send().await?;
                    Ok(())
                }

                Chunk::Part(chunk) => {
                    // Large file; do a multipart upload.
                    let upload = client
                        .create_multipart_upload()
                        .bucket(&self.bkt.name)
                        .key(cloud_name.to_string())
                        .set_content_type(options.content_type)
                        .send()
                        .await
                        .context("unable to begin streaming upload")?;

                    let upload_id = upload
                        .upload_id
                        .context("missing upload_id in streaming upload")?;

                    let res = upload_multipart_chunks(
                        &client,
                        &mut data,
                        chunk.freeze(),
                        &upload_id,
                        &self.bkt.name,
                        &cloud_name,
                    )
                    .await;

                    match res {
                        Ok(()) => Ok(()),
                        Err(err) => {
                            let fut = client
                                .abort_multipart_upload()
                                .bucket(&self.bkt.name)
                                .key(cloud_name)
                                .upload_id(upload_id)
                                .send();
                            tokio::spawn(async move {
                                let _ = fut.await;
                            });
                            return Err(err);
                        }
                    }
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
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.cloud_name));
            let res = client
                .get_object()
                .bucket(&self.bkt.name)
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
            let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.cloud_name));
            let res = client
                .delete_object()
                .bucket(&self.bkt.name)
                .key(cloud_name.into_owned())
                .set_version_id(options.version)
                .send()
                .await;
            match res {
                Ok(_) => Ok(()),
                Err(SdkError::ServiceError(err)) if err.raw().status().as_u16() == 404 => Ok(()),
                Err(err) => Err(Error::Other(err.into())),
            }
        })
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

async fn upload_multipart_chunks<R: AsyncRead + Unpin + ?Sized>(
    client: &s3::Client,
    reader: &mut R,
    first_chunk: Bytes,
    upload_id: &str,
    bucket: &str,
    key: &str,
) -> anyhow::Result<()> {
    let mut handles = Vec::new();
    let mut part_number = 0;
    let mut upload_part = |chunk: Bytes| {
        part_number += 1;
        let content_md5 =
            base64::engine::general_purpose::STANDARD.encode(md5::compute(&chunk).as_ref());
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
        let bytes = read_chunk_async(reader).await?.into_bytes();
        if bytes.is_empty() {
            break;
        }
        upload_part(bytes.freeze());
    }

    // Wait for all the parts to finish uploading.
    let responses = futures::future::join_all(handles).await;

    // Check for errors.
    for res in responses {
        let _ = res?;
    }

    Ok(())
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
