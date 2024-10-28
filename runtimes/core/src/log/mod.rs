use once_cell::sync::OnceCell;

mod consolewriter;
mod fields;
mod logger;
mod writers;

use crate::log::fields::FieldConfig;
pub use logger::{Fields, LogFromExternalRuntime, LogFromRust, Logger};

use crate::trace::Tracer;

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

/// Set the tracer on the global logger
pub fn set_tracer(tracer: Tracer) {
    root().set_tracer(tracer);
}

/// Returns a reference to the global root logger instance.
pub fn root() -> &'static Logger {
    ROOT.get_or_init(|| {
        let logger = {
            let fields = FieldConfig::default();

            // Construct our rust log filter.
            let filter = {
                // If RUST_LOG is set, use that.
                let level = std::env::var("RUST_LOG").unwrap_or_else(|_| {
                    // Otherwise use ENCORE_RUNTIME_LOG to set the Encore runtime log level,
                    // which defaults
                    let level = std::env::var("ENCORE_RUNTIME_LOG").unwrap_or("debug".to_string());
                    format!("encore_={level},pingora_core::listeners=warn,pingora_core::services::listening=warn")
                });
                env_logger::filter::Builder::new().parse(&level).build()
            };

            // Construct our app log level.
            let app_level: log::LevelFilter = std::env::var("ENCORE_LOG")
                .ok()
                .and_then(|v| v.parse().ok())
                .unwrap_or(log::LevelFilter::Trace);

            Logger::new(app_level, filter, fields)
        };

        // Leak the logger to ensure it has a static lifetime.
        // We only do this once.
        let logger = Box::leak(Box::new(logger));

        let disable_logging = std::env::var("ENCORE_NOLOG").is_ok_and(|v| !v.is_empty());
        let filter = if disable_logging {
            log::LevelFilter::Off
        } else {
            log::LevelFilter::Trace
        };

        #[cfg(feature = "rttrace")]
        {
            let filter = tracing_subscriber::EnvFilter::from_env("ENCORE_RUNTIME_TRACE");
            tracing_subscriber::fmt()
                .with_span_events(tracing_subscriber::fmt::format::FmtSpan::ENTER)
                .with_env_filter(filter)
                .with_writer(std::io::stderr)
                .init();
        }

        log::set_max_level(filter);
        log::set_logger(logger).expect("unable to set global logger instance");
        logger
    })
}
