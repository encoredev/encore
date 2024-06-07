use crate::api::Request;
use encore_runtime_core::error::{AppError, StackFrame, StackTrace};
use encore_runtime_core::log::Fields;
use encore_runtime_core::log::LogFromExternalRuntime;
use napi::{Env, Error};
use napi_derive::napi;
use std::collections::HashMap;

/// A logger that can be used to log messages from the runtime.
#[napi]
pub struct Logger {
    pub(crate) logger: encore_runtime_core::log::Logger,
}

#[napi]
pub enum LogLevel {
    Trace = 1,
    Debug,
    Info,
    Warn,
    Error,
}

impl From<LogLevel> for log::LevelFilter {
    fn from(value: LogLevel) -> Self {
        match value {
            LogLevel::Trace => log::LevelFilter::Trace,
            LogLevel::Debug => log::LevelFilter::Debug,
            LogLevel::Info => log::LevelFilter::Info,
            LogLevel::Warn => log::LevelFilter::Warn,
            LogLevel::Error => log::LevelFilter::Error,
        }
    }
}

impl From<LogLevel> for log::Level {
    fn from(value: LogLevel) -> Self {
        match value {
            LogLevel::Trace => log::Level::Trace,
            LogLevel::Debug => log::Level::Debug,
            LogLevel::Info => log::Level::Info,
            LogLevel::Warn => log::Level::Warn,
            LogLevel::Error => log::Level::Error,
        }
    }
}

impl Default for Logger {
    fn default() -> Self {
        Self::new()
    }
}

#[napi]
impl Logger {
    pub fn new() -> Self {
        Self {
            logger: encore_runtime_core::log::root().clone(),
        }
    }

    /// log a message from the application
    #[napi]
    #[allow(clippy::too_many_arguments)]
    pub fn log(
        &self,
        env: Env,
        request: Option<&Request>,
        level: LogLevel,
        msg: String,
        #[napi(ts_arg_type = "Error")] error: Option<napi::JsObject>,
        #[napi(ts_arg_type = "string")] caller: Option<String>,
        #[napi(ts_arg_type = "Record<string, unknown>")] fields: Option<
            HashMap<String, napi::JsUnknown>,
        >,
    ) -> napi::Result<()> {
        self.logger
            .log(
                request.map(|r| r.inner.as_ref()),
                level.into(),
                msg,
                convert_error(env, error)?,
                caller,
                convert_fields(env, fields)?,
            )
            .map_err(Error::from)
    }

    /// Returns a new logger with the specified level
    #[napi]
    pub fn with_level(&self, level: LogLevel) -> Self {
        Self {
            logger: self.logger.with_level(level.into()),
        }
    }

    /// Returns a new logger with the given fields added to the context
    /// that the logger will use when emitting logs as extra fields
    #[napi]
    pub fn with(
        &self,
        env: Env,
        #[napi(ts_arg_type = "Record<string, unknown>")] fields: HashMap<String, napi::JsUnknown>,
    ) -> napi::Result<Self> {
        let fields = convert_fields(env, Some(fields))?.unwrap();

        Ok(Self {
            logger: self.logger.with(fields),
        })
    }
}

fn convert_error(env: Env, input: Option<napi::JsObject>) -> napi::Result<Option<AppError>> {
    match input {
        None => Ok(None),
        Some(input) => {
            let message: napi::JsUnknown = input.get_named_property("message")?;
            let message = message.coerce_to_string()?;
            let message: String = env.from_js_value(message)?;

            // try to convert the JS stack trace
            let stack: napi::Result<napi::JsUnknown> = input.get_named_property("stack");
            let stack = stack
                .map(|unknown| parse_js_stack(&env, unknown).unwrap_or_default())
                .unwrap_or_default();

            Ok(Some(AppError {
                message,
                stack,
                cause: None,
            }))
        }
    }
}

pub fn parse_js_stack(env: &Env, value: napi::JsUnknown) -> napi::Result<StackTrace> {
    let value = value.coerce_to_string()?;
    let value: String = env.from_js_value(value)?;

    Ok(value
        .lines()
        .filter_map(extract_frame)
        .filter(|frame| !is_common_frame(frame))
        .collect::<Vec<StackFrame>>())
}

/// is_common_frame returns true if the frame is a common frame that should be ignored.
/// as it is outside the users code base (such as it comes from the node runtime).
fn is_common_frame(frame: &StackFrame) -> bool {
    // "node:" is a node internal frame
    // "bun:" is a bun internal frame
    // "deno:" is a deno internal frame
    // "" is a frame with no file, so a core JS function
    frame.file.starts_with("node:")
        || frame.file.starts_with("bun:")
        || frame.file.starts_with("deno:")
        || frame.file.is_empty()
}

/// Attempts to extract a stack frame from a line of text.
///
/// All frames in javascript engines are in one of the formats of (after whitespaces):
/// - "at <function> (<file>:<line>:<column>)"
/// - "at <file>:<line>:<column>"
fn extract_frame(line: &str) -> Option<StackFrame> {
    let line = line.trim_start().trim_end();
    if !line.starts_with("at ") {
        return None;
    }
    let line = line[2..].trim_start();

    let (function, file, line, column) = if line.ends_with(')') {
        match line.rsplit_once('(') {
            Some((function, rest)) => {
                let (file, line, column) = match extract_file_line_col(rest.strip_suffix(')')?) {
                    Some((file, line, column)) => (file, line, column),
                    None => return None,
                };

                (Some(function.trim()), file, line, column)
            }
            None => return None,
        }
    } else {
        extract_file_line_col(line).map(|(file, line, column)| (None, file, line, column))?
    };

    // Format the file path to be relative to the current working directory
    let file = match std::env::current_dir() {
        Ok(cwd) => {
            let prefix = format!("{}/", cwd.to_string_lossy());
            file.strip_prefix(prefix.as_str()).unwrap_or(file.as_str())
        }
        Err(_) => file.as_str(),
    };

    Some(StackFrame {
        file: file.to_string(),
        line,
        column,
        module: None,
        function: function.map(|s| s.to_string()),
    })
}

fn extract_file_line_col(string: &str) -> Option<(String, u32, Option<u32>)> {
    let string = string.strip_prefix("file://").unwrap_or(string);
    let (file, rest) = string.split_once(':')?;

    match rest.split_once(':') {
        Some((line, column)) => Some((
            file.trim().to_string(),
            line.parse::<u32>().ok()?,
            Some(column.parse::<u32>().ok()?),
        )),
        None => Some((file.trim().to_string(), rest.parse::<u32>().ok()?, None)),
    }
}

/// converts a hash map of unknown JS values to a BTree of serde_json::Value's
fn convert_fields(
    env: Env,
    input: Option<HashMap<String, napi::JsUnknown>>,
) -> napi::Result<Option<Fields>> {
    match input {
        None => Ok(None),
        Some(input) => {
            if input.is_empty() {
                return Ok(None);
            }

            let mut fields = Fields::new();

            for (key, value) in input {
                let value: serde_json::Value = env.from_js_value(value)?;
                fields.insert(key, value);
            }

            Ok(Some(fields))
        }
    }
}
