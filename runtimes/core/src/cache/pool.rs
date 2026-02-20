use std::num::NonZeroUsize;
use std::sync::Arc;

use bb8::{ErrorSink, Pool as Bb8Pool, RunError};
use bb8_redis::redis::{self as redis, AsyncCommands, RedisResult};
use bb8_redis::RedisConnectionManager;

use crate::cache::error::{Error, Result};
use crate::cache::memcluster::MemoryStore;
use crate::model::{Request, TraceEventId};
use crate::trace::protocol::{CacheCallEndData, CacheCallStartData, CacheOpResult};
use crate::trace::Tracer;

/// TTL operation for cache write commands.
#[derive(Debug, Clone, Copy)]
pub enum TtlOp {
    /// Preserve the existing TTL (KEEPTTL for SET; no-op for others).
    Keep,
    /// Set TTL in milliseconds (PX for SET; atomic PEXPIRE for others).
    SetMs(u64),
    /// Remove TTL / never expire (no TTL flags for SET; atomic PERSIST for others).
    Persist,
}

/// Backend type for the pool.
enum Backend {
    /// Real Redis connection pool.
    Redis {
        pool: Bb8Pool<RedisConnectionManager>,
        key_prefix: Option<String>,
    },
    /// In-memory store (used in Encore Cloud).
    Memory(Arc<MemoryStore>),
}

/// A connection pool to a Redis cache cluster.
/// Can use either a real Redis connection or an in-memory store.
pub struct Pool {
    backend: Backend,
    tracer: Tracer,
}

#[derive(Debug, Clone)]
struct RedisErrorSink {
    cluster_name: String,
}

impl ErrorSink<redis::RedisError> for RedisErrorSink {
    fn sink(&self, err: redis::RedisError) {
        log::error!(
            "cache cluster {}: connection pool error: {:?}",
            self.cluster_name,
            err
        );
    }

    fn boxed_clone(&self) -> Box<dyn ErrorSink<redis::RedisError>> {
        Box::new(self.clone())
    }
}

impl Pool {
    pub(crate) fn new(
        client: redis::Client,
        key_prefix: Option<String>,
        tracer: Tracer,
        min_conns: u32,
        max_conns: u32,
    ) -> anyhow::Result<Self> {
        let conn_info = client.get_connection_info().clone();
        log::debug!(
            "cache pool: creating connection manager - addr: {:?}",
            conn_info.addr()
        );

        let mgr = RedisConnectionManager::new(conn_info)?;

        let cluster_name = key_prefix.clone().unwrap_or_else(|| "default".to_string());
        let mut pool = Bb8Pool::builder()
            .error_sink(Box::new(RedisErrorSink { cluster_name }))
            .max_size(if max_conns > 0 { max_conns } else { 30 })
            .connection_timeout(std::time::Duration::from_secs(10));

        if min_conns > 0 {
            pool = pool.min_idle(Some(min_conns));
        }

        let pool = pool.build_unchecked(mgr);

        Ok(Self {
            backend: Backend::Redis { pool, key_prefix },
            tracer,
        })
    }

    /// Creates a pool backed by an in-memory store.
    pub(crate) fn in_memory(store: Arc<MemoryStore>, tracer: Tracer) -> Self {
        Self {
            backend: Backend::Memory(store),
            tracer,
        }
    }

    /// Gets a connection from the pool (Redis backend only).
    async fn conn(&self) -> Result<bb8::PooledConnection<'_, RedisConnectionManager>> {
        match &self.backend {
            Backend::Redis { pool, .. } => pool.get().await.map_err(|e| match e {
                RunError::User(err) => Error::Redis(err),
                RunError::TimedOut => Error::Pool("connection pool timeout".to_string()),
            }),
            Backend::Memory(_) => Err(Error::Pool(
                "in-memory backend does not use connections".to_string(),
            )),
        }
    }

    /// Returns a prefixed key if a key prefix is configured (Redis backend).
    fn prefixed_key(&self, key: &str) -> String {
        match &self.backend {
            Backend::Redis { key_prefix, .. } => match key_prefix {
                Some(prefix) => format!("{}{}", prefix, key),
                None => key.to_string(),
            },
            Backend::Memory(_) => key.to_string(),
        }
    }

    /// Gets the memory store if using in-memory backend.
    fn memory_store(&self) -> Option<&Arc<MemoryStore>> {
        match &self.backend {
            Backend::Memory(store) => Some(store),
            Backend::Redis { .. } => None,
        }
    }

    // ==================== Basic Operations ====================

    /// Get a value by key.
    pub async fn get(&self, key: &str, source: Option<&Request>) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("get", false, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.get(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Option<Vec<u8>>> = (*conn).get(&key).await;

        match result {
            Ok(value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set a value by key with optional TTL operation.
    pub async fn set(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("set", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.set(&key, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let mut cmd = redis::cmd("SET");
        cmd.arg(&key).arg(value);
        match ttl {
            Some(TtlOp::Keep) => { cmd.arg("KEEPTTL"); }
            Some(TtlOp::SetMs(ms)) => { cmd.arg("PX").arg(ms); }
            Some(TtlOp::Persist) | None => {} // No TTL flags
        }
        let result: RedisResult<()> = cmd.query_async(&mut *conn).await;

        match result {
            Ok(()) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set a value only if the key doesn't exist (SET NX).
    pub async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("setnx", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.set_if_not_exists(&key, value, ttl);
            match &result {
                Ok(set) => {
                    let op_result = if *set {
                        CacheOpResult::Ok
                    } else {
                        CacheOpResult::Conflict
                    };
                    self.trace_end(trace, source, op_result, None);
                }
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<bool> = {
            let mut conn = self.conn().await?;
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("NX");
            match ttl {
                Some(TtlOp::SetMs(ms)) => { cmd.arg("PX").arg(ms); }
                _ => {} // KeepTTL doesn't apply to NX (new key has no TTL to keep)
            }
            cmd.query_async(&mut *conn).await
        };

        match result {
            Ok(set) => {
                let op_result = if set {
                    CacheOpResult::Ok
                } else {
                    CacheOpResult::Conflict
                };
                self.trace_end(trace, source, op_result, None);
                Ok(set)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Replace a value only if the key exists (SET XX).
    pub async fn replace(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("replace", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.replace(&key, value, ttl);
            match &result {
                Ok(replaced) => {
                    let op_result = if *replaced {
                        CacheOpResult::Ok
                    } else {
                        CacheOpResult::NoSuchKey
                    };
                    self.trace_end(trace, source, op_result, None);
                }
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Option<()>> = {
            let mut conn = self.conn().await?;
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("XX");
            match ttl {
                Some(TtlOp::Keep) => { cmd.arg("KEEPTTL"); }
                Some(TtlOp::SetMs(ms)) => { cmd.arg("PX").arg(ms); }
                Some(TtlOp::Persist) | None => {}
            }
            cmd.query_async(&mut *conn).await
        };

        match result {
            Ok(Some(())) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(true)
            }
            Ok(None) => {
                self.trace_end(trace, source, CacheOpResult::NoSuchKey, None);
                Ok(false)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get old value and set new value atomically (SET GET).
    pub async fn get_and_set(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("getset", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.get_and_set(&key, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Option<Vec<u8>>> = {
            let mut conn = self.conn().await?;
            let mut cmd = redis::cmd("SET");
            cmd.arg(&key).arg(value).arg("GET");
            match ttl {
                Some(TtlOp::Keep) => { cmd.arg("KEEPTTL"); }
                Some(TtlOp::SetMs(ms)) => { cmd.arg("PX").arg(ms); }
                Some(TtlOp::Persist) | None => {}
            }
            cmd.query_async(&mut *conn).await
        };

        match result {
            Ok(old_value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(old_value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.get_and_delete(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Option<Vec<u8>>> =
            redis::cmd("GETDEL").arg(&key).query_async(&mut *conn).await;

        match result {
            Ok(value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Delete one or more keys.
    pub async fn delete(&self, keys: &[&str], source: Option<&Request>) -> Result<u64> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("del", true, &key_refs, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.delete(&key_refs);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<u64> = (*conn).del(&prefixed).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.mget(&key_refs);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Vec<Option<Vec<u8>>>> = (*conn).mget(&prefixed).await;

        match result {
            Ok(values) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== String Operations ====================

    /// Append to a string value.
    pub async fn append(&self, key: &str, value: &[u8], ttl: Option<TtlOp>, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("append", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.append(&key, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).append(&key, value).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("APPEND").arg(&key).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("APPEND").arg(&key).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(new_len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(new_len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.get_range(&key, start, end);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<u8>> = self
            .conn()
            .await?
            .getrange(&key, start as isize, end as isize)
            .await;

        match result {
            Ok(value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("setrange", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.set_range(&key, offset, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).setrange(&key, offset as isize, value).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("SETRANGE").arg(&key).arg(offset).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("SETRANGE").arg(&key).arg(offset).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(new_len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(new_len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get string length.
    pub async fn strlen(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("strlen", false, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.strlen(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<i64> = self.conn().await?.strlen(&key).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Numeric Operations ====================

    /// Increment an integer value.
    pub async fn incr_by(&self, key: &str, delta: i64, ttl: Option<TtlOp>, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("incrby", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.incr_by(&key, delta, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).incr(&key, delta).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("INCRBY").arg(&key).arg(delta)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("INCRBY").arg(&key).arg(delta)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(new_val) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(new_val)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Increment a float value.
    pub async fn incr_by_float(
        &self,
        key: &str,
        delta: f64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<f64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("incrbyfloat", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.incr_by_float(&key, delta, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<f64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).incr(&key, delta).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("INCRBYFLOAT").arg(&key).arg(delta)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("INCRBYFLOAT").arg(&key).arg(delta)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(new_val) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(new_val)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lpush", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lpush(&key, values, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).lpush(&key, values).await,
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("LPUSH").arg(&key);
                for v in values { pipe.arg(*v); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("LPUSH").arg(&key);
                for v in values { pipe.arg(*v); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Push values to the right (tail) of a list.
    pub async fn rpush(
        &self,
        key: &str,
        values: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("rpush", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.rpush(&key, values, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).rpush(&key, values).await,
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("RPUSH").arg(&key);
                for v in values { pipe.arg(*v); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("RPUSH").arg(&key);
                for v in values { pipe.arg(*v); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop value from the left (head) of a list.
    pub async fn lpop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lpop", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lpop(&key, count, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Vec<Vec<u8>>> = match ttl {
            None | Some(TtlOp::Keep) => {
                match count.and_then(NonZeroUsize::new) {
                    Some(n) => (*conn).lpop(&key, Some(n)).await,
                    None => {
                        let single: RedisResult<Option<Vec<u8>>> = (*conn).lpop(&key, None).await;
                        single.map(|v| v.into_iter().collect())
                    }
                }
            }
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("LPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("LPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(values) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop value from the right (tail) of a list.
    pub async fn rpop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("rpop", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.rpop(&key, count, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Vec<Vec<u8>>> = match ttl {
            None | Some(TtlOp::Keep) => {
                match count.and_then(NonZeroUsize::new) {
                    Some(n) => (*conn).rpop(&key, Some(n)).await,
                    None => {
                        let single: RedisResult<Option<Vec<u8>>> = (*conn).rpop(&key, None).await;
                        single.map(|v| v.into_iter().collect())
                    }
                }
            }
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("RPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("RPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(values) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lindex(&key, index);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Option<Vec<u8>>> =
            self.conn().await?.lindex(&key, index as isize).await;

        match result {
            Ok(value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lset", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lset(&key, index, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<()> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).lset(&key, index as isize, value).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LSET").arg(&key).arg(index).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LSET").arg(&key).arg(index).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(()) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lrange(&key, start, stop);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<Vec<u8>>> = self
            .conn()
            .await?
            .lrange(&key, start as isize, stop as isize)
            .await;

        match result {
            Ok(values) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(values)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<()> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("ltrim", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.ltrim(&key, start, stop, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<()> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).ltrim(&key, start as isize, stop as isize).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LTRIM").arg(&key).arg(start).arg(stop)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LTRIM").arg(&key).arg(start).arg(stop)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(()) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(())
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("linsert", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.linsert_before(&key, pivot, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => {
                redis::cmd("LINSERT").arg(&key).arg("BEFORE").arg(pivot).arg(value)
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LINSERT").arg(&key).arg("BEFORE").arg(pivot).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LINSERT").arg(&key).arg("BEFORE").arg(pivot).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(pos) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(pos)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("linsert", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.linsert_after(&key, pivot, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => {
                redis::cmd("LINSERT").arg(&key).arg("AFTER").arg(pivot).arg(value)
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LINSERT").arg(&key).arg("AFTER").arg(pivot).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LINSERT").arg(&key).arg("AFTER").arg(pivot).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(pos) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(pos)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("lrem", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lrem(&key, count, value, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).lrem(&key, count as isize, value).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LREM").arg(&key).arg(count).arg(value)
                    .cmd("PEXPIRE").arg(&key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LREM").arg(&key).arg(count).arg(value)
                    .cmd("PERSIST").arg(&key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(removed) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(removed)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<Option<Vec<u8>>> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        let trace = self.trace_start("lmove", true, &[&src_key, &dst_key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.lmove(&src_key, &dst_key, src_dir, dst_dir, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Option<Vec<u8>>> = match ttl {
            None | Some(TtlOp::Keep) => {
                redis::cmd("LMOVE").arg(&src_key).arg(&dst_key)
                    .arg(src_dir.as_str()).arg(dst_dir.as_str())
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("LMOVE").arg(&src_key).arg(&dst_key).arg(src_dir.as_str()).arg(dst_dir.as_str())
                    .cmd("PEXPIRE").arg(&dst_key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("LMOVE").arg(&src_key).arg(&dst_key).arg(src_dir.as_str()).arg(dst_dir.as_str())
                    .cmd("PERSIST").arg(&dst_key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(value) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(value)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get list length.
    pub async fn llen(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("llen", false, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.llen(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<i64> = self.conn().await?.llen(&key).await;

        match result {
            Ok(len) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(len)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("sadd", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sadd(&key, members, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).sadd(&key, members).await,
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SADD").arg(&key);
                for m in members { pipe.arg(*m); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SADD").arg(&key);
                for m in members { pipe.arg(*m); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(added) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(added)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Remove members from a set.
    pub async fn srem(
        &self,
        key: &str,
        members: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("srem", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.srem(&key, members, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).srem(&key, members).await,
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SREM").arg(&key);
                for m in members { pipe.arg(*m); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SREM").arg(&key);
                for m in members { pipe.arg(*m); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(removed) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(removed)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sismember(&key, member);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<bool> = self.conn().await?.sismember(&key, member).await;

        match result {
            Ok(is_member) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(is_member)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Pop random members from a set.
    pub async fn spop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("spop", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.spop(&key, count, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Vec<Vec<u8>>> = match ttl {
            None | Some(TtlOp::Keep) => {
                match count {
                    Some(n) => {
                        redis::cmd("SPOP").arg(&key).arg(n).query_async(&mut *conn).await
                    }
                    None => {
                        let single: RedisResult<Option<Vec<u8>>> = (*conn).spop(&key).await;
                        single.map(|v| v.into_iter().collect())
                    }
                }
            }
            Some(TtlOp::SetMs(ms)) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PEXPIRE").arg(&key).arg(ms).ignore();
                pipe.query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                let mut pipe = redis::pipe();
                pipe.atomic().cmd("SPOP").arg(&key);
                if let Some(n) = count { pipe.arg(n); }
                pipe.cmd("PERSIST").arg(&key).ignore();
                pipe.query_async(&mut *conn).await
            }
        };

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.srandmember(&key, count);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<Vec<Vec<u8>>> = redis::cmd("SRANDMEMBER")
            .arg(&key)
            .arg(count)
            .query_async(&mut *conn)
            .await;

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get all members of a set.
    pub async fn smembers(&self, key: &str, source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("smembers", false, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.smembers(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.smembers(&key).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Get set cardinality.
    pub async fn scard(&self, key: &str, source: Option<&Request>) -> Result<i64> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("scard", false, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.scard(&key);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<i64> = self.conn().await?.scard(&key).await;

        match result {
            Ok(count) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set difference.
    pub async fn sdiff(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sdiff", false, &key_refs, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sdiff(&key_refs);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sdiff(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set difference.
    pub async fn sdiffstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        let trace = self.trace_start("sdiffstore", true, &all_keys, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sdiffstore(&dest_key, &key_refs, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).sdiffstore(&dest_key, &prefixed).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("SDIFFSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PEXPIRE").arg(&dest_key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("SDIFFSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PERSIST").arg(&dest_key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(count) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set intersection.
    pub async fn sinter(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sinter", false, &key_refs, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sinter(&key_refs);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sinter(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set intersection.
    pub async fn sinterstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        let trace = self.trace_start("sinterstore", true, &all_keys, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sinterstore(&dest_key, &key_refs, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).sinterstore(&dest_key, &prefixed).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("SINTERSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PEXPIRE").arg(&dest_key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("SINTERSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PERSIST").arg(&dest_key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(count) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Set union.
    pub async fn sunion(&self, keys: &[&str], source: Option<&Request>) -> Result<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        let trace = self.trace_start("sunion", false, &key_refs, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sunion(&key_refs);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let result: RedisResult<Vec<Vec<u8>>> = self.conn().await?.sunion(&prefixed).await;

        match result {
            Ok(members) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(members)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    /// Store set union.
    pub async fn sunionstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        let trace = self.trace_start("sunionstore", true, &all_keys, source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.sunionstore(&dest_key, &key_refs, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<i64> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).sunionstore(&dest_key, &prefixed).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("SUNIONSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PEXPIRE").arg(&dest_key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("SUNIONSTORE").arg(&dest_key).arg(&prefixed)
                    .cmd("PERSIST").arg(&dest_key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(count) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(count)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> Result<bool> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        let trace = self.trace_start("smove", true, &[&src_key, &dst_key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.smove(&src_key, &dst_key, member, ttl);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<bool> = match ttl {
            None | Some(TtlOp::Keep) => (*conn).smove(&src_key, &dst_key, member).await,
            Some(TtlOp::SetMs(ms)) => {
                redis::pipe().atomic()
                    .cmd("SMOVE").arg(&src_key).arg(&dst_key).arg(member)
                    .cmd("PEXPIRE").arg(&dst_key).arg(ms).ignore()
                    .query_async(&mut *conn).await
            }
            Some(TtlOp::Persist) => {
                redis::pipe().atomic()
                    .cmd("SMOVE").arg(&src_key).arg(&dst_key).arg(member)
                    .cmd("PERSIST").arg(&dst_key).ignore()
                    .query_async(&mut *conn).await
            }
        };

        match result {
            Ok(moved) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(moved)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
                Err(e.into())
            }
        }
    }

    // ==================== Expiry Operations ====================

    /// Set expiry on a key in milliseconds.
    pub async fn pexpire(&self, key: &str, ttl_ms: u64, source: Option<&Request>) -> Result<bool> {
        let key = self.prefixed_key(key);
        let trace = self.trace_start("pexpire", true, &[&key], source);

        // Use in-memory backend if available
        if let Some(store) = self.memory_store() {
            let result = store.pexpire(&key, ttl_ms);
            match &result {
                Ok(_) => self.trace_end(trace, source, CacheOpResult::Ok, None),
                Err(_) => self.trace_end_err(trace, source),
            }
            return result;
        }

        let mut conn = self.conn().await?;
        let result: RedisResult<bool> = redis::cmd("PEXPIRE")
            .arg(&key)
            .arg(ttl_ms as i64)
            .query_async(&mut *conn)
            .await;

        match result {
            Ok(set) => {
                self.trace_end(trace, source, CacheOpResult::Ok, None);
                Ok(set)
            }
            Err(e) => {
                self.trace_end(trace, source, CacheOpResult::Err, Some(&e));
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
    ) -> Option<TraceEventId> {
        let source = source?;
        self.tracer.cache_call_start(CacheCallStartData {
            source,
            operation,
            is_write,
            keys,
        })
    }

    fn trace_end(
        &self,
        start_id: Option<TraceEventId>,
        source: Option<&Request>,
        result: CacheOpResult,
        err: Option<&redis::RedisError>,
    ) {
        let Some(source) = source else { return };
        self.tracer.cache_call_end(CacheCallEndData {
            start_id,
            source,
            result,
            error: err,
        });
    }

    /// Trace end helper for in-memory errors (no Redis error available).
    fn trace_end_err(&self, start_id: Option<TraceEventId>, source: Option<&Request>) {
        let Some(source) = source else { return };
        self.tracer
            .cache_call_end(CacheCallEndData::<redis::RedisError> {
                start_id,
                source,
                result: CacheOpResult::Err,
                error: None,
            });
    }
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
