use std::future::Future;

use crate::{
    cache::{error::Error, OpError, OpResult},
    model::Request,
    trace::{
        protocol::{self, CacheCallStartData, CacheOpResult},
        Tracer,
    },
};

pub(crate) struct CacheTracer(Tracer);

impl CacheTracer {
    pub(crate) fn new(inner: Tracer) -> Self {
        Self(inner)
    }

    pub(crate) async fn trace<'a, T, F, Fut>(
        &self,
        source: Option<&'a Request>,
        operation: &'static str,
        is_write: bool,
        keys: &'a [&'a str],
        f: F,
    ) -> OpResult<T>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = crate::cache::Result<T>>,
    {
        let traced = if let Some(source) = source {
            let start_id = self.0.cache_call_start(CacheCallStartData {
                source,
                operation,
                is_write,
                keys,
            });
            Some((start_id, source))
        } else {
            None
        };

        let result = match f().await {
            Ok(value) => Ok(value),
            Err(err) => Err(OpError::new(
                operation,
                keys.first().copied().unwrap_or(""),
                err.clone(),
            )),
        };

        if let Some((start_id, source)) = traced {
            let (cache_op_result, error) = match result.as_ref() {
                Ok(_) => (CacheOpResult::Ok, None),
                Err(err) => match &err.source {
                    Error::Miss => (CacheOpResult::NoSuchKey, None),
                    Error::KeyExist => (CacheOpResult::Conflict, None),
                    _ => (CacheOpResult::Err, Some(err)),
                },
            };

            self.0.cache_call_end(protocol::CacheCallEndData {
                start_id,
                source,
                result: cache_op_result,
                error,
            });
        }

        result
    }
}
