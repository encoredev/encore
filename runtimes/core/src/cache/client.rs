use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use bb8::{ErrorSink, Pool as Bb8Pool, RunError};
use bb8_redis::redis::{self as redis, RedisResult};
use bb8_redis::RedisConnectionManager;
use redis::{FromRedisValue, SetExpiry, ToSingleRedisArg};

use crate::cache::error::{Error, OpResult, Result};
use crate::cache::memcluster::MemoryStore;
use crate::cache::tracer::CacheTracer;
use crate::model::Request;
use crate::trace::Tracer;

/// TTL operation for cache write commands.
#[derive(Debug, Clone, Copy)]
pub enum TtlOp {
    /// Preserve the existing TTL (KEEPTTL for SET; no-op for others).
    Keep,
    /// Set TTL in milliseconds (PX for SET; atomic PEXPIREAT for others).
    SetMs(u64),
    /// Remove TTL / never expire (no TTL flags for SET; atomic PERSIST for others).
    Persist,
}

/// Direction for list operations.
#[derive(Debug, Clone, Copy)]
pub enum ListDirection {
    Left,
    Right,
}

impl From<ListDirection> for redis::Direction {
    fn from(value: ListDirection) -> Self {
        match value {
            ListDirection::Left => Self::Left,
            ListDirection::Right => Self::Right,
        }
    }
}

enum LRemOp {
    All,
    First(i64),
    Last(i64),
}

impl LRemOp {
    fn name(&self) -> &'static str {
        match self {
            LRemOp::All => "remove all",
            LRemOp::First(_) => "remove first",
            LRemOp::Last(_) => "remove last",
        }
    }
}

/// Converts a relative TTL in milliseconds to an absolute PEXPIREAT timestamp.
fn expire_at_ms(relative_ms: u64) -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_millis() as u64
        + relative_ms
}

/// Builds an expiration command for a key based on the TTL operation.
fn exp_cmd(key: &str, ttl: Option<TtlOp>) -> Option<redis::Cmd> {
    match ttl? {
        TtlOp::Keep => None,
        TtlOp::SetMs(ms) => Some(
            redis::cmd("PEXPIREAT")
                .arg(key)
                .arg(expire_at_ms(ms))
                .take(),
        ),
        TtlOp::Persist => Some(redis::cmd("PERSIST").arg(key).take()),
    }
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

struct RedisBackend {
    pool: Bb8Pool<RedisConnectionManager>,
}

impl RedisBackend {
    fn new(
        client: redis::Client,
        cluster_name: String,
        min_conns: u32,
        max_conns: u32,
    ) -> anyhow::Result<Self> {
        let conn_info = client.get_connection_info().clone();
        let mgr = RedisConnectionManager::new(conn_info)?;

        let mut pool = Bb8Pool::builder()
            .error_sink(Box::new(RedisErrorSink { cluster_name }))
            .max_size(if max_conns > 0 {
                max_conns
            } else {
                (std::thread::available_parallelism()
                    .map(|n| n.get())
                    .unwrap_or(4)
                    * 10) as u32
            })
            .connection_timeout(std::time::Duration::from_secs(10));

        if min_conns > 0 {
            pool = pool.min_idle(Some(min_conns));
        }

        let pool = pool.build_unchecked(mgr);
        Ok(Self { pool })
    }

    async fn conn(&self) -> Result<bb8::PooledConnection<'_, RedisConnectionManager>> {
        self.pool.get().await.map_err(|e| match e {
            RunError::User(err) => Error::Redis(err),
            RunError::TimedOut => Error::PoolTimeout,
        })
    }

    /// Execute a single Redis command, mapping nil to Error::Miss.
    async fn query<T, F>(&self, f: F) -> Result<T>
    where
        F: FnOnce(&mut redis::Pipeline) -> &mut redis::Pipeline,
        T: FromRedisValue,
    {
        let mut pipe = redis::pipe();
        let mut conn = self.conn().await?;
        let res: RedisResult<(Option<T>,)> = f(&mut pipe).query_async(&mut *conn).await;
        match res?.0 {
            None => Err(Error::Miss),
            Some(v) => Ok(v),
        }
    }

    /// Execute a Redis command atomically with TTL management for one key.
    async fn query_with_ttl<T, F>(&self, key: &str, ttl: Option<TtlOp>, f: F) -> Result<T>
    where
        F: FnOnce(&mut redis::Pipeline) -> &mut redis::Pipeline,
        T: FromRedisValue,
    {
        let exp = exp_cmd(key, ttl);
        if exp.is_none() {
            return self.query(f).await;
        }

        let mut pipe = redis::pipe();
        let mut conn = self.conn().await?;
        pipe.atomic();
        f(&mut pipe);
        if let Some(exp) = exp {
            pipe.add_command(exp).ignore();
        }
        let res: RedisResult<(Option<T>,)> = pipe.query_async(&mut *conn).await;
        match res?.0 {
            None => Err(Error::Miss),
            Some(v) => Ok(v),
        }
    }

    /// Execute a Redis command atomically with TTL management for two keys.
    async fn query_with_ttl2<T, F>(&self, keys: (&str, &str), ttl: Option<TtlOp>, f: F) -> Result<T>
    where
        F: FnOnce(&mut redis::Pipeline) -> &mut redis::Pipeline,
        T: FromRedisValue,
    {
        let exp_a = exp_cmd(keys.0, ttl);
        let exp_b = exp_cmd(keys.1, ttl);
        if exp_a.is_none() && exp_b.is_none() {
            return self.query(f).await;
        }

        let mut pipe = redis::pipe();
        let mut conn = self.conn().await?;
        pipe.atomic();
        f(&mut pipe);
        if let Some(exp) = exp_a {
            pipe.add_command(exp).ignore();
        }
        if let Some(exp) = exp_b {
            pipe.add_command(exp).ignore();
        }
        let res: RedisResult<(Option<T>,)> = pipe.query_async(&mut *conn).await;
        match res?.0 {
            None => Err(Error::Miss),
            Some(v) => Ok(v),
        }
    }

    async fn _set<T, V>(
        &self,
        key: &str,
        ttl: Option<TtlOp>,
        set_cond: Option<redis::ExistenceCheck>,
        get: bool,
        value: V,
    ) -> Result<T>
    where
        V: ToSingleRedisArg + Sync + Send,
        T: FromRedisValue,
    {
        let mut opts = redis::SetOptions::default().get(get);

        if let Some(set_cond) = set_cond {
            opts = opts.conditional_set(set_cond);
        }

        if let Some(set_exp) = ttl.and_then(|t| match t {
            TtlOp::Keep => Some(SetExpiry::KEEPTTL),
            TtlOp::SetMs(ms) => Some(SetExpiry::PX(ms)),
            TtlOp::Persist => None,
        }) {
            opts = opts.with_expiration(set_exp);
        }

        match self.query(|pipe| pipe.set_options(key, value, opts)).await {
            Err(Error::Miss) if matches!(set_cond, Some(redis::ExistenceCheck::NX)) => {
                Err(Error::KeyExist)
            }
            other => other,
        }
    }

    async fn get(&self, key: &str) -> Result<Vec<u8>> {
        self.query(|pipe| pipe.get(key)).await
    }

    async fn set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self._set(key, ttl, None, false, value).await
    }

    async fn set_if_not_exists(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self._set(key, ttl, Some(redis::ExistenceCheck::NX), false, value)
            .await
    }

    async fn replace(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self._set(key, ttl, Some(redis::ExistenceCheck::XX), false, value)
            .await
    }

    async fn get_and_set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self._set(key, ttl, None, true, value).await
    }

    async fn get_and_delete(&self, key: &str) -> Result<Vec<u8>> {
        self.query(|pipe| pipe.get_del(key)).await
    }

    async fn delete(&self, keys: &[&str]) -> Result<u64> {
        self.query(|pipe| pipe.del(keys)).await
    }

    async fn mget(&self, keys: &[&str]) -> Result<Vec<Option<Vec<u8>>>> {
        self.query(|pipe| pipe.mget(keys)).await
    }

    async fn append(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.append(key, value))
            .await
    }

    async fn get_range(&self, key: &str, start: i64, end: i64) -> Result<Vec<u8>> {
        self.query(|pipe| pipe.getrange(key, start as isize, end as isize))
            .await
    }

    async fn set_range(
        &self,
        key: &str,
        offset: i64,
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.setrange(key, offset as isize, value))
            .await
    }

    async fn strlen(&self, key: &str) -> Result<i64> {
        self.query(|pipe| pipe.strlen(key)).await
    }

    async fn incr_by(&self, key: &str, delta: i64, ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.incr(key, delta))
            .await
    }

    async fn incr_by_float(&self, key: &str, delta: f64, ttl: Option<TtlOp>) -> Result<f64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.incr(key, delta))
            .await
    }

    async fn lpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.lpush(key, values))
            .await
    }

    async fn rpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.rpush(key, values))
            .await
    }

    async fn lpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.query_with_ttl(key, ttl, |pipe| pipe.lpop(key, None))
            .await
    }

    async fn rpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.query_with_ttl(key, ttl, |pipe| pipe.rpop(key, None))
            .await
    }

    async fn lindex(&self, key: &str, index: i64) -> Result<Vec<u8>> {
        self.query(|pipe| pipe.lindex(key, index as isize)).await
    }

    async fn lset(&self, key: &str, index: i64, value: &[u8], _ttl: Option<TtlOp>) -> Result<()> {
        self.query(|pipe| pipe.lset(key, index as isize, value))
            .await
    }

    async fn lrange(&self, key: &str, start: i64, stop: i64) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.lrange(key, start as isize, stop as isize))
            .await
    }

    async fn ltrim(&self, key: &str, start: i64, stop: i64, _ttl: Option<TtlOp>) -> Result<()> {
        self.query(|pipe| pipe.ltrim(key, start as isize, stop as isize))
            .await
    }

    async fn linsert_before(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.linsert_before(key, pivot, value))
            .await
    }

    async fn linsert_after(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.linsert_after(key, pivot, value))
            .await
    }

    async fn lrem(&self, key: &str, count: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.lrem(key, count as isize, value))
            .await
    }

    async fn lmove(
        &self,
        src: &str,
        dst: &str,
        src_dir: ListDirection,
        dst_dir: ListDirection,
        ttl: Option<TtlOp>,
    ) -> Result<Vec<u8>> {
        self.query_with_ttl2((src, dst), ttl, |pipe| {
            pipe.lmove(src, dst, src_dir.into(), dst_dir.into())
        })
        .await
    }

    async fn llen(&self, key: &str) -> Result<i64> {
        self.query(|pipe| pipe.llen(key)).await
    }

    async fn sadd(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.sadd(key, members))
            .await
    }

    async fn srem(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(key, ttl, |pipe| pipe.srem(key, members))
            .await
    }

    async fn sismember(&self, key: &str, member: &[u8]) -> Result<bool> {
        self.query(|pipe| pipe.sismember(key, member)).await
    }

    async fn spop_one(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.query_with_ttl(key, ttl, |pipe| pipe.spop(key)).await
    }

    async fn spop(&self, key: &str, count: usize, ttl: Option<TtlOp>) -> Result<Vec<Vec<u8>>> {
        self.query_with_ttl(key, ttl, |pipe| pipe.spop(key).arg(count))
            .await
    }

    async fn srandmember(&self, key: &str) -> Result<Vec<u8>> {
        self.query(|pipe| pipe.srandmember(key)).await
    }

    async fn srandmember_multiple(&self, key: &str, count: i64) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.srandmember_multiple(key, count as isize))
            .await
    }

    async fn smembers(&self, key: &str) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.smembers(key)).await
    }

    async fn scard(&self, key: &str) -> Result<i64> {
        self.query(|pipe| pipe.scard(key)).await
    }

    async fn sdiff(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.sdiff(keys)).await
    }

    async fn sdiffstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(dest, ttl, |pipe| pipe.sdiffstore(dest, keys))
            .await
    }

    async fn sinter(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.sinter(keys)).await
    }

    async fn sinterstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(dest, ttl, |pipe| pipe.sinterstore(dest, keys))
            .await
    }

    async fn sunion(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.query(|pipe| pipe.sunion(keys)).await
    }

    async fn sunionstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.query_with_ttl(dest, ttl, |pipe| pipe.sunionstore(dest, keys))
            .await
    }

    async fn smove(&self, src: &str, dst: &str, member: &[u8], ttl: Option<TtlOp>) -> Result<bool> {
        self.query_with_ttl2((src, dst), ttl, |pipe| pipe.smove(src, dst, member))
            .await
    }
}

struct MemoryBackend {
    store: Arc<MemoryStore>,
}

impl MemoryBackend {
    fn get(&self, key: &str) -> Result<Vec<u8>> {
        self.store.get(key)
    }

    fn set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.store.set(key, value, ttl)
    }

    fn set_if_not_exists(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.store.set_if_not_exists(key, value, ttl)
    }

    fn replace(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.store.replace(key, value, ttl)
    }

    fn get_and_set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.store.get_and_set(key, value, ttl)
    }

    fn get_and_delete(&self, key: &str) -> Result<Vec<u8>> {
        self.store.get_and_delete(key)
    }

    fn delete(&self, keys: &[&str]) -> Result<u64> {
        self.store.delete(keys)
    }

    fn mget(&self, keys: &[&str]) -> Result<Vec<Option<Vec<u8>>>> {
        self.store.mget(keys)
    }

    fn append(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.append(key, value, ttl)
    }

    fn get_range(&self, key: &str, start: i64, end: i64) -> Result<Vec<u8>> {
        self.store.get_range(key, start, end)
    }

    fn set_range(&self, key: &str, offset: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.set_range(key, offset, value, ttl)
    }

    fn strlen(&self, key: &str) -> Result<i64> {
        self.store.strlen(key)
    }

    fn incr_by(&self, key: &str, delta: i64, ttl: Option<TtlOp>) -> Result<i64> {
        self.store.incr_by(key, delta, ttl)
    }

    fn incr_by_float(&self, key: &str, delta: f64, ttl: Option<TtlOp>) -> Result<f64> {
        self.store.incr_by_float(key, delta, ttl)
    }

    fn lpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.lpush(key, values, ttl)
    }

    fn rpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.rpush(key, values, ttl)
    }

    fn lpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.store.lpop(key, ttl)
    }

    fn rpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.store.rpop(key, ttl)
    }

    fn lindex(&self, key: &str, index: i64) -> Result<Vec<u8>> {
        self.store.lindex(key, index)
    }

    fn lset(&self, key: &str, index: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.store.lset(key, index, value, ttl)
    }

    fn lrange(&self, key: &str, start: i64, stop: i64) -> Result<Vec<Vec<u8>>> {
        self.store.lrange(key, start, stop)
    }

    fn ltrim(&self, key: &str, start: i64, stop: i64, ttl: Option<TtlOp>) -> Result<()> {
        self.store.ltrim(key, start, stop, ttl)
    }

    fn linsert_before(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.store.linsert_before(key, pivot, value, ttl)
    }

    fn linsert_after(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.store.linsert_after(key, pivot, value, ttl)
    }

    fn lrem(&self, key: &str, count: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.lrem(key, count, value, ttl)
    }

    fn lmove(
        &self,
        src: &str,
        dst: &str,
        src_dir: ListDirection,
        dst_dir: ListDirection,
        ttl: Option<TtlOp>,
    ) -> Result<Vec<u8>> {
        self.store.lmove(src, dst, src_dir, dst_dir, ttl)
    }

    fn llen(&self, key: &str) -> Result<i64> {
        self.store.llen(key)
    }

    fn sadd(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.sadd(key, members, ttl)
    }

    fn srem(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.srem(key, members, ttl)
    }

    fn sismember(&self, key: &str, member: &[u8]) -> Result<bool> {
        self.store.sismember(key, member)
    }

    fn spop_one(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>> {
        self.store
            .spop(key, None, ttl)
            .and_then(|m| m.into_iter().next().ok_or(Error::Miss))
    }

    fn spop(&self, key: &str, count: usize, ttl: Option<TtlOp>) -> Result<Vec<Vec<u8>>> {
        self.store.spop(key, Some(count), ttl)
    }

    fn srandmember(&self, key: &str) -> Result<Vec<u8>> {
        self.store
            .srandmember(key, 1)
            .and_then(|m| m.into_iter().next().ok_or(Error::Miss))
    }

    fn srandmember_multiple(&self, key: &str, count: i64) -> Result<Vec<Vec<u8>>> {
        self.store.srandmember(key, count)
    }

    fn smembers(&self, key: &str) -> Result<Vec<Vec<u8>>> {
        self.store.smembers(key)
    }

    fn scard(&self, key: &str) -> Result<i64> {
        self.store.scard(key)
    }

    fn sdiff(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.store.sdiff(keys)
    }

    fn sdiffstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.sdiffstore(dest, keys, ttl)
    }

    fn sinter(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.store.sinter(keys)
    }

    fn sinterstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.sinterstore(dest, keys, ttl)
    }

    fn sunion(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.store.sunion(keys)
    }

    fn sunionstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.store.sunionstore(dest, keys, ttl)
    }

    fn smove(&self, src: &str, dst: &str, member: &[u8], ttl: Option<TtlOp>) -> Result<bool> {
        self.store.smove(src, dst, member, ttl)
    }
}

enum Backend {
    Redis(RedisBackend),
    Memory(MemoryBackend),
}

/// Generates async dispatch methods on `Backend` that delegate to the
/// appropriate backend variant.
macro_rules! dispatch {
    ($(async fn $name:ident(&self $(, $arg:ident: $ty:ty)*) -> $ret:ty;)*) => {
        $(
            async fn $name(&self $(, $arg: $ty)*) -> $ret {
                match self {
                    Backend::Redis(r) => r.$name($($arg),*).await,
                    Backend::Memory(m) => m.$name($($arg),*),
                }
            }
        )*
    };
}

impl Backend {
    dispatch! {
        // String operations
        async fn get(&self, key: &str) -> Result<Vec<u8>>;
        async fn set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()>;
        async fn set_if_not_exists(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()>;
        async fn replace(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()>;
        async fn get_and_set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<Vec<u8>>;
        async fn get_and_delete(&self, key: &str) -> Result<Vec<u8>>;
        async fn delete(&self, keys: &[&str]) -> Result<u64>;
        async fn mget(&self, keys: &[&str]) -> Result<Vec<Option<Vec<u8>>>>;
        async fn append(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<i64>;
        async fn get_range(&self, key: &str, start: i64, end: i64) -> Result<Vec<u8>>;
        async fn set_range(&self, key: &str, offset: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64>;
        async fn strlen(&self, key: &str) -> Result<i64>;
        async fn incr_by(&self, key: &str, delta: i64, ttl: Option<TtlOp>) -> Result<i64>;
        async fn incr_by_float(&self, key: &str, delta: f64, ttl: Option<TtlOp>) -> Result<f64>;
        // List operations
        async fn lpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64>;
        async fn rpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64>;
        async fn lpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>>;
        async fn rpop(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>>;
        async fn lindex(&self, key: &str, index: i64) -> Result<Vec<u8>>;
        async fn lset(&self, key: &str, index: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<()>;
        async fn lrange(&self, key: &str, start: i64, stop: i64) -> Result<Vec<Vec<u8>>>;
        async fn ltrim(&self, key: &str, start: i64, stop: i64, ttl: Option<TtlOp>) -> Result<()>;
        async fn linsert_before(&self, key: &str, pivot: &[u8], value: &[u8], ttl: Option<TtlOp>) -> Result<i64>;
        async fn linsert_after(&self, key: &str, pivot: &[u8], value: &[u8], ttl: Option<TtlOp>) -> Result<i64>;
        async fn lrem(&self, key: &str, count: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64>;
        async fn lmove(&self, src: &str, dst: &str, src_dir: ListDirection, dst_dir: ListDirection, ttl: Option<TtlOp>) -> Result<Vec<u8>>;
        async fn llen(&self, key: &str) -> Result<i64>;
        // Set operations
        async fn sadd(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64>;
        async fn srem(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64>;
        async fn sismember(&self, key: &str, member: &[u8]) -> Result<bool>;
        async fn spop_one(&self, key: &str, ttl: Option<TtlOp>) -> Result<Vec<u8>>;
        async fn spop(&self, key: &str, count: usize, ttl: Option<TtlOp>) -> Result<Vec<Vec<u8>>>;
        async fn srandmember(&self, key: &str) -> Result<Vec<u8>>;
        async fn srandmember_multiple(&self, key: &str, count: i64) -> Result<Vec<Vec<u8>>>;
        async fn smembers(&self, key: &str) -> Result<Vec<Vec<u8>>>;
        async fn scard(&self, key: &str) -> Result<i64>;
        async fn sdiff(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>>;
        async fn sdiffstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64>;
        async fn sinter(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>>;
        async fn sinterstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64>;
        async fn sunion(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>>;
        async fn sunionstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64>;
        async fn smove(&self, src: &str, dst: &str, member: &[u8], ttl: Option<TtlOp>) -> Result<bool>;
    }
}

/// A cache client for a Redis-compatible cluster.
/// Handles key prefixing, tracing, and dispatching to the appropriate backend.
pub struct Client {
    backend: Backend,
    tracer: CacheTracer,
    key_prefix: Option<String>,
}

impl Client {
    pub(crate) fn new(
        client: redis::Client,
        key_prefix: Option<String>,
        tracer: Tracer,
        min_conns: u32,
        max_conns: u32,
    ) -> anyhow::Result<Self> {
        let cluster_name = key_prefix.clone().unwrap_or_else(|| "default".to_string());
        let backend = RedisBackend::new(client, cluster_name, min_conns, max_conns)?;
        Ok(Self {
            backend: Backend::Redis(backend),
            tracer: CacheTracer::new(tracer),
            key_prefix,
        })
    }

    /// Creates a client backed by an in-memory store.
    pub(crate) fn in_memory(store: Arc<MemoryStore>, tracer: Tracer) -> Self {
        Self {
            backend: Backend::Memory(MemoryBackend { store }),
            tracer: CacheTracer::new(tracer),
            key_prefix: None,
        }
    }

    fn prefixed_key(&self, key: &str) -> String {
        match &self.key_prefix {
            Some(prefix) => format!("{}{}", prefix, key),
            None => key.to_string(),
        }
    }

    /// Get a value by key.
    pub async fn get(&self, key: &str, source: Option<&Request>) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "get", false, &[&key], async || {
                self.backend.get(&key).await
            })
            .await
    }

    /// Set a value by key with optional TTL operation.
    pub async fn set(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<()> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set", true, &[&key], async || {
                self.backend.set(&key, value, ttl).await
            })
            .await
    }

    /// Set a value only if the key doesn't exist (SET NX).
    pub async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<()> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set if not exists", true, &[&key], async || {
                self.backend.set_if_not_exists(&key, value, ttl).await
            })
            .await
    }

    /// Replace a value only if the key exists (SET XX).
    pub async fn replace(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<()> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "replace", true, &[&key], async || {
                self.backend.replace(&key, value, ttl).await
            })
            .await
    }

    /// Get old value and set new value atomically (SET GET).
    pub async fn get_and_set(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "get and set", true, &[&key], async || {
                self.backend.get_and_set(&key, value, ttl).await
            })
            .await
    }

    /// Get value and delete key atomically (GETDEL).
    pub async fn get_and_delete(&self, key: &str, source: Option<&Request>) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "get and delete", true, &[&key], async || {
                self.backend.get_and_delete(&key).await
            })
            .await
    }

    /// Delete one or more keys.
    pub async fn delete(&self, keys: &[&str], source: Option<&Request>) -> OpResult<u64> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        self.tracer
            .trace(source, "delete", true, &key_refs, async || {
                self.backend.delete(&key_refs).await
            })
            .await
    }

    /// Get multiple values (MGET).
    pub async fn mget(
        &self,
        keys: &[&str],
        source: Option<&Request>,
    ) -> OpResult<Vec<Option<Vec<u8>>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        self.tracer
            .trace(source, "multi get", false, &key_refs, async || {
                self.backend.mget(&key_refs).await
            })
            .await
    }

    /// Append to a string value.
    pub async fn append(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "append", true, &[&key], async || {
                self.backend.append(&key, value, ttl).await
            })
            .await
    }

    /// Get a substring of a string value.
    pub async fn get_range(
        &self,
        key: &str,
        start: i64,
        end: i64,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "get range", false, &[&key], async || {
                self.backend.get_range(&key, start, end).await
            })
            .await
    }

    /// Set a substring at a specific offset.
    pub async fn set_range(
        &self,
        key: &str,
        offset: i64,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set range", true, &[&key], async || {
                self.backend.set_range(&key, offset, value, ttl).await
            })
            .await
    }

    /// Get string length.
    pub async fn strlen(&self, key: &str, source: Option<&Request>) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "len", false, &[&key], async || {
                self.backend.strlen(&key).await
            })
            .await
    }

    /// Increment an integer value.
    pub async fn incr_by(
        &self,
        key: &str,
        delta: i64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "increment", true, &[&key], async || {
                self.backend.incr_by(&key, delta, ttl).await
            })
            .await
    }

    /// Decrement an integer value.
    pub async fn decr_by(
        &self,
        key: &str,
        delta: i64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "decrement", true, &[&key], async || {
                self.backend.incr_by(&key, -delta, ttl).await
            })
            .await
    }

    /// Increment a float value.
    pub async fn incr_by_float(
        &self,
        key: &str,
        delta: f64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<f64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "increment", true, &[&key], async || {
                self.backend.incr_by_float(&key, delta, ttl).await
            })
            .await
    }

    /// Decrement a float value.
    pub async fn decr_by_float(
        &self,
        key: &str,
        delta: f64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<f64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "decrement", true, &[&key], async || {
                self.backend.incr_by_float(&key, -delta, ttl).await
            })
            .await
    }

    /// Push values to the left (head) of a list.
    pub async fn lpush(
        &self,
        key: &str,
        values: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "push left", true, &[&key], async || {
                self.backend.lpush(&key, values, ttl).await
            })
            .await
    }

    /// Push values to the right (tail) of a list.
    pub async fn rpush(
        &self,
        key: &str,
        values: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "push right", true, &[&key], async || {
                self.backend.rpush(&key, values, ttl).await
            })
            .await
    }

    /// Pop value from the left (head) of a list.
    pub async fn lpop(
        &self,
        key: &str,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "pop left", true, &[&key], async || {
                self.backend.lpop(&key, ttl).await
            })
            .await
    }

    /// Pop value from the right (tail) of a list.
    pub async fn rpop(
        &self,
        key: &str,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "pop right", true, &[&key], async || {
                self.backend.rpop(&key, ttl).await
            })
            .await
    }

    /// Get element at index from a list.
    pub async fn lindex(
        &self,
        key: &str,
        index: i64,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "list get", false, &[&key], async || {
                self.backend.lindex(&key, index).await
            })
            .await
    }

    /// Set element at index in a list.
    pub async fn lset(
        &self,
        key: &str,
        index: i64,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<()> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "list set", true, &[&key], async || {
                self.backend.lset(&key, index, value, ttl).await
            })
            .await
    }

    /// Get a range of elements from a list.
    pub async fn lrange(
        &self,
        key: &str,
        start: i64,
        stop: i64,
        source: Option<&Request>,
    ) -> OpResult<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "get range", false, &[&key], async || {
                self.backend.lrange(&key, start, stop).await
            })
            .await
    }

    /// Get all elements of a list. Equivalent to LRANGE 0 -1 but traced as "items".
    pub async fn litems(&self, key: &str, source: Option<&Request>) -> OpResult<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "items", false, &[&key], async || {
                self.backend.lrange(&key, 0, -1).await
            })
            .await
    }

    /// Trim list to specified range.
    pub async fn ltrim(
        &self,
        key: &str,
        start: i64,
        stop: i64,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<()> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "list trim", true, &[&key], async || {
                self.backend.ltrim(&key, start, stop, ttl).await
            })
            .await
    }

    /// Insert element before pivot in list.
    pub async fn linsert_before(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "insert before", true, &[&key], async || {
                let result = self.backend.linsert_before(&key, pivot, value, ttl).await;
                match result {
                    Ok(n) if n < 0 => Err(Error::Miss),
                    other => other,
                }
            })
            .await
    }

    /// Insert element after pivot in list.
    pub async fn linsert_after(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "insert after", true, &[&key], async || {
                let result = self.backend.linsert_after(&key, pivot, value, ttl).await;
                match result {
                    Ok(n) if n < 0 => Err(Error::Miss),
                    other => other,
                }
            })
            .await
    }

    /// Remove elements from list.
    pub async fn lrem(
        &self,
        key: &str,
        count: i64,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let op = if count == 0 {
            LRemOp::All
        } else if count > 0 {
            LRemOp::First(count)
        } else {
            LRemOp::Last(-count)
        };
        self._lrem(key, op, value, ttl, source).await
    }

    async fn _lrem(
        &self,
        key: &str,
        op: LRemOp,
        value: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, op.name(), true, &[&key], async || {
                let count = {
                    use LRemOp::*;
                    match &op {
                        First(0) | Last(0) => return Ok(0),
                        First(count) | Last(count) if *count < 0 => {
                            return Err(Error::InvalidArgument("negative count"))
                        }
                        All => 0,
                        First(count) => *count,
                        Last(count) => -*count,
                    }
                };
                self.backend.lrem(&key, count, value, ttl).await
            })
            .await
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
    ) -> OpResult<Vec<u8>> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        self.tracer
            .trace(
                source,
                "list move",
                true,
                &[&src_key, &dst_key],
                async || {
                    self.backend
                        .lmove(&src_key, &dst_key, src_dir, dst_dir, ttl)
                        .await
                },
            )
            .await
    }

    /// Get list length.
    pub async fn llen(&self, key: &str, source: Option<&Request>) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "list len", false, &[&key], async || {
                self.backend.llen(&key).await
            })
            .await
    }

    /// Add members to a set.
    pub async fn sadd(
        &self,
        key: &str,
        members: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set add", true, &[&key], async || {
                self.backend.sadd(&key, members, ttl).await
            })
            .await
    }

    /// Remove members from a set.
    pub async fn srem(
        &self,
        key: &str,
        members: &[&[u8]],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set remove", true, &[&key], async || {
                self.backend.srem(&key, members, ttl).await
            })
            .await
    }

    /// Check if member exists in set.
    pub async fn sismember(
        &self,
        key: &str,
        member: &[u8],
        source: Option<&Request>,
    ) -> OpResult<bool> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set contains", false, &[&key], async || {
                self.backend.sismember(&key, member).await
            })
            .await
    }

    /// Pop a single random member from a set (SPOP without count).
    pub async fn spop_one(
        &self,
        key: &str,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set pop one", true, &[&key], async || {
                self.backend.spop_one(&key, ttl).await
            })
            .await
    }

    /// Pop random members from a set (SPOP with count).
    pub async fn spop(
        &self,
        key: &str,
        count: usize,
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set pop", true, &[&key], async || {
                self.backend.spop(&key, count, ttl).await
            })
            .await
    }

    /// Get a single random member from a set without removing (SRANDMEMBER).
    pub async fn srandmember(&self, key: &str, source: Option<&Request>) -> OpResult<Vec<u8>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set sample one", false, &[&key], async || {
                self.backend.srandmember(&key).await
            })
            .await
    }

    /// Get random members from a set without removing (SRANDMEMBER).
    pub async fn srandmember_multiple(
        &self,
        key: &str,
        count: i64,
        source: Option<&Request>,
    ) -> OpResult<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        let op = if count < 0 {
            "set sample with replacement"
        } else {
            "set sample"
        };
        self.tracer
            .trace(source, op, false, &[&key], async || {
                self.backend.srandmember_multiple(&key, count).await
            })
            .await
    }

    /// Get all members of a set.
    pub async fn smembers(&self, key: &str, source: Option<&Request>) -> OpResult<Vec<Vec<u8>>> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set items", false, &[&key], async || {
                self.backend.smembers(&key).await
            })
            .await
    }

    /// Get set cardinality.
    pub async fn scard(&self, key: &str, source: Option<&Request>) -> OpResult<i64> {
        let key = self.prefixed_key(key);
        self.tracer
            .trace(source, "set len", false, &[&key], async || {
                self.backend.scard(&key).await
            })
            .await
    }

    /// Set difference.
    pub async fn sdiff(&self, keys: &[&str], source: Option<&Request>) -> OpResult<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        self.tracer
            .trace(source, "set diff", false, &key_refs, async || {
                self.backend.sdiff(&key_refs).await
            })
            .await
    }

    /// Store set difference.
    pub async fn sdiffstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        self.tracer
            .trace(source, "store set diff", true, &all_keys, async || {
                self.backend.sdiffstore(&dest_key, &key_refs, ttl).await
            })
            .await
    }

    /// Set intersection.
    pub async fn sinter(&self, keys: &[&str], source: Option<&Request>) -> OpResult<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        self.tracer
            .trace(source, "intersect", false, &key_refs, async || {
                self.backend.sinter(&key_refs).await
            })
            .await
    }

    /// Store set intersection.
    pub async fn sinterstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        self.tracer
            .trace(source, "store set intersect", true, &all_keys, async || {
                self.backend.sinterstore(&dest_key, &key_refs, ttl).await
            })
            .await
    }

    /// Set union.
    pub async fn sunion(&self, keys: &[&str], source: Option<&Request>) -> OpResult<Vec<Vec<u8>>> {
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        self.tracer
            .trace(source, "union", false, &key_refs, async || {
                self.backend.sunion(&key_refs).await
            })
            .await
    }

    /// Store set union.
    pub async fn sunionstore(
        &self,
        dest: &str,
        keys: &[&str],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<i64> {
        let dest_key = self.prefixed_key(dest);
        let prefixed: Vec<String> = keys.iter().map(|k| self.prefixed_key(k)).collect();
        let mut all_keys: Vec<&str> = vec![dest_key.as_str()];
        let key_refs: Vec<&str> = prefixed.iter().map(|s| s.as_str()).collect();
        all_keys.extend(key_refs.iter().copied());
        self.tracer
            .trace(source, "store set union", true, &all_keys, async || {
                self.backend.sunionstore(&dest_key, &key_refs, ttl).await
            })
            .await
    }

    /// Move member between sets.
    pub async fn smove(
        &self,
        src: &str,
        dst: &str,
        member: &[u8],
        ttl: Option<TtlOp>,
        source: Option<&Request>,
    ) -> OpResult<bool> {
        let src_key = self.prefixed_key(src);
        let dst_key = self.prefixed_key(dst);
        self.tracer
            .trace(source, "move", true, &[&src_key, &dst_key], async || {
                self.backend.smove(&src_key, &dst_key, member, ttl).await
            })
            .await
    }
}

#[cfg(test)]
#[path = "client_tests.rs"]
mod client_tests;
