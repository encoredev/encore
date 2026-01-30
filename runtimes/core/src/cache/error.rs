use bb8_redis::redis;
use thiserror::Error;

/// Result type for cache operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for cache operations.
#[derive(Error, Debug)]
pub enum Error {
    /// Key was not found in the cache.
    #[error("cache miss")]
    KeyNotFound,

    /// Key already exists (used for SetIfNotExists).
    #[error("key already exists")]
    KeyExists,

    /// Key does not exist (used for operations that require existing key).
    #[error("no such key")]
    NoSuchKey,

    /// Type mismatch error (e.g., trying to use list operations on a string).
    #[error("type mismatch: {0}")]
    TypeMismatch(String),

    /// Invalid value error (e.g., value is not a valid integer).
    #[error("invalid value: {0}")]
    InvalidValue(String),

    /// Cache cluster is not configured for this service.
    #[error("cache: this service is not configured to use this cache cluster")]
    NotConfigured,

    /// Redis connection error.
    #[error("redis error: {0}")]
    Redis(#[from] redis::RedisError),

    /// Connection pool error.
    #[error("pool error: {0}")]
    Pool(String),

    /// Serialization error.
    #[error("serialization error: {0}")]
    Serialization(String),

    /// Internal error.
    #[error("internal error: {0}")]
    Internal(String),

    /// Connection error.
    #[error("connection error: {0}")]
    Connection(#[from] anyhow::Error),
}

/// Represents the result of a cache operation for tracing.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
#[allow(dead_code)]
pub enum OpResult {
    Unknown = 0,
    Ok = 1,
    NoSuchKey = 2,
    Conflict = 3,
    Err = 4,
}

impl From<&Error> for OpResult {
    fn from(err: &Error) -> Self {
        match err {
            Error::KeyNotFound | Error::NoSuchKey => OpResult::NoSuchKey,
            Error::KeyExists => OpResult::Conflict,
            _ => OpResult::Err,
        }
    }
}
