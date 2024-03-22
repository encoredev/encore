use std::env;
use std::fmt::Display;
use std::str::FromStr;

/// The LogLevel represents the log level of a log message.
///
/// The default log level is `INFO` unless one of the following environment variables
/// is set to a valid log level:
/// - `LOG`
/// - `LOG_LEVEL`
/// - `RUST_LOG`
/// - `ENCORE_LOG_LEVEL`
///
/// If none of the above are set, but `TRACE` or `DEBUG` are set, those log levels
/// will be used instead.
#[derive(PartialEq, Eq, PartialOrd, Ord, Debug, Clone, Copy)]
pub enum LogLevel {
    Trace = 1,
    Debug,
    Info,
    Warn,
    Error,
    Fatal,
    Disabled = 99,
}

impl Default for LogLevel {
    fn default() -> Self {
        // First check if any of the environment variables are set to control the log level
        // and if they are, use that log level.
        for var in &["ENCORE_LOG_LEVEL", "RUST_LOG", "LOG_LEVEL", "LOG"] {
            if let Ok(level) = env::var(var) {
                if let Ok(level) = level.parse::<LogLevel>() {
                    return level;
                }
            }
        }

        // Next check if `TRACE` or `DEBUG` are set and use those log levels.
        if env::var("TRACE").is_ok() {
            return LogLevel::Trace;
        } else if env::var("DEBUG").is_ok() {
            return LogLevel::Debug;
        }

        // Finally, default to `INFO`.
        LogLevel::Info
    }
}

impl FromStr for LogLevel {
    type Err = ();

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "trace" | "trc" => Ok(LogLevel::Trace),
            "debug" | "dbg" => Ok(LogLevel::Debug),
            "info" | "inf" => Ok(LogLevel::Info),
            "warn" | "wrn" => Ok(LogLevel::Warn),
            "error" | "err" => Ok(LogLevel::Error),
            "fatal" | "ftl" => Ok(LogLevel::Fatal),
            "disabled" | "dis" | "none" | "off" => Ok(LogLevel::Disabled),
            _ => Err(()),
        }
    }
}

impl Display for LogLevel {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let str = match self {
            LogLevel::Trace => "TRACE",
            LogLevel::Debug => "DEBUG",
            LogLevel::Info => "INFO",
            LogLevel::Warn => "WARN",
            LogLevel::Error => "ERROR",
            LogLevel::Fatal => "FATAL",
            LogLevel::Disabled => "DISABLED",
        }
        .to_string();
        write!(f, "{}", str)
    }
}

/// Allows conversion of a `log::Level` into to a `LogLevel`
impl Into<LogLevel> for log::Level {
    fn into(self) -> LogLevel {
        match self {
            log::Level::Trace => LogLevel::Trace,
            log::Level::Debug => LogLevel::Debug,
            log::Level::Info => LogLevel::Info,
            log::Level::Warn => LogLevel::Warn,
            log::Level::Error => LogLevel::Error,
        }
    }
}
