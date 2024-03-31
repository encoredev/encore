use crate::error::AppError;
use crate::log::fields::{FieldConfig, DEFAULT_FIELDS, GCP_FIELDS};
use crate::log::writers::{default_writer, Writer};
use crate::log::LogLevel;
use crate::model;
use anyhow::Context;
use log::{Log, Metadata, Record};
use std::collections::BTreeMap;
use std::sync::Arc;
use std::time::SystemTime;

pub type Fields = BTreeMap<String, serde_json::Value>;

/// Logger is a structured JSON logger that can be used to emit structured logs to stderr
#[derive(Debug, Clone)]
pub struct Logger {
    level: LogLevel,
    field_config: &'static FieldConfig,
    writer: Arc<dyn Writer>,
    extra_fields: Fields,
}

impl Logger {
    /// New returns a new logger with the given field config.
    pub fn new(field_config: &'static FieldConfig) -> Self {
        Self {
            level: LogLevel::default(),
            field_config,
            writer: default_writer(field_config),
            extra_fields: Fields::new(),
        }
    }

    /// Returns a new logger with the given log level.
    pub fn with_level(&self, level: LogLevel) -> Self {
        Self {
            level,
            ..self.clone()
        }
    }

    /// Returns a new logger with the given writer.
    pub fn with_writer(&self, writer: Arc<dyn Writer>) -> Self {
        Self {
            writer,
            ..self.clone()
        }
    }

    /// Returns a new logger with the given fields added to the context
    /// that the logger will use when emitting logs as extra fields
    pub fn with(&self, fields: Fields) -> Self {
        let mut replacement = self.clone();

        for (key, value) in fields.iter() {
            replacement
                .extra_fields
                .insert(key.to_string(), value.clone());
        }

        replacement
    }

    /// Returns the current log level as expected by the `log` crate.
    fn level_to_value(&self, level: LogLevel) -> serde_json::Value {
        serde_json::Value::from(match level {
            LogLevel::Trace => &self.field_config.level_trace_value,
            LogLevel::Debug => &self.field_config.level_debug_value,
            LogLevel::Info => &self.field_config.level_info_value,
            LogLevel::Warn => &self.field_config.level_warn_value,
            LogLevel::Error => &self.field_config.level_error_value,
            LogLevel::Fatal => &self.field_config.level_fatal_value,
            LogLevel::Disabled => "disabled",
        })
    }

    /// Takes the given message and attempts to log it to the configured writer.
    fn try_log(
        &self,
        request: Option<&model::Request>,
        level: LogLevel,
        msg: String,
        error: Option<AppError>,
        caller: Option<String>,
        fields: Option<Fields>,
    ) -> anyhow::Result<()> {
        if level < self.level {
            return Ok(());
        }

        let mut values = Fields::new();

        // Copy the extra fields into the values map.
        for (key, value) in self.extra_fields.iter() {
            values.insert(key.to_string(), value.clone());
        }

        // Copy the fields from the logger into the values map.
        if let Some(fields) = fields {
            values.extend(fields);
        }

        // If we have a caller field, add it to the values map.
        if let Some(caller) = caller {
            values.insert(
                self.field_config.caller_field_name.to_string(),
                serde_json::Value::from(caller),
            );
        }

        // If we have an error field, then let's add it
        if let Some(error) = error {
            values.insert(
                self.field_config.error_field_name.to_string(),
                serde_json::Value::from(error.message),
            );

            if error.stack.len() > 0 {
                values.insert(
                    self.field_config.stack_trace_field_name.to_string(),
                    serde_json::to_value(error.stack)?,
                );
            }
        }

        // Now add the standard fields.
        values.insert(
            self.field_config.level_field_name.to_string(),
            self.level_to_value(level),
        );
        values.insert(
            self.field_config.timestamp_field_name.to_string(),
            iso8601_now(),
        );
        values.insert(
            self.field_config.message_field_name.to_string(),
            serde_json::Value::from(msg),
        );

        if let Some(req) = request {
            match &req.data {
                model::RequestData::RPC(rpc) => {
                    let ep = &rpc.endpoint.name;
                    values.insert(
                        "service".into(),
                        serde_json::Value::String(ep.service().to_string()),
                    );
                    values.insert(
                        "endpoint".into(),
                        serde_json::Value::String(ep.endpoint().to_string()),
                    );
                    if let Some(uid) = &rpc.auth_user_id {
                        values.insert("uid".into(), serde_json::Value::String(uid.clone()));
                    }
                }
                model::RequestData::Auth(auth) => {
                    let ep = &auth.auth_handler;
                    values.insert(
                        "service".into(),
                        serde_json::Value::String(ep.service().to_string()),
                    );
                    values.insert(
                        "endpoint".into(),
                        serde_json::Value::String(ep.endpoint().to_string()),
                    );
                }
                model::RequestData::PubSub(msg) => {
                    values.insert(
                        "service".into(),
                        serde_json::Value::String(msg.service.to_string()),
                    );
                    values.insert(
                        "topic".into(),
                        serde_json::Value::String(msg.topic.to_string()),
                    );
                    values.insert(
                        "subscription".into(),
                        serde_json::Value::String(msg.subscription.to_string()),
                    );
                }
            };

            values.insert(
                "trace_id".into(),
                serde_json::Value::String(req.span.0.serialize_encore()),
            );
            values.insert(
                "span_id".into(),
                serde_json::Value::String(req.span.1.serialize_encore()),
            );

            if let Some(corr_id) = &req.ext_correlation_id {
                values.insert(
                    "x_correlation_id".into(),
                    serde_json::Value::String(corr_id.clone()),
                );
            } else if let Some(parent_trace) = &req.parent_trace {
                values.insert(
                    "x_correlation_id".into(),
                    serde_json::Value::String(parent_trace.serialize_encore()),
                );
            }
        }

        // Now write the log to the configured writer.
        self.writer
            .write(level, &values)
            .context("unable to write")?;

        Ok(())
    }

    /// Takes a `log::Record` and attempts to log it to the configured writer.
    fn try_log_record(&self, record: &Record) -> anyhow::Result<()> {
        let kvs = record.key_values();
        let mut visitor = KeyValueVisitor(BTreeMap::new());
        let _ = kvs.visit(&mut visitor);

        let msg = match record.args().as_str() {
            Some(msg) => msg.to_string(),
            None => record.args().to_string(),
        };

        let caller = match (record.file(), record.line()) {
            (Some(file), Some(line)) => Some(format!("{}:{}", file, line)),
            _ => None,
        };

        self.try_log(
            None,
            record.level().into(),
            msg,
            None,
            caller,
            Some(visitor.0),
        )
    }
}

/// This trait defines the logging functions that are available on the `Logger` type.
///
/// It is used to allow Rust code to emit structured logs via our `Logger` implementation
/// as it will automatically capture the caller location.
pub trait LogFromRust<T: std::fmt::Display> {
    fn trace(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>);
    fn debug(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>);

    fn info(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>);

    fn warn<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    );

    fn error<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    );

    fn fatal<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    );
}

/// This trait defines the logging functions that are available on the `Logger` type.
///
/// It is used to allow other languages to emit structured logs via our `Logger` implementation
/// with them passing in their own caller location.
pub trait LogFromExternalRuntime<T: std::fmt::Display> {
    fn log<Err: Into<AppError>>(
        &self,
        request: Option<&model::Request>,
        level: LogLevel,
        msg: T,
        error: Option<Err>,
        caller: Option<String>,
        fields: Option<Fields>,
    ) -> anyhow::Result<()>;
}

impl<T> LogFromExternalRuntime<T> for Logger
where
    T: std::fmt::Display,
{
    /// Logs the given message at the trace level
    fn log<Err: Into<AppError>>(
        &self,
        request: Option<&model::Request>,
        level: LogLevel,
        msg: T,
        error: Option<Err>,
        caller: Option<String>,
        fields: Option<Fields>,
    ) -> anyhow::Result<()> {
        self.try_log(
            request,
            level,
            msg.to_string(),
            error.map(|e| e.into().trim_stack(file!(), line!(), 1)),
            caller,
            fields,
        )
    }
}

impl<T> LogFromRust<T> for Logger
where
    T: std::fmt::Display,
{
    #[track_caller]
    fn trace(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>) {
        self.try_log(
            req,
            LogLevel::Trace,
            msg.to_string(),
            None,
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }

    #[track_caller]
    fn debug(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>) {
        self.try_log(
            req,
            LogLevel::Debug,
            msg.to_string(),
            None,
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }

    #[track_caller]
    fn info(&self, req: Option<&model::Request>, msg: T, fields: Option<Fields>) {
        self.try_log(
            req,
            LogLevel::Info,
            msg.to_string(),
            None,
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }

    #[track_caller]
    fn warn<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    ) {
        self.try_log(
            req,
            LogLevel::Warn,
            msg.to_string(),
            error.map(|e| e.into().trim_stack(file!(), line!(), 1)),
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }

    #[track_caller]
    fn error<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    ) {
        self.try_log(
            req,
            LogLevel::Error,
            msg.to_string(),
            error.map(|e| e.into().trim_stack(file!(), line!(), 1)),
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }

    #[track_caller]
    fn fatal<Err: Into<AppError>>(
        &self,
        req: Option<&model::Request>,
        msg: T,
        error: Option<Err>,
        fields: Option<Fields>,
    ) {
        self.try_log(
            req,
            LogLevel::Fatal,
            msg.to_string(),
            error.map(|e| e.into().trim_stack(file!(), line!(), 1)),
            None, // get_rust_caller(),
            fields,
        )
        .expect("failed to log");
    }
}

#[inline]
#[track_caller]
#[allow(dead_code)]
fn get_rust_caller() -> Option<String> {
    let location = std::panic::Location::caller();
    Some(format!("{}:{}", location.file(), location.line()))
}

#[inline]
fn should_ignore_caller(c: Option<&str>) -> bool {
    match c {
        Some(c) => c.contains(".cargo/registry/src/"),
        None => false,
    }
}

/// Returns the current unix timestamp in milliseconds.
#[inline]
pub fn iso8601_now() -> serde_json::Value {
    let now = SystemTime::now();
    let date = chrono::DateTime::<chrono::Utc>::from(now);

    serde_json::Value::from(date.to_rfc3339_opts(chrono::SecondsFormat::Millis, true))
}

impl Default for Logger {
    fn default() -> Self {
        // If we're running in GCP, then we'll use the GCP fields.
        for var in &[
            "GCP_PROJECT",
            "GOOGLE_CLOUD_PROJECT",
            "GCP_METADATA_PROJECT",
        ] {
            if let Ok(_) = std::env::var(var) {
                return Self::new(&GCP_FIELDS);
            }
        }

        Self::new(&DEFAULT_FIELDS)
    }
}

/// Implement the `Log` trait for `Logger` which allows other creates which use the `log` facade
/// crate to emit structured logs via our `Logger` implementation.
impl Log for Logger {
    fn enabled(&self, metadata: &Metadata) -> bool {
        let level: LogLevel = metadata.level().into();
        level >= self.level
    }

    fn log(&self, record: &Record) {
        if self.enabled(record.metadata()) && !should_ignore_caller(record.file()) {
            self.try_log_record(record).unwrap_or_else(|e| {
                eprintln!("failed to log: {}", e);
            });
        }
    }

    fn flush(&self) {}
}

/// A visitor that can be used to visit key-value pairs and insert them into a `BTreeMap`.
/// after converting them from the `log::kv::Value` type to `serde_json::Value`.
struct KeyValueVisitor(BTreeMap<String, serde_json::Value>);

impl log::kv::Visitor<'_> for KeyValueVisitor {
    #[inline]
    fn visit_pair(
        &mut self,
        key: log::kv::Key,
        value: log::kv::Value,
    ) -> Result<(), log::kv::Error> {
        match serde_json::to_value(&value) {
            Ok(value) => {
                self.0.insert(key.to_string(), value);
                Ok(())
            }
            Err(e) => Err(log::kv::Error::boxed(e)),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::LogFromRust;
    use crate::error::AppError;
    use crate::log::{init, root};
    use colored::control;
    use log::{debug, error, info, trace, warn};

    #[test]
    fn test_logger() {
        init();

        control::set_override(true);

        trace!("something tracing");
        debug!("this was a debug");
        info!(zzz=123,a_boolean=true,another_string="err",error="some error here";"hello world");
        warn!(some_numer=12;"hello from a trace");
        error!("this is an error");

        root().info(None, "hello world", None);
        root().error(None, "this error", Some(AppError::new("boo hoo")), None);
        root().error(None, "this error 2", Some("I'm sad"), None);
    }
}
