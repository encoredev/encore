/// Correlates a system time with a time instant.
#[derive(Debug, Clone)]
pub struct TimeAnchor {
    /// The system time.
    pub system_time: chrono::DateTime<chrono::Utc>,

    /// The time instant.
    pub instant: tokio::time::Instant,
}

impl TimeAnchor {
    pub fn new() -> Self {
        TimeAnchor {
            system_time: chrono::Utc::now(),
            instant: tokio::time::Instant::now(),
        }
    }

    pub fn trace_header(&self) -> String {
        // Format the system as with RFC3339Nano.
        let dt = self
            .system_time
            .to_rfc3339_opts(chrono::SecondsFormat::AutoSi, true);

        format!("0 {}", dt)
    }
}
