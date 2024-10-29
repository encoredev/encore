use std::pin::Pin;

use napi::bindgen_prelude::Buffer;
use napi::{Env, JsBuffer, JsObject};
use napi_derive::napi;

use encore_runtime_core::objects::{
    Bucket as CoreBucket, DownloadError, ListEntry as CoreListEntry, ListStream,
    Object as CoreObject, ObjectAttrs as CoreAttrs, UploadOptions as CoreUploadOptions,
    UploadPreconditions as CoreUploadPreconditions,
};

#[napi]
pub struct Bucket {
    bkt: CoreBucket,
}

#[napi]
impl Bucket {
    pub(crate) fn new(bkt: CoreBucket) -> Self {
        Self { bkt }
    }

    #[napi]
    pub fn object(&self, name: String) -> BucketObject {
        BucketObject::new(self.bkt.object(name))
    }

    #[napi]
    pub async fn list(&self) -> napi::Result<ListIterator> {
        self.bkt
            .list()
            .await
            .map_err(map_objects_err)
            .map(ListIterator::new)
    }
}

#[napi]
pub struct BucketObject {
    obj: CoreObject,
}

#[napi]
impl BucketObject {
    pub(crate) fn new(obj: CoreObject) -> Self {
        Self { obj }
    }

    #[napi]
    pub async fn attrs(&self) -> napi::Result<ObjectAttrs> {
        self.obj
            .attrs()
            .await
            .map(ObjectAttrs::from)
            .map_err(map_objects_err)
    }

    #[napi]
    pub async fn exists(&self) -> napi::Result<bool> {
        self.obj.exists().await.map_err(napi::Error::from)
    }

    #[napi(ts_return_type = "Promise<ObjectAttrs>")]
    pub fn upload(
        &self,
        env: Env,
        data: JsBuffer,
        opts: Option<UploadOptions>,
    ) -> napi::Result<JsObject> {
        // TODO: reference the data via a Ref, so that we can keep it alive throughout the upload.
        let data = data.into_value()?.as_ref().to_vec();

        let cursor = std::io::Cursor::new(data);
        let opts = opts.unwrap_or_default().into();
        let fut = self.obj.upload(Box::new(cursor), opts);

        // We need to always execute the handler below so that we can decrement the ref count.
        // To do so, we need the future to be a napi::Result::Ok. So wrap the result inside that
        // so that the handler gets called regardless of result.
        let fut = async move { Ok(fut.await) };

        env.execute_tokio_future(fut, move |&mut env, result| {
            // TODO: Decrement the ref count on the data buffer.
            result.map_err(napi::Error::from)
        })
    }

    #[napi]
    pub async fn download_all(&self) -> napi::Result<Buffer> {
        let buf = self.obj.download_all().await.map_err(map_download_err)?;
        Ok(buf.into())
    }

    #[napi]
    pub async fn delete(&self) -> napi::Result<()> {
        self.obj.delete().await.map_err(map_objects_err)
    }
}

#[napi]
pub struct ObjectAttrs {
    pub name: String,
    pub version: String,
    pub size: i64,
    pub content_type: Option<String>,
    pub etag: String,
}

impl From<CoreAttrs> for ObjectAttrs {
    fn from(value: CoreAttrs) -> Self {
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

impl From<CoreListEntry> for ListEntry {
    fn from(value: CoreListEntry) -> Self {
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

impl From<UploadOptions> for CoreUploadOptions {
    fn from(value: UploadOptions) -> Self {
        Self {
            content_type: value.content_type,
            preconditions: value.preconditions.map(|p| p.into()),
        }
    }
}

impl From<UploadPreconditions> for CoreUploadPreconditions {
    fn from(value: UploadPreconditions) -> Self {
        Self {
            not_exists: value.not_exists,
        }
    }
}

fn map_objects_err(err: encore_runtime_core::objects::Error) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, err)
}

fn map_download_err(err: DownloadError) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, err)
}

#[napi]
pub struct ListIterator {
    stream: tokio::sync::Mutex<Pin<ListStream>>,
}

#[napi]
impl ListIterator {
    fn new(stream: ListStream) -> Self {
        Self {
            stream: tokio::sync::Mutex::new(stream.into()),
        }
    }

    #[napi]
    pub async fn next(&self) -> napi::Result<Option<ListEntry>> {
        use futures::StreamExt;
        let mut stream = self.stream.lock().await;
        let row = stream
            .next()
            .await
            .transpose()
            .map_err(|e| napi::Error::new(napi::Status::GenericFailure, format!("{:#?}", e)))?;

        Ok(row.map(ListEntry::from))
    }
}
