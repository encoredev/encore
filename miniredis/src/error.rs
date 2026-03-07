/// Error type for miniredis-rs.
pub type Error = Box<dyn std::error::Error + Send + Sync>;

/// Result type for miniredis-rs.
pub type Result<T> = std::result::Result<T, Error>;
