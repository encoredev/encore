use napi_derive::napi;

use encore_runtime_core::objects::{Bucket as CoreBucket, Object as CoreObject};

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
    pub async fn exists(&self) -> napi::Result<bool> {
        self.obj.exists().await.map_err(napi::Error::from)
    }
}
