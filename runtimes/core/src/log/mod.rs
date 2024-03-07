use std::sync::Arc;

use once_cell::sync::OnceCell;

mod consolewriter;
mod fields;
mod levels;
mod logger;
mod writers;

pub use levels::LogLevel;
pub use logger::{Fields, LogFromExternalRuntime, LogFromRust, Logger};

/// The global root logger instance that is used by both the `log` crate
/// and all other code in the Encore runtime.
static ROOT: OnceCell<&Logger> = OnceCell::new();

/// Initialize the global logger with the `root()` instance
///
/// This function is idempotent and will not re-initialize the logger
/// if it has already been initialized.
pub fn init() {
    _ = root();
}

/// Returns a reference to the global root logger instance.
pub fn root() -> &'static Logger {
    ROOT.get_or_init(|| {
        // Leak the logger to ensure it has a static lifetime.
        // We only do this once.
        let logger = Logger::default();
        let logger = Box::leak(Box::new(logger));

        let disable_logging = std::env::var("ENCORE_NOLOG").is_ok_and(|v| v != "");
        let filter = if disable_logging {
            log::LevelFilter::Off
        } else {
            log::LevelFilter::Trace
        };

        log::set_max_level(filter);
        log::set_logger(logger).expect("unable to set global logger instance");
        logger
    })
}
