use napi::{Error, Status};
use napi_derive::napi;
use std::sync::Arc;

#[napi]
pub struct Secret {
    secret: Arc<encore_runtime_core::secrets::Secret>,
}

impl Secret {
    pub fn new(secret: Arc<encore_runtime_core::secrets::Secret>) -> Self {
        Self { secret }
    }
}

#[napi]
impl Secret {
    /// Returns the cached value of the secret.
    #[napi]
    pub fn cached(&self) -> napi::Result<String> {
        let val = self.secret.get().map_err(|e| {
            Error::new(
                Status::GenericFailure,
                format!("failed to resolve secret: {}", e),
            )
        })?;
        String::from_utf8(val.to_vec())
            .map_err(|e| Error::new(Status::GenericFailure, e.to_string()))
    }
}
