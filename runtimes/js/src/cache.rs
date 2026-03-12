use crate::api::Request;
use encore_runtime_core::cache;
use encore_runtime_core::cache::TtlOp;
use napi::bindgen_prelude::*;
use napi::{Error, Status};
use napi_derive::napi;
use std::sync::{Arc, OnceLock};

/// Maps i64 sentinel values from TypeScript to TtlOp.
/// - None → None (no TTL config)
/// - -1 → Keep
/// - -2 → Persist (NeverExpire)
/// - >= 0 → SetMs(ms)
fn to_ttl_op(ttl_ms: Option<i64>) -> Option<TtlOp> {
    match ttl_ms {
        None => None,
        Some(-1) => Some(TtlOp::Keep),
        Some(-2) => Some(TtlOp::Persist),
        Some(ms) if ms >= 0 => Some(TtlOp::SetMs(ms as u64)),
        Some(_) => None, // invalid negative values treated as no TTL
    }
}

/// A cache cluster for storing cached data.
#[napi]
pub struct CacheCluster {
    inner: Arc<dyn cache::Cluster>,
    client: OnceLock<napi::Result<cache::Client>>,
}

#[napi]
impl CacheCluster {
    pub fn new(inner: Arc<dyn cache::Cluster>) -> napi::Result<Self> {
        Ok(Self {
            inner,
            client: OnceLock::new(),
        })
    }

    fn client(&self) -> napi::Result<&cache::Client> {
        self.client
            .get_or_init(|| {
                self.inner.client().map_err(|e| {
                    Error::new(
                        Status::GenericFailure,
                        format!("failed to create cache client: {e}"),
                    )
                })
            })
            .as_ref()
            .map_err(|e| Error::new(e.status, e.reason.clone()))
    }

    /// Get a value by key.
    #[napi]
    pub async fn get(&self, key: String, source: Option<&Request>) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.get(&key, source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Set a value by key with optional TTL.
    #[napi]
    pub async fn set(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .set(&key, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Set a value only if the key doesn't exist.
    #[napi]
    pub async fn set_if_not_exists(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .set_if_not_exists(&key, &value, to_ttl_op(ttl_ms), source)
            .await;
        as_bool(result)
    }

    /// Replace a value only if the key exists.
    #[napi]
    pub async fn replace(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .replace(&key, &value, to_ttl_op(ttl_ms), source)
            .await;
        as_bool(result)
    }

    /// Get old value and set new value atomically.
    #[napi]
    pub async fn get_and_set(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .get_and_set(&key, &value, to_ttl_op(ttl_ms), source)
            .await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Get value and delete key atomically.
    #[napi]
    pub async fn get_and_delete(
        &self,
        key: String,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.get_and_delete(&key, source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Delete one or more keys.
    #[napi]
    pub async fn delete(&self, keys: Vec<String>, source: Option<&Request>) -> napi::Result<u32> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self
            .client()?
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
        let result = self
            .client()?
            .mget(&key_refs, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.map(|b| b.into())).collect())
    }

    /// Append to a string value.
    #[napi]
    pub async fn append(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .append(&key, &value, to_ttl_op(ttl_ms), source)
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
            .client()?
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
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .set_range(&key, offset as i64, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Get string length.
    #[napi]
    pub async fn strlen(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?.strlen(&key, source).await.map_err(to_error)
    }

    /// Increment an integer value.
    #[napi]
    pub async fn incr_by(
        &self,
        key: String,
        delta: i64,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .incr_by(&key, delta, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Decrement an integer value.
    #[napi]
    pub async fn decr_by(
        &self,
        key: String,
        delta: i64,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .decr_by(&key, delta, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Increment a float value.
    #[napi]
    pub async fn incr_by_float(
        &self,
        key: String,
        delta: f64,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<f64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .incr_by_float(&key, delta, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Push values to the left (head) of a list.
    #[napi]
    pub async fn lpush(
        &self,
        key: String,
        values: Vec<Buffer>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let value_refs: Vec<&[u8]> = values.iter().map(|v| v.as_ref()).collect();
        self.client()?
            .lpush(&key, &value_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Push values to the right (tail) of a list.
    #[napi]
    pub async fn rpush(
        &self,
        key: String,
        values: Vec<Buffer>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let value_refs: Vec<&[u8]> = values.iter().map(|v| v.as_ref()).collect();
        self.client()?
            .rpush(&key, &value_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Pop a value from the left (head) of a list.
    /// Returns null if the list is empty or doesn't exist.
    #[napi]
    pub async fn lpop(
        &self,
        key: String,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.lpop(&key, to_ttl_op(ttl_ms), source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Pop a value from the right (tail) of a list.
    /// Returns null if the list is empty or doesn't exist.
    #[napi]
    pub async fn rpop(
        &self,
        key: String,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.rpop(&key, to_ttl_op(ttl_ms), source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
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
        let result = self.client()?.lindex(&key, index as i64, source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
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
            .client()?
            .lrange(&key, start as i64, stop as i64, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get all elements of a list (traced as "items").
    #[napi]
    pub async fn lrange_all(
        &self,
        key: String,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .lrange_all(&key, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get list length.
    #[napi]
    pub async fn llen(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?.llen(&key, source).await.map_err(to_error)
    }

    /// Trim a list to the specified range.
    #[napi]
    pub async fn ltrim(
        &self,
        key: String,
        start: i32,
        stop: i32,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .ltrim(&key, start as i64, stop as i64, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Set element at index in list.
    #[napi]
    pub async fn lset(
        &self,
        key: String,
        index: i32,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<()> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .lset(&key, index as i64, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Insert value before pivot in list.
    #[napi]
    pub async fn linsert_before(
        &self,
        key: String,
        pivot: Buffer,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .linsert_before(&key, &pivot, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Insert value after pivot in list.
    #[napi]
    pub async fn linsert_after(
        &self,
        key: String,
        pivot: Buffer,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .linsert_after(&key, &pivot, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Remove elements from list.
    #[napi]
    pub async fn lrem_all(
        &self,
        key: String,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .lrem_all(&key, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Remove elements from list.
    #[napi]
    pub async fn lrem_first(
        &self,
        key: String,
        count: u32,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .lrem_first(&key, count as u64, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Remove elements from list.
    #[napi]
    pub async fn lrem_last(
        &self,
        key: String,
        count: u32,
        value: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .lrem_last(&key, count as u64, &value, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Move element between lists atomically.
    #[napi]
    pub async fn lmove(
        &self,
        src: String,
        dst: String,
        src_dir: String,
        dst_dir: String,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let src_dir = match src_dir.as_str() {
            "left" => cache::ListDirection::Left,
            "right" => cache::ListDirection::Right,
            _ => return Err(Error::new(Status::InvalidArg, "invalid source direction")),
        };
        let dst_dir = match dst_dir.as_str() {
            "left" => cache::ListDirection::Left,
            "right" => cache::ListDirection::Right,
            _ => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "invalid destination direction",
                ))
            }
        };
        let result = self
            .client()?
            .lmove(&src, &dst, src_dir, dst_dir, to_ttl_op(ttl_ms), source)
            .await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Add members to a set.
    #[napi]
    pub async fn sadd(
        &self,
        key: String,
        members: Vec<Buffer>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let member_refs: Vec<&[u8]> = members.iter().map(|v| v.as_ref()).collect();
        self.client()?
            .sadd(&key, &member_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Remove members from a set.
    #[napi]
    pub async fn srem(
        &self,
        key: String,
        members: Vec<Buffer>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let member_refs: Vec<&[u8]> = members.iter().map(|v| v.as_ref()).collect();
        self.client()?
            .srem(&key, &member_refs, to_ttl_op(ttl_ms), source)
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
        self.client()?
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
            .client()?
            .smembers(&key, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get set cardinality.
    #[napi]
    pub async fn scard(&self, key: String, source: Option<&Request>) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?.scard(&key, source).await.map_err(to_error)
    }

    /// Pop a single random member from a set.
    /// Returns null if the set is empty or doesn't exist.
    #[napi]
    pub async fn spop(
        &self,
        key: String,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.spop(&key, to_ttl_op(ttl_ms), source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Pop random members from a set.
    #[napi]
    pub async fn spop_n(
        &self,
        key: String,
        count: u32,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .spop_n(&key, count as usize, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get a single random member from a set (without removing).
    /// Returns null if the set is empty or doesn't exist.
    #[napi]
    pub async fn srandmember(
        &self,
        key: String,
        source: Option<&Request>,
    ) -> napi::Result<Option<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self.client()?.srandmember(&key, source).await;
        Ok(miss_as_none(result)?.map(|v| v.into()))
    }

    /// Get random members from a set (without removing).
    /// Positive count returns distinct elements, negative count may return duplicates.
    #[napi]
    pub async fn srandmember_n(
        &self,
        key: String,
        count: i32,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let result = self
            .client()?
            .srandmember_n(&key, count as i64, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Get the difference between sets.
    #[napi]
    pub async fn sdiff(
        &self,
        keys: Vec<String>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self
            .client()?
            .sdiff(&key_refs, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Store the difference between sets.
    #[napi]
    pub async fn sdiffstore(
        &self,
        destination: String,
        keys: Vec<String>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        self.client()?
            .sdiffstore(&destination, &key_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Get the intersection of sets.
    #[napi]
    pub async fn sinter(
        &self,
        keys: Vec<String>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self
            .client()?
            .sinter(&key_refs, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Store the intersection of sets.
    #[napi]
    pub async fn sinterstore(
        &self,
        destination: String,
        keys: Vec<String>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        self.client()?
            .sinterstore(&destination, &key_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Get the union of sets.
    #[napi]
    pub async fn sunion(
        &self,
        keys: Vec<String>,
        source: Option<&Request>,
    ) -> napi::Result<Vec<Buffer>> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        let result = self
            .client()?
            .sunion(&key_refs, source)
            .await
            .map_err(to_error)?;
        Ok(result.into_iter().map(|v| v.into()).collect())
    }

    /// Store the union of sets.
    #[napi]
    pub async fn sunionstore(
        &self,
        destination: String,
        keys: Vec<String>,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<i64> {
        let source = source.map(|s| s.inner.as_ref());
        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        self.client()?
            .sunionstore(&destination, &key_refs, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }

    /// Move member from one set to another.
    #[napi]
    pub async fn smove(
        &self,
        src: String,
        dst: String,
        member: Buffer,
        ttl_ms: Option<i64>,
        source: Option<&Request>,
    ) -> napi::Result<bool> {
        let source = source.map(|s| s.inner.as_ref());
        self.client()?
            .smove(&src, &dst, &member, to_ttl_op(ttl_ms), source)
            .await
            .map_err(to_error)
    }
}

fn to_error(e: cache::OpError) -> napi::Error {
    Error::new(Status::GenericFailure, format!("{e}"))
}

/// Convert an OpResult into Option, mapping Miss to None.
fn miss_as_none<T>(result: cache::OpResult<T>) -> napi::Result<Option<T>> {
    match result {
        Ok(v) => Ok(Some(v)),
        Err(e) if matches!(e.source, cache::Error::Miss) => Ok(None),
        Err(e) => Err(to_error(e)),
    }
}

/// Convert an OpResult<()> into bool, mapping KeyExist/Miss to false.
fn as_bool(result: cache::OpResult<()>) -> napi::Result<bool> {
    match result {
        Ok(()) => Ok(true),
        Err(e) if matches!(e.source, cache::Error::KeyExist | cache::Error::Miss) => Ok(false),
        Err(e) => Err(to_error(e)),
    }
}
