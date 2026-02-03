use std::num::NonZeroUsize;
use std::time::Instant;

use anyhow::Context;
use redis::aio::ConnectionManager;
use redis::{AsyncCommands, RedisResult};
use tokio::sync::OnceCell;

use crate::cache::error::{OpResult, Result};
use crate::model::{Request, SpanKey};
use crate::trace::protocol::{BasicEventData, EventType};
use crate::trace::Tracer;

/// A connection pool to a Redis cache cluster.
/// Uses ConnectionManager for automatic reconnection handling.
pub struct Pool {
    client: redis::Client,
    conn: OnceCell<ConnectionManager>,
    key_prefix: Option<String>,
    tracer: Tracer,
}

impl Pool {
    pub(crate) fn new(
        client: redis::Client,
        key_prefix: Option<String>,
        tracer: Tracer,
        _min_conns: u32,
        _max_conns: u32,
    ) -> anyhow::Result<Self> {
        Ok(Self {
            client,
            conn: OnceCell::new(),
            key_prefix,
            tracer,
        })
    }

    /// Gets or creates the ConnectionManager lazily.
    /// ConnectionManager handles automatic reconnection on connection failures.
    async fn conn(&self) -> Result<ConnectionManager> {
        self.conn
            .get_or_try_init(|| async {
                ConnectionManager::new(self.client.clone())
                    .await
                    .context("failed to connect to Redis")
            })
            .await
            .cloned()
            .map_err(Into::into)
    }

    /// Returns a prefixed key if a key prefix is configured.
    fn prefixed_key(&self, key: &str) -> String {
        match &self.key_prefix {
            Some(prefix) => format!("{}{}", prefix, key),
            None => key.to_string(),
        }
    }

    // ==================== Basic Operations ====================

    /// Get a value by key.
    pub async fn get(&self, key: &str, source: Option<&Request>) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("get", false, &[&key], source);

        let result: RedisResult<Option<Vec<u8>>> = self.conn().await?.get(&key).await;

        match result {
            Ok(value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set a value by key with optional TTL in milliseconds.
    pub async fn set(
        &self,
        key: &str,
        value: &[u8],
        ttl_ms: Option<u64>,
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("set", true, &[&key], source);

        let result: RedisResult<()> = if let Some(ms) = ttl_ms {
            self.conn().await?.set_ex(&key, value, ms / 1000).await
        } else {
            self.conn().await?.set(&key, value).await
        };

        match result {
            Ok(()) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set a value only if the key doesn't exist (SET NX).
    pub async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl_ms: Option<u64>,
        source: Option<&Request>,
    ) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("setnx", true, &[&key], source);

        let result: RedisResult<bool> = {
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("NX");
            if let Some(ms) = ttl_ms {
                cmd.arg("PX").arg(ms);
            }
            cmd.query_async(&mut self.conn().await?).await
        };

        match result {
            Ok(set) => {
                let op_result = if set {
                    OpResult::Ok
                } else {
                    OpResult::Conflict
                };
                self.trace_end(trace, op_result, None);
                Ok(set)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Replace a value only if the key exists (SET XX).
    pub async fn replace(
        &self,
        key: &str,
        value: &[u8],
        ttl_ms: Option<u64>,
        source: Option<&Request>,
    ) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("replace", true, &[&key], source);

        let result: RedisResult<Option<()>> = {
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("XX");
            if let Some(ms) = ttl_ms {
                cmd.arg("PX").arg(ms);
            }
            cmd.query_async(&mut self.conn().await?).await
        };

        match result {
            Ok(Some(())) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(true)
            }
            Ok(None) => {
                self.trace_end(trace, OpResult::NoSuchKey, None);
                Ok(false)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get old value and set new value atomically (SET GET).
    pub async fn get_and_set(
        &self,
        key: &str,
        value: &[u8],
        ttl_ms: Option<u64>,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("getset", true, &[&key], source);

        let result: RedisResult<Option<Vec<u8>>> = {
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("GET");
            if let Some(ms) = ttl_ms {
                cmd.arg("PX").arg(ms);
            }
            cmd.query_async(&mut self.conn().await?).await
        };

        match result {
            Ok(old_value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(old_value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get value and delete key atomically (GETDEL).
    pub async fn get_and_delete(
        &self,
        key: &str,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("getdel", true, &[&key], source);

        let result: RedisResult<Option<Vec<u8>>> = redis::cmd("GETDEL")
            .arg(&key)
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Delete one or more keys.
    pub async fn delete(&self, keys: &[&str], source: Option<&Request>) -> Result<u64> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("del", true, &key_refs, source);

        let result: RedisResult<u64> = self.conn().await?.del(&prefixed).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get multiple values (MGET).
    pub async fn mget(
        &self,
        keys: &[&str],
        source: Option<&Request>,
    ) -> Result<Vec<Option<Vec<u8>>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("mget", false, &key_refs, source);

        let result: RedisResult<Vec<Option<Vec<u8>>>> = self.conn().await?.mget(&prefixed).await;

        match result {
            Ok(values) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== String Operations ====================

    /// Append to a string value.
    pub async fn append(&self, key: &str, value: &[u8], source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("append", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.append(&key, value).await;

        match result {
            Ok(new_len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(new_len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get a substring of a string value.
    pub async fn get_range(
        &self,
        key: &str,
        start: i64,
        end: i64,
        source: Option<&Request>,
    ) -> Result<Vec<u8>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("getrange", false, &[&key], source);

        let result: RedisResult<Vec<u8>> = self
            .conn()
            .await?
            .getrange(&key, start as isize, end as isize)
            .await;

        match result {
            Ok(value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set a substring at a specific offset.
    pub async fn set_range(
        &self,
        key: &str,
        offset: i64,
        value: &[u8],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("setrange", true, &[&key], source);

        let result: RedisResult<i64> = self
            .conn()
            .await?
            .setrange(&key, offset as isize, value)
            .await;

        match result {
            Ok(new_len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(new_len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get string length.
    pub async fn strlen(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("strlen", false, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.strlen(&key).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Numeric Operations ====================

    /// Increment an integer value.
    pub async fn incr_by(&self, key: &str, delta: i64, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("incrby", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.incr(&key, delta).await;

        match result {
            Ok(new_val) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(new_val)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Increment a float value.
    pub async fn incr_by_float(
        &self,
        key: &str,
        delta: f64,
        source: Option<&Request>,
    ) -> Result<f64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("incrbyfloat", true, &[&key], source);

        let result: RedisResult<f64> = self.conn().await?.incr(&key, delta).await;

        match result {
            Ok(new_val) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(new_val)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== List Operations ====================

    /// Push values to the left (head) of a list.
    pub async fn lpush(
        &self,
        key: &str,
        values: &[&[u8]],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lpush", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.lpush(&key, values).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Push values to the right (tail) of a list.
    pub async fn rpush(
        &self,
        key: &str,
        values: &[&[u8]],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("rpush", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.rpush(&key, values).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop value from the left (head) of a list.
    pub async fn lpop(
        &self,
        key: &str,
        count: Option<usize>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lpop", true, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = match count.and_then(NonZeroUsize::new) {
            Some(n) => self.conn().await?.lpop(&key, Some(n)).await,
            None => {
                let single: RedisResult<Option<Vec<u8>>> =
                    self.conn().await?.lpop(&key, None).await;
                single.map(|v| v.into_iter().collect())
            }
        };

        match result {
            Ok(values) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop value from the right (tail) of a list.
    pub async fn rpop(
        &self,
        key: &str,
        count: Option<usize>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("rpop", true, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = match count.and_then(NonZeroUsize::new) {
            Some(n) => self.conn().await?.rpop(&key, Some(n)).await,
            None => {
                let single: RedisResult<Option<Vec<u8>>> =
                    self.conn().await?.rpop(&key, None).await;
                single.map(|v| v.into_iter().collect())
            }
        };

        match result {
            Ok(values) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get element at index from a list.
    pub async fn lindex(
        &self,
        key: &str,
        index: i64,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lindex", false, &[&key], source);

        let result: RedisResult<Option<Vec<u8>>> =
            self.conn().await?.lindex(&key, index as isize).await;

        match result {
            Ok(value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set element at index in a list.
    pub async fn lset(
        &self,
        key: &str,
        index: i64,
        value: &[u8],
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lset", true, &[&key], source);

        let result: RedisResult<()> = self.conn().await?.lset(&key, index as isize, value).await;

        match result {
            Ok(()) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get a range of elements from a list.
    pub async fn lrange(
        &self,
        key: &str,
        start: i64,
        stop: i64,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lrange", false, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = self
            .conn()
            .await?
            .lrange(&key, start as isize, stop as isize)
            .await;

        match result {
            Ok(values) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Trim list to specified range.
    pub async fn ltrim(
        &self,
        key: &str,
        start: i64,
        stop: i64,
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("ltrim", true, &[&key], source);

        let result: RedisResult<()> = self
            .conn()
            .await?
            .ltrim(&key, start as isize, stop as isize)
            .await;

        match result {
            Ok(()) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Insert element before pivot in list.
    pub async fn linsert_before(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("linsert", true, &[&key], source);

        let result: RedisResult<i64> = redis::cmd("LINSERT")
            .arg(&key)
            .arg("BEFORE")
            .arg(pivot)
            .arg(value)
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(pos) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(pos)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Insert element after pivot in list.
    pub async fn linsert_after(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("linsert", true, &[&key], source);

        let result: RedisResult<i64> = redis::cmd("LINSERT")
            .arg(&key)
            .arg("AFTER")
            .arg(pivot)
            .arg(value)
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(pos) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(pos)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Remove elements from list. Count specifies:
    /// - count > 0: remove first count occurrences
    /// - count < 0: remove last |count| occurrences
    /// - count = 0: remove all occurrences
    pub async fn lrem(
        &self,
        key: &str,
        count: i64,
        value: &[u8],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lrem", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.lrem(&key, count as isize, value).await;

        match result {
            Ok(removed) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(removed)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Move element between lists.
    pub async fn lmove(
        &self,
        src: &str,
        dst: &str,
        src_dir: ListDirection,
        dst_dir: ListDirection,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        let trace = self.trace_start("lmove", true, &[&src_key, &dst_key], source);

        let result: RedisResult<Option<Vec<u8>>> = redis::cmd("LMOVE")
            .arg(&src_key)
            .arg(&dst_key)
            .arg(src_dir.as_str())
            .arg(dst_dir.as_str())
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(value) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get list length.
    pub async fn llen(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("llen", false, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.llen(&key).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Set Operations ====================

    /// Add members to a set.
    pub async fn sadd(
        &self,
        key: &str,
        members: &[&[u8]],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("sadd", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.sadd(&key, members).await;

        match result {
            Ok(added) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(added)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Remove members from a set.
    pub async fn srem(
        &self,
        key: &str,
        members: &[&[u8]],
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("srem", true, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.srem(&key, members).await;

        match result {
            Ok(removed) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(removed)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Check if member exists in set.
    pub async fn sismember(
        &self,
        key: &str,
        member: &[u8],
        source: Option<&Request>,
    ) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("sismember", false, &[&key], source);

        let result: RedisResult<bool> = self.conn().await?.sismember(&key, member).await;

        match result {
            Ok(is_member) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(is_member)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop random members from a set.
    pub async fn spop(
        &self,
        key: &str,
        count: Option<usize>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("spop", true, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = match count {
            Some(n) => {
                redis::cmd("SPOP")
                    .arg(&key)
                    .arg(n)
                    .query_async(&mut self.conn().await?)
                    .await
            }
            None => {
                let single: RedisResult<Option<Vec<u8>>> = self.conn().await?.spop(&key).await;
                single.map(|v| v.into_iter().collect())
            }
        };

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get random members from a set without removing (SRANDMEMBER).
    /// If count is negative, may return duplicates.
    pub async fn srandmember(
        &self,
        key: &str,
        count: i64,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("srandmember", false, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = redis::cmd("SRANDMEMBER")
            .arg(&key)
            .arg(count)
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get all members of a set.
    pub async fn smembers(&self, key: &str, source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("smembers", false, &[&key], source);

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.smembers(&key).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get set cardinality.
    pub async fn scard(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("scard", false, &[&key], source);

        let result: RedisResult<i64> = self.conn().await?.scard(&key).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set difference.
    pub async fn sdiff(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sdiff", false, &key_refs, source);

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sdiff(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set difference.
    pub async fn sdiffstore(
        &self,
        dest: &str,
        keys: &[&str],
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        all_keys.extend(prefixed.iter().map(|s| s.as_str()));
        let trace = self.trace_start("sdiffstore", true, &all_keys, source);

        let result: RedisResult<i64> = self.conn().await?.sdiffstore(&dest_key, &prefixed).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set intersection.
    pub async fn sinter(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sinter", false, &key_refs, source);

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sinter(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set intersection.
    pub async fn sinterstore(
        &self,
        dest: &str,
        keys: &[&str],
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        all_keys.extend(prefixed.iter().map(|s| s.as_str()));
        let trace = self.trace_start("sinterstore", true, &all_keys, source);

        let result: RedisResult<i64> = self.conn().await?.sinterstore(&dest_key, &prefixed).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set union.
    pub async fn sunion(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sunion", false, &key_refs, source);

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sunion(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set union.
    pub async fn sunionstore(
        &self,
        dest: &str,
        keys: &[&str],
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        all_keys.extend(prefixed.iter().map(|s| s.as_str()));
        let trace = self.trace_start("sunionstore", true, &all_keys, source);

        let result: RedisResult<i64> = self.conn().await?.sunionstore(&dest_key, &prefixed).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Move member between sets.
    pub async fn smove(
        &self,
        src: &str,
        dst: &str,
        member: &[u8],
        source: Option<&Request>,
    ) -> Result<bool> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        let trace = self.trace_start("smove", true, &[&src_key, &dst_key], source);

        let result: RedisResult<bool> = self.conn().await?.smove(&src_key, &dst_key, member).await;

        match result {
            Ok(moved) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(moved)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Expiry Operations ====================

    /// Set expiry on a key in milliseconds.
    pub async fn pexpire(&self, key: &str, ttl_ms: u64, source: Option<&Request>) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("pexpire", true, &[&key], source);

        let result: RedisResult<bool> = redis::cmd("PEXPIRE")
            .arg(&key)
            .arg(ttl_ms as i64)
            .query_async(&mut self.conn().await?)
            .await;

        match result {
            Ok(set) => {
                self.trace_end(trace, OpResult::Ok, None);
                Ok(set)
            }
            Err(e) => {
                self.trace_end(trace, OpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Tracing ====================

    fn trace_start(
        &self,
        operation: &str,
        is_write: bool,
        keys: &[&str],
        source: Option<&Request>,
    ) -> Option<TraceStart> {
        let source = source?;
        if !source.traced {
            return None;
        }

        Some(TraceStart {
            start: Instant::now(),
            span: source.span,
            operation: operation.to_string(),
            is_write,
            keys: keys.iter().map(|k| k.to_string()).collect(),
        })
    }

    fn trace_end(
        &self,
        trace: Option<TraceStart>,
        result: OpResult,
        err: Option<&redis::RedisError>,
    ) {
        let Some(trace) = trace else { return };

        // Send start event
        // Format must match Go parser: header (def_loc, goid, correlation), then Operation, Write, Stack, Keys
        let start_id = {
            let mut eb = BasicEventData {
                correlation_event_id: None,
                extra_space: 64 + trace.operation.len() + trace.keys.len() * 32,
            }
            .into_eb();

            eb.str(&trace.operation);
            eb.bool(trace.is_write);
            eb.nyi_stack_pcs(); // Stack comes before keys
            eb.uvarint(trace.keys.len() as u64);
            for key in &trace.keys {
                eb.str(key);
            }

            self.tracer
                .send_raw(EventType::CacheCallStart, trace.span, eb)
        };

        // Send end event
        // Format: header (def_loc, goid, correlation), then Result, Err
        {
            let mut eb = BasicEventData {
                correlation_event_id: start_id,
                extra_space: 32,
            }
            .into_eb();

            eb.byte(result as u8);
            eb.err_with_legacy_stack(err);

            self.tracer
                .send_raw(EventType::CacheCallEnd, trace.span, eb);
        }
    }
}

struct TraceStart {
    #[allow(dead_code)]
    start: Instant, // Reserved for future duration tracking
    span: SpanKey,
    operation: String,
    is_write: bool,
    keys: Vec<String>,
}

/// Direction for list operations.
#[derive(Debug, Clone, Copy)]
pub enum ListDirection {
    Left,
    Right,
}

impl ListDirection {
    fn as_str(&self) -> &'static str {
        match self {
            ListDirection::Left => "LEFT",
            ListDirection::Right => "RIGHT",
        }
    }
}
