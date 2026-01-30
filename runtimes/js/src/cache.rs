use crate::api::Request;
use encore_runtime_core::cache;
use napi::bindgen_prelude::*;
use napi::{Error, Status};
use napi_derive::napi;
use std::sync::{Arc, OnceLock};

/// A cache cluster for storing cached data.
#[napi]
pub struct CacheCluster {
    inner: Arc<dyn cache::Cluster>,
    pool: OnceLock<napi::Result<cache::Pool>>,
}

#[napi]
impl CacheCluster {
    pub fn new(inner: Arc<dyn cache::Cluster>) -> napi::Result<Self> {
        Ok(Self {
            inner,
            pool: OnceLock::new(),
        })
    }

    fn pool(&self) -> napi::Result<&cache::Pool> {
        self.pool
            .get_or_init(|| {
                self.inner.pool().map_err(|e| {
                    Error::new(Status::GenericFailure, format!("failed to create pool: {e}"))
                })
            })
            .as_ref()
            .map_err(|e| Error::new(e.status, e.reason.clone()))
    }

    // ==================== Basic Operations ====================

    /// Get a value by key.
    #[napi]
    pub async fn get(&self, key: String, source: Option<&Request>) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.pool()?.get(&key, source).await.map_err(to_error)?;
        Ok(result.map(|v| v.into()))
    }

    /// Set a value by key with optional TTL in milliseconds.
    #[napi]
    pub async fn set(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .set(&key, &value, ttl_ms.map(|t| t as u64), source)
            .await
            .map_err(to_error)
    }

    /// Set a value only if the key doesn't exist.
    #[napi]
    pub async fn set_if_not_exists(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .set_if_not_exists(&key, &value, ttl_ms.map(|t| t as u64), source)
            .await
            .map_err(to_error)
    }

    /// Replace a value only if the key exists.
    #[napi]
    pub async fn replace(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .replace(&key, &value, ttl_ms.map(|t| t as u64), source)
            .await
            .map_err(to_error)
    }

    /// Get old value and set new value atomically.
    #[napi]
    pub async fn get_and_set(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .get_and_set(&key, &value, ttl_ms.map(|t| t as u64), source)
            .await
            .map_err(to_error)?;
        Ok(result.map(|v| v.into()))
    }

    /// Get value and delete key atomically.
    #[napi]
    pub async fn get_and_delete(
        &self,
        key: String,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .get_and_delete(&key, source)
            .await
            .map_err(to_error)?;
        Ok(result.map(|v| v.into()))
    }

    /// Delete one or more keys.
    #[napi]
    pub async fn delete(&self, keys: Vec<String>, source: Option<&Request>) -> napi::Result<u32> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self
            .pool()?
            .delete(&key_refs, source)
            .await
            .map_err(to_error)?;
        Ok(result as u32)
    }

    /// Get multiple values.
    #[napi]
    pub async fn mget(
        &self,
        keys: Vec<String>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Option<Buffer>>> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self.pool()?.mget(&key_refs, source).await.map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.map(|b| b.into())).collect())
    }

    // ==================== String Operations ====================

    /// Append to a string value.
    #[napi]
    pub async fn append(
        &self,
        key: String,
        value: Buffer,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .append(&key, &value, source)
            .await
            .map_err(to_error)
    }

    /// Get a substring of a string value.
    #[napi]
    pub async fn get_range(
        &self,
        key: String,
        start: i32,
        end: i32,
        source: Option<&Request>,
    ) -> napi::Result<Buffer> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .get_range(&key, start as i64, end as i64, source)
            .await
            .map_err(to_error)?;
        Ok(result.into())
    }

    /// Set a substring at a specific offset.
    #[napi]
    pub async fn set_range(
        &self,
        key: String,
        offset: i32,
        value: Buffer,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .set_range(&key, offset as i64, &value, source)
            .await
            .map_err(to_error)
    }

    /// Get string length.
    #[napi]
    pub async fn strlen(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?.strlen(&key, source).await.map_err(to_error)
    }

    // ==================== Numeric Operations ====================

    /// Increment an integer value.
    #[napi]
    pub async fn incr_by(
        &self,
        key: String,
        delta: i64,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .incr_by(&key, delta, source)
            .await
            .map_err(to_error)
    }

    /// Increment a float value.
    #[napi]
    pub async fn incr_by_float(
        &self,
        key: String,
        delta: f64,
        source: Option<&Request>,
    ) -> napi::Result<f64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .incr_by_float(&key, delta, source)
            .await
            .map_err(to_error)
    }

    // ==================== List Operations ====================

    /// Push values to the left (head) of a list.
    #[napi]
    pub async fn lpush(
        &self,
        key: String,
        values: Vec<Buffer>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let value_refs: Vec<&[u8]> = values.iter().map(|v| v.as_ref()).collect();
        self.pool()?
            .lpush(&key, &value_refs, source)
            .await
            .map_err(to_error)
    }

    /// Push values to the right (tail) of a list.
    #[napi]
    pub async fn rpush(
        &self,
        key: String,
        values: Vec<Buffer>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let value_refs: Vec<&[u8]> = values.iter().map(|v| v.as_ref()).collect();
        self.pool()?
            .rpush(&key, &value_refs, source)
            .await
            .map_err(to_error)
    }

    /// Pop values from the left (head) of a list.
    #[napi]
    pub async fn lpop(
        &self,
        key: String,
        count: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .lpop(&key, count.map(|c| c as usize), source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Pop values from the right (tail) of a list.
    #[napi]
    pub async fn rpop(
        &self,
        key: String,
        count: Option<u32>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .rpop(&key, count.map(|c| c as usize), source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get element at index from a list.
    #[napi]
    pub async fn lindex(
        &self,
        key: String,
        index: i32,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .lindex(&key, index as i64, source)
            .await
            .map_err(to_error)?;
        Ok(result.map(|v| v.into()))
    }

    /// Get a range of elements from a list.
    #[napi]
    pub async fn lrange(
        &self,
        key: String,
        start: i32,
        stop: i32,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .lrange(&key, start as i64, stop as i64, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get list length.
    #[napi]
    pub async fn llen(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?.llen(&key, source).await.map_err(to_error)
    }

    // ==================== Set Operations ====================

    /// Add members to a set.
    #[napi]
    pub async fn sadd(
        &self,
        key: String,
        members: Vec<Buffer>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let member_refs: Vec<&[u8]> = members.iter().map(|v| v.as_ref()).collect();
        self.pool()?
            .sadd(&key, &member_refs, source)
            .await
            .map_err(to_error)
    }

    /// Remove members from a set.
    #[napi]
    pub async fn srem(
        &self,
        key: String,
        members: Vec<Buffer>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let member_refs: Vec<&[u8]> = members.iter().map(|v| v.as_ref()).collect();
        self.pool()?
            .srem(&key, &member_refs, source)
            .await
            .map_err(to_error)
    }

    /// Check if member exists in set.
    #[napi]
    pub async fn sismember(
        &self,
        key: String,
        member: Buffer,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .sismember(&key, &member, source)
            .await
            .map_err(to_error)
    }

    /// Get all members of a set.
    #[napi]
    pub async fn smembers(
        &self,
        key: String,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .pool()?
            .smembers(&key, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get set cardinality.
    #[napi]
    pub async fn scard(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?.scard(&key, source).await.map_err(to_error)
    }

    // ==================== Expiry Operations ====================

    /// Set expiry on a key in milliseconds.
    #[napi]
    pub async fn pexpire(
        &self,
        key: String,
        ttl_ms: u32,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        self.pool()?
            .pexpire(&key, ttl_ms as u64, source)
            .await
            .map_err(to_error)
    }
}

fn to_error(e: cache::Error) -> napi::Error {
    Error::new(Status::GenericFailure, format!("cache error: {e}"))
}
