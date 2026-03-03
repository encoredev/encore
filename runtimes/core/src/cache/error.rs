use bb8_redis::redis;
use thiserror::Error;

/// Result type for internal cache operations (without operation context).
pub type Result<T> = std::result::Result<T, Error>;

/// Result type for pool operations with operation context.
pub type OpResult<T> = std::result::Result<T, OpError>;

/// Error wrapper with operation context (operation name + key).
/// Analogous to Go's `cache.OpError`.
#[derive(Error, Debug)]
#[error("cache {operation} \"{key}\": {source}")]
pub struct OpError {
    pub operation: &'static str,
    pub key: String,
    #[source]
    pub source: Error,
}

impl OpError {
    pub fn new(operation: &'static str, key: &str, source: Error) -> Self {
        Self {
            operation,
            key: key.to_string(),
            source,
        }
    }
}

/// Error type for cache operations.
#[derive(Error, Debug, Clone)]
pub enum Error {
    /// Miss is the error value reported when a key is missing from the cache.
    #[error("cache miss")]
    Miss,

    /// KeyExists is the error reported when a key already exists
    /// and the requested operation is specified to only apply to
    /// keys that do not already exist.
    #[error("key already exist")]
    KeyExist,

    /// Redis error.
    #[error("redis error: {0}")]
    Redis(#[from] redis::RedisError),

    /// Connection pool error.
    #[error("connection pool timeout")]
    PoolTimeout,
}
