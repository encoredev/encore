use encore_runtime_core::objects as core;
use napi::bindgen_prelude::Buffer;
use napi::{Env, JsBuffer, JsObject};
use napi_derive::napi;

use crate::api::Request;

#[napi]
pub struct Bucket {
    bkt: core::Bucket,
}

#[napi]
impl Bucket {
    pub(crate) fn new(bkt: core::Bucket) -> Self {
        Self { bkt }
    }

    #[napi]
    pub fn object(&self, name: String) -> BucketObject {
        BucketObject::new(self.bkt.object(name))
    }

    #[napi]
    pub async fn list(
        &self,
        options: Option<ListOptions>,
        source: Option<&Request>,
    ) -> napi::Either<ListIterator, TypedObjectError> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.bkt.list(options, source).await {
            Ok(iter) => napi::Either::A(ListIterator::new(iter)),
            Err(err) => napi::Either::B(err.into()),
        }
    }
}

#[napi]
pub struct BucketObject {
    obj: core::Object,
}

#[napi]
impl BucketObject {
    pub(crate) fn new(obj: core::Object) -> Self {
        Self { obj }
    }

    #[napi]
    pub async fn attrs(
        &self,
        options: Option<AttrsOptions>,
        source: Option<&Request>,
    ) -> napi::Either<ObjectAttrs, TypedObjectError> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.obj.attrs(options, source).await {
            Ok(attrs) => napi::Either::A(attrs.into()),
            Err(err) => napi::Either::B(err.into()),
        }
    }

    #[napi]
    pub async fn exists(
        &self,
        options: Option<ExistsOptions>,
        source: Option<&Request>,
    ) -> napi::Either<bool, TypedObjectError> {
        let opts = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.obj.exists(opts, source).await {
            Ok(val) => napi::Either::A(val),
            Err(err) => napi::Either::B(err.into()),
        }
    }

    #[napi(ts_return_type = "Promise<ObjectAttrs | TypedObjectError>")]
    pub fn upload(
        &self,
        env: Env,
        data: JsBuffer,
        opts: Option<UploadOptions>,
        source: Option<&Request>,
    ) -> napi::Result<JsObject> {
        // TODO: reference the data via a Ref, so that we can keep it alive throughout the upload.
        let data = data.into_value()?.as_ref().to_vec();

        let cursor = std::io::Cursor::new(data);
        let opts = opts.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());

        let fut = self.obj.upload(Box::new(cursor), opts, source);

        // We need to always execute the handler below so that we can decrement the ref count.
        // To do so, we need the future to be a napi::Result::Ok. So wrap the result inside that
        // so that the handler gets called regardless of result.
        let fut = async move { Ok(fut.await) };

        env.execute_tokio_future(fut, move |&mut _env, result| {
            // TODO: Decrement the ref count on the data buffer.
            match result {
                Ok(attrs) => Ok(napi::Either::A(ObjectAttrs::from(attrs))),
                Err(err) => Ok(napi::Either::B(TypedObjectError::from(err))),
            }
        })
    }

    #[napi]
    pub async fn signed_upload_url(
        &self,
        options: Option<UploadUrlOptions>,
        source: Option<&Request>,
    ) -> napi::Either<SignedUploadUrl, TypedObjectError> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.obj.signed_upload_url(options, source).await {
            Ok(url) => napi::Either::A(SignedUploadUrl { url }),
            Err(err) => napi::Either::B(err.into()),
        }
    }

    #[napi]
    pub async fn download_all(
        &self,
        options: Option<DownloadOptions>,
        source: Option<&Request>,
    ) -> napi::Either<Buffer, TypedObjectError> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.obj.download_all(options, source).await {
            Ok(buf) => napi::Either::A(buf.into()),
            Err(err) => napi::Either::B(err.into()),
        }
    }

    #[napi]
    pub async fn delete(
        &self,
        options: Option<DeleteOptions>,
        source: Option<&Request>,
    ) -> Option<TypedObjectError> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        match self.obj.delete(options, source).await {
            Ok(()) => None,
            Err(err) => Some(err.into()),
        }
    }

    #[napi]
    pub fn public_url(&self) -> napi::Result<String> {
        match self.obj.public_url() {
            Ok(url) => Ok(url),
            Err(err) => Err(napi::Error::new(
                napi::Status::GenericFailure,
                err.to_string(),
            )),
        }
    }
}

#[napi]
pub struct ObjectAttrs {
    pub name: String,
    pub version: Option<String>,
    pub size: i64,
    pub content_type: Option<String>,
    pub etag: String,
}

impl From<core::ObjectAttrs> for ObjectAttrs {
    fn from(value: core::ObjectAttrs) -> Self {
        Self {
            name: value.name,
            version: value.version,
            size: value.size as i64,
            content_type: value.content_type,
            etag: value.etag,
        }
    }
}

#[napi]
pub struct ListEntry {
    pub name: String,
    pub size: i64,
    pub etag: String,
}

impl From<core::ListEntry> for ListEntry {
    fn from(value: core::ListEntry) -> Self {
        Self {
            name: value.name,
            size: value.size as i64,
            etag: value.etag,
        }
    }
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct UploadOptions {
    pub content_type: Option<String>,
    pub preconditions: Option<UploadPreconditions>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct UploadPreconditions {
    pub not_exists: Option<bool>,
}

impl From<UploadOptions> for core::UploadOptions {
    fn from(value: UploadOptions) -> Self {
        Self {
            content_type: value.content_type,
            preconditions: value.preconditions.map(|p| p.into()),
        }
    }
}

impl From<UploadPreconditions> for core::UploadPreconditions {
    fn from(value: UploadPreconditions) -> Self {
        Self {
            not_exists: value.not_exists,
        }
    }
}

#[napi]
pub enum ObjectErrorKind {
    NotFound,
    PreconditionFailed,
    Other,
    Internal,
}

#[napi]
pub struct TypedObjectError {
    pub kind: ObjectErrorKind,
    pub message: String,
}

impl From<core::Error> for TypedObjectError {
    fn from(value: core::Error) -> Self {
        let kind = match &value {
            core::Error::NotFound => ObjectErrorKind::NotFound,
            core::Error::PreconditionFailed => ObjectErrorKind::PreconditionFailed,
            core::Error::Internal(_) => ObjectErrorKind::Internal,
            core::Error::Other(_) => ObjectErrorKind::Other,
        };
        Self {
            kind,
            message: value.to_string(),
        }
    }
}

#[napi]
pub struct ListIterator {
    stream: tokio::sync::Mutex<Option<core::ListIterator>>,
}

#[napi]
impl ListIterator {
    fn new(stream: core::ListIterator) -> Self {
        Self {
            stream: tokio::sync::Mutex::new(Some(stream)),
        }
    }

    #[napi]
    pub async fn next(&self) -> napi::Result<Option<ListEntry>> {
        let mut stream = self.stream.lock().await;
        if let Some(stream) = stream.as_mut() {
            let row =
                stream.next().await.transpose().map_err(|e| {
                    napi::Error::new(napi::Status::GenericFailure, format!("{:#?}", e))
                })?;
            Ok(row.map(ListEntry::from))
        } else {
            Err(napi::Error::new(
                napi::Status::GenericFailure,
                "iterator is closed",
            ))
        }
    }

    #[napi]
    pub fn mark_done(&mut self) {
        if let Some(stream) = self.stream.get_mut().take() {
            drop(stream);
        };
    }
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct ExistsOptions {
    pub version: Option<String>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct AttrsOptions {
    pub version: Option<String>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct UploadUrlOptions {
    pub ttl: Option<i64>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct SignedUploadUrl {
    pub url: String,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct DeleteOptions {
    pub version: Option<String>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct DownloadOptions {
    pub version: Option<String>,
}

#[napi(object)]
#[derive(Debug, Default)]
pub struct ListOptions {
    pub prefix: Option<String>,
    pub limit: Option<i64>,
}

impl From<DownloadOptions> for core::DownloadOptions {
    fn from(value: DownloadOptions) -> Self {
        Self {
            version: value.version,
        }
    }
}

impl From<DeleteOptions> for core::DeleteOptions {
    fn from(value: DeleteOptions) -> Self {
        Self {
            version: value.version,
        }
    }
}

impl From<ExistsOptions> for core::ExistsOptions {
    fn from(value: ExistsOptions) -> Self {
        Self {
            version: value.version,
        }
    }
}

impl From<AttrsOptions> for core::AttrsOptions {
    fn from(value: AttrsOptions) -> Self {
        Self {
            version: value.version,
        }
    }
}

impl From<UploadUrlOptions> for core::UploadUrlOptions {
    fn from(value: UploadUrlOptions) -> Self {
        Self {
            ttl: value.ttl.map(|v| v as u64).unwrap_or(3600),
        }
    }
}

impl From<ListOptions> for core::ListOptions {
    fn from(value: ListOptions) -> Self {
        Self {
            prefix: value.prefix,
            limit: value.limit.map(|v| v as u64),
        }
    }
}
