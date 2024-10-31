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
    ) -> napi::Result<ListIterator> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        self.bkt
            .list(options, source)
            .await
            .map_err(map_objects_err)
            .map(ListIterator::new)
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
    ) -> napi::Result<ObjectAttrs> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        self.obj
            .attrs(options, source)
            .await
            .map(ObjectAttrs::from)
            .map_err(map_objects_err)
    }

    #[napi]
    pub async fn exists(
        &self,
        options: Option<ExistsOptions>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let opts = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        self.obj.exists(opts, source).await.map_err(map_objects_err)
    }

    #[napi(ts_return_type = "Promise<ObjectAttrs>")]
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
            result.map(ObjectAttrs::from).map_err(map_objects_err)
        })
    }

    #[napi]
    pub async fn download_all(
        &self,
        options: Option<DownloadOptions>,
        source: Option<&Request>,
    ) -> napi::Result<Buffer> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        let buf = self
            .obj
            .download_all(options, source)
            .await
            .map_err(map_objects_err)?;
        Ok(buf.into())
    }

    #[napi]
    pub async fn delete(
        &self,
        options: Option<DeleteOptions>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let options = options.unwrap_or_default().into();
        let source = source.map(|s| s.inner.clone());
        self.obj
            .delete(options, source)
            .await
            .map_err(map_objects_err)?;
        Ok(true)
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

fn map_objects_err(err: core::Error) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, err)
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

impl From<ListOptions> for core::ListOptions {
    fn from(value: ListOptions) -> Self {
        Self {
            prefix: value.prefix,
            limit: value.limit.map(|v| v as u64),
        }
    }
}
