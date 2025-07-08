use crate::log::consolewriter::ConsoleWriter;
use crate::log::fields::FieldConfig;
use anyhow::Context;
use serde_json::Value;
use std::cell::RefCell;
use std::collections::BTreeMap;
use std::env;
use std::fmt::Debug;
use std::io::Write;
use std::sync::mpsc::SyncSender;
use std::sync::{Arc, Mutex};

/// A log writer.
pub trait Writer: Send + Sync + 'static {
    /// Write the given key-value pairs to the log.
    fn write(&self, level: log::Level, values: &BTreeMap<String, Value>) -> anyhow::Result<()>;
}

impl Debug for dyn Writer {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Writer").finish()
    }
}

/// default_writer returns the default writer based on the environment.
///
/// If the `ENCORE_LOG_FORMAT` environment variable is set to `console` then
/// the pretty console writer will be used to write logs to stderr, otherwise
/// JSONL logs will be written to stderr.
///
/// For JSONL logs, if a tokio runtime is detected then the async writer
/// will be used, otherwise a blocking writer will be used, resulting
/// in blocking writes to stderr.
pub fn default_writer(fields: &'static FieldConfig) -> Arc<dyn Writer> {
    // Check if the user has set the `ENCORE_LOG_FORMAT` environment variable to `console`.
    // if so we'll use the pretty console writer.
    for var in &["ENCORE_LOG_FORMAT"] {
        if let Ok(format) = env::var(var) {
            if format == "console" {
                return Arc::new(ConsoleWriter::new(fields, std::io::stderr()));
            }
        }
    }

    Arc::new(ActorWriter::default())
}

// ActorWriter creates a bounded channel that sends log data to a separate thread that handles the writing.
pub struct ActorWriter {
    sender: SyncSender<bytes::Bytes>,
}
impl ActorWriter {
    pub fn new<W: Write + Sync + Send + 'static>(w: W) -> Self {
        let (sender, recv) = std::sync::mpsc::sync_channel::<bytes::Bytes>(1024);
        std::thread::spawn(move || {
            let mut writer = w;
            while let Ok(bytes) = recv.recv() {
                if let Err(e) = writer.write_all(&bytes) {
                    eprintln!("Failed to write log entry: {e}");
                    continue;
                }
            }
        });
        Self { sender }
    }
}
impl Writer for ActorWriter {
    fn write(&self, _: log::Level, values: &BTreeMap<String, Value>) -> anyhow::Result<()> {
        let mut buf = Vec::with_capacity(256);
        serde_json::to_writer(&mut buf, values)
            .map_err(std::io::Error::from)
            .context("serde_writer")?;
        buf.extend_from_slice(b"\n");

        self.sender.send(buf.into())?;
        Ok(())
    }
}

impl Default for ActorWriter {
    fn default() -> Self {
        Self::new(std::io::stderr())
    }
}

/// A log writer that synchronizes writes to stderr blocking
/// until the write is complete.
#[derive(Debug)]
pub struct BlockingWriter<W: Write + Sync + Send + 'static> {
    mu: Mutex<RefCell<Box<W>>>,
}

impl<W: Write + Sync + Send + 'static> BlockingWriter<W> {
    pub fn new(w: W) -> Self {
        Self {
            mu: Mutex::new(RefCell::new(Box::new(w))),
        }
    }
}

impl Default for BlockingWriter<std::io::Stderr> {
    fn default() -> Self {
        Self::new(std::io::stderr())
    }
}

/// A Writer implementation that writes logs in JSON format.
impl<W: Write + Sync + Send + 'static> Writer for BlockingWriter<W> {
    fn write(&self, _: log::Level, values: &BTreeMap<String, Value>) -> anyhow::Result<()> {
        let mut buf = Vec::with_capacity(256);
        serde_json::to_writer(&mut buf, values)
            .map_err(std::io::Error::from)
            .context("serde_writer")?;
        buf.write_all(b"\n").context("new line")?;

        match self.mu.lock() {
            Ok(guard) => {
                let mut w = guard.try_borrow_mut().context("unable to borrow")?;
                w.write_all(&buf).context("write")?;
                Ok(())
            }
            Err(poisoned) => Err(anyhow::anyhow!("poisoned mutex: {:?}", poisoned)),
        }
    }
}
