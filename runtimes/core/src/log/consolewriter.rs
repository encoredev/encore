use std::cell::RefCell;
use std::collections::BTreeMap;
use std::io::Write;
use std::sync::Mutex;
use anyhow::Context;
use chrono::Timelike;
use serde_json::Value;
use crate::log::fields::FieldConfig;
use crate::log::LogLevel;
use crate::log::writers::{Writer};
use colored::Colorize;
use crate::error::{StackFrame, write_stack_trace};

pub struct ConsoleWriter<W: Write + Sync + Send + 'static> {
    field_config: &'static FieldConfig,
    mu: Mutex<RefCell<Box<W>>>,
}

impl<W: Write + Sync + Send + 'static> ConsoleWriter<W> {
    pub fn new(field_config: &'static FieldConfig, w: W) -> Self {
        Self {
            field_config,
            mu: Mutex::new(RefCell::new(Box::new(w))),
        }
    }

    fn write_fields(&self, buf: &mut Vec<u8>, values: &BTreeMap<String, Value>) -> anyhow::Result<()> {
        // error field is always first
        if let Some(err) = values.get(self.field_config.error_field_name) {
            if buf.len() > 0 {
                buf.push(' ' as u8);
            }
            write!(buf, "{}{}", format!("{}=", self.field_config.error_field_name).cyan(), format!("{}", err).red()).map_err(std::io::Error::from).context("unable to write error field value")?;
        }

        for key in values.keys() {
            if key == self.field_config.timestamp_field_name
                || key == self.field_config.level_field_name
                || key == self.field_config.caller_field_name
                || key == self.field_config.message_field_name
                || key == self.field_config.error_field_name
                || key == self.field_config.stack_trace_field_name
            {
                continue;
            }

            if buf.len() > 0 {
                buf.push(' ' as u8);
            }
            write!(buf, "{}", format!("{}=", key).cyan()).map_err(std::io::Error::from).context(format!("unable to write field key {}", key))?;

            let value = values.get(key).expect("key not found");

            let value_to_print = match value {
                // Strings have special handling, as if the string does not contain any special characters or whitespace
                // we just print the string, otherwise we print the string as a JSON string (i.e. wrapped in quotes and escaped)
                Value::String(s) => {
                    if s.contains(" ") || s.contains("\t") || s.contains("\n") || s.contains("\r") || s.contains("\\") || s.contains("\"") {
                        format!("{}", value)
                    } else {
                        format!("{}", s)
                    }
                }
                _ => format!("{}", value),
            };

            write!(buf, "{}", value_to_print).map_err(std::io::Error::from).context(format!("unable to write field value {}", key))?;
        }

        // Finally, write the stack trace to the log
        if let Some(stack) = values.get(self.field_config.stack_trace_field_name) {
            let stack: Result<Vec<StackFrame>, serde_json::Error> = serde_json::from_value(stack.clone());
            if let Ok(stack) = stack {
                let mut writer = VecWriter { buf };
                write_stack_trace(&stack, &mut writer).context("unable to write stack trace")?;
            }
        }


        Ok(())
    }
}

struct VecWriter<'a> {
    buf: &'a mut Vec<u8>,
}
impl std::fmt::Write for VecWriter<'_> {
    fn write_str(&mut self, s: &str) -> std::fmt::Result {
        self.buf.extend_from_slice(s.as_bytes());
        Ok(())

    }
}

impl<W: Write + Sync + Send + 'static> Writer for ConsoleWriter<W> {
    fn write(&self, level: LogLevel, values: &BTreeMap<String, Value>) -> anyhow::Result<()> {
        let mut buf = Vec::with_capacity(256);

        write_part(&mut buf, self.field_config.timestamp_field_name, values, format_timestamp)?;
        write_level(&mut buf, level)?;
        write_part(&mut buf, self.field_config.caller_field_name, values, format_caller)?;
        write_part(&mut buf, self.field_config.message_field_name, values, format_message)?;

        self.write_fields(&mut buf, values)?;

        buf.write_all(b"\n").map_err(std::io::Error::from).context("new line")?;

        match self.mu.lock() {
            Ok(guard) => {
                let mut w = guard.try_borrow_mut().context("unable to borrow console output")?;
                w.write_all(&buf).map_err(std::io::Error::from).context("write")?;
                Ok(())
            }
            Err(poisoned) => {
                Err(anyhow::anyhow!("poisoned mutex: {:?}", poisoned))
            }
        }
    }
}

fn write_part(buf: &mut Vec<u8>, field: &'static str, values: &BTreeMap<String, Value>, mapper: fn(&str) -> anyhow::Result<String>) -> anyhow::Result<()> {
    if let Some(value) = values.get(field) {
        if let Some(value) = value.as_str() {
            let value = mapper(value).context(format!("unable to map part {}", field))?;
            if buf.len() > 0 {
                buf.push(' ' as u8);
            }
            write!(buf, "{}", value).map_err(std::io::Error::from).context(format!("unable to write part {}", field))?;
        }
    }
    Ok(())
}

fn format_timestamp(timestamp: &str) -> anyhow::Result<String> {
    let timestamp = chrono::DateTime::parse_from_rfc3339(timestamp).context(format!("unable to parse timestamp: {}", timestamp))?;
    let datetime: chrono::DateTime<chrono::Local> = timestamp.into();

    let (is_pm, hour) = datetime.hour12();
    let minute = datetime.minute();

    let mut timestamp = String::with_capacity(32);
    timestamp.push_str(&format!("{:02}:{:02}", hour, minute));

    if is_pm {
        timestamp.push_str("PM");
    } else {
        timestamp.push_str("AM");
    }

    Ok(format!("{}", timestamp.bright_black()))
}

fn write_level(buf: &mut Vec<u8>, level: LogLevel) -> anyhow::Result<()> {
    let level_str = match level {
        LogLevel::Trace => "TRC".magenta(),
        LogLevel::Debug => "DBG".yellow(),
        LogLevel::Info => "INF".green(),
        LogLevel::Warn => "WRN".red(),
        LogLevel::Error => "ERR".red().bold(),
        LogLevel::Fatal => "FTL".red().bold(),
        LogLevel::Disabled => "???".bold(),
    };

    if buf.len() > 0 {
        buf.push(' ' as u8);
    }
    write!(buf, "{}", level_str).map_err(std::io::Error::from).context("unable to write log level")?;

    Ok(())
}

fn format_caller(caller: &str) -> anyhow::Result<String> {
    Ok(format!("{} {}", caller.bold(), ">".cyan()))
}

fn format_message(message: &str) -> anyhow::Result<String> {
    Ok(message.to_string())
}