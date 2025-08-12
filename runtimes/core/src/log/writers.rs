use crate::log::consolewriter::ConsoleWriter;
use crate::log::fields::FieldConfig;
use anyhow::Context;
use serde_json::Value;
use std::collections::BTreeMap;
use std::env;
use std::fmt::Debug;
use std::io::{IoSlice, Write};
use std::sync::mpsc::{self, Receiver, RecvError, SyncSender, TryRecvError};
use std::sync::Arc;
use std::time::Duration;

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
    sender: SyncSender<Vec<u8>>,
}
impl ActorWriter {
    pub fn new<W: Write + Sync + Send + 'static>(mut writer: W) -> Self {
        let (sender, recv) = mpsc::sync_channel::<Vec<u8>>(10_000);
        std::thread::spawn(move || {
            while let Ok(bytes) = Self::recv_batch(&recv) {
                Self::write_batch_with_retry(&mut writer, &bytes);
            }
        });
        Self { sender }
    }

    fn recv_batch(recv: &Receiver<Vec<u8>>) -> Result<Vec<Vec<u8>>, RecvError> {
        const MAX_BATCH_SIZE: usize = 256;

        // wait for a log message
        let mut bufs = vec![recv.recv()?];

        // receive logs until channel is empty or max batch size is reached
        loop {
            match recv.try_recv() {
                Ok(log) => {
                    bufs.push(log);

                    if bufs.len() >= MAX_BATCH_SIZE {
                        break;
                    }
                }
                // on error, break the loop and return the bufs that we have already collected.
                Err(TryRecvError::Disconnected) => break,
                Err(TryRecvError::Empty) => break,
            }
        }
        Ok(bufs)
    }

    fn write_batch_with_retry<W: Write>(writer: &mut W, bufs: &[Vec<u8>]) {
        const INITIAL_DELAY_MS: u64 = 1;
        const MAX_DELAY_MS: u64 = 1000;

        let mut io_slices = bufs
            .iter()
            .map(|buf| IoSlice::new(buf))
            .collect::<Vec<IoSlice>>();
        let mut bufs = &mut io_slices[..];

        // Guarantee that bufs is empty if it contains no data,
        // to avoid calling write_vectored if there is no data to be written.
        IoSlice::advance_slices(&mut bufs, 0);
        let mut delay_ms = INITIAL_DELAY_MS;
        while !bufs.is_empty() {
            match writer.write_vectored(bufs) {
                Ok(0) | Err(_) => {
                    std::thread::sleep(Duration::from_millis(delay_ms));
                    delay_ms = u64::min(delay_ms * 2, MAX_DELAY_MS);
                }
                Ok(n) => {
                    delay_ms = INITIAL_DELAY_MS;
                    IoSlice::advance_slices(&mut bufs, n)
                }
            }
        }
    }
}
impl Writer for ActorWriter {
    fn write(&self, _: log::Level, values: &BTreeMap<String, Value>) -> anyhow::Result<()> {
        let mut buf = Vec::with_capacity(256);
        serde_json::to_writer(&mut buf, values)
            .map_err(std::io::Error::from)
            .context("serde_writer")?;
        buf.extend_from_slice(b"\n");

        self.sender.send(buf)?;
        Ok(())
    }
}

impl Default for ActorWriter {
    fn default() -> Self {
        Self::new(std::io::stderr())
    }
}
