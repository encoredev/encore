use bytes::BytesMut;
use std::collections::HashMap;
use std::io::Cursor;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt, BufWriter};
use tokio::net::TcpStream;

use crate::frame::{Frame, FrameError};

/// Trait alias for an async stream that supports both read and write.
pub trait IoStream: AsyncRead + AsyncWrite + Unpin + Send {}
impl<T: AsyncRead + AsyncWrite + Unpin + Send> IoStream for T {}

/// A connection wraps a stream with buffered read/write and RESP
/// frame parsing. Supports both plain TCP and TLS connections.
pub struct Connection {
    stream: BufWriter<Box<dyn IoStream>>,
    buffer: BytesMut,
    /// Whether this connection uses RESP3 encoding (set via HELLO 3).
    pub resp3: bool,
}

impl Connection {
    /// Create a new `Connection` backed by a plain TCP socket.
    pub fn new(socket: TcpStream) -> Connection {
        Connection {
            stream: BufWriter::new(Box::new(socket)),
            buffer: BytesMut::with_capacity(4096),
            resp3: false,
        }
    }

    /// Create a new `Connection` backed by any async read/write stream (e.g. TLS).
    pub fn new_stream(stream: impl IoStream + 'static) -> Connection {
        Connection {
            stream: BufWriter::new(Box::new(stream)),
            buffer: BytesMut::with_capacity(4096),
            resp3: false,
        }
    }

    /// Read a single RESP frame from the connection.
    ///
    /// Returns `None` if the remote half closed the connection cleanly.
    pub async fn read_frame(&mut self) -> crate::Result<Option<Frame>> {
        loop {
            // Try to parse a frame from the buffered data.
            if let Some(frame) = self.parse_frame()? {
                return Ok(Some(frame));
            }

            // Not enough data for a frame — read more from the socket.
            let n = self.stream.read_buf(&mut self.buffer).await?;
            if n == 0 {
                // Connection closed
                if self.buffer.is_empty() {
                    return Ok(None);
                } else {
                    return Err("connection reset by peer".into());
                }
            }
        }
    }

    /// Try to parse a frame from the current buffer contents.
    fn parse_frame(&mut self) -> crate::Result<Option<Frame>> {
        use bytes::Buf;

        let mut cursor = Cursor::new(&self.buffer[..]);

        match Frame::check(&mut cursor) {
            Ok(()) => {
                // We know a complete frame is in the buffer.
                let len = cursor.position() as usize;

                // Reset cursor and parse.
                cursor.set_position(0);
                let frame = Frame::parse(&mut cursor)
                    .map_err(|e| -> crate::Error { e.to_string().into() })?;

                // Advance the buffer past the consumed bytes.
                self.buffer.advance(len);

                Ok(Some(frame))
            }
            Err(FrameError::Incomplete) => Ok(None),
            Err(e) => Err(e.to_string().into()),
        }
    }

    /// Write a frame to the connection, using RESP3 encoding if negotiated.
    pub async fn write_frame(&mut self, frame: &Frame) -> crate::Result<()> {
        let bytes = frame.serialize_resp(self.resp3);
        self.stream.write_all(&bytes).await?;
        self.stream.flush().await?;
        Ok(())
    }

    /// Write raw bytes (used for inline protocol or multi-frame writes).
    pub async fn write_all(&mut self, data: &[u8]) -> crate::Result<()> {
        self.stream.write_all(data).await?;
        self.stream.flush().await?;
        Ok(())
    }
}

// ── Per-Connection State ─────────────────────────────────────────────

/// Per-connection context, carrying state that persists across commands
/// within a single client session.
pub struct ConnCtx {
    /// Currently selected database index (0-15).
    pub selected_db: usize,
    /// True once the client has sent a valid AUTH command (when passwords are configured).
    pub authenticated: bool,
    /// If Some, we're inside a MULTI block; the vec holds queued command args.
    pub transaction: Option<Vec<QueuedCommand>>,
    /// Set to true if any error occurs while queuing commands in a MULTI.
    pub dirty_transaction: bool,
    /// WATCH map: (db_index, key) -> version at WATCH time.
    pub watch: HashMap<(usize, String), u64>,
    /// True if the client negotiated RESP3 via HELLO.
    pub resp3: bool,
    /// CLIENT SETNAME value.
    pub client_name: Option<String>,
    /// True when executing inside a Lua script (nested call).
    pub nested: bool,
    /// SHA of the currently executing Lua script (if nested).
    pub nested_sha: Option<String>,
}

/// A command queued inside a MULTI transaction.
pub struct QueuedCommand {
    /// The raw arguments (command name + args).
    pub args: Vec<Vec<u8>>,
}

impl Default for ConnCtx {
    fn default() -> Self {
        Self::new()
    }
}

impl ConnCtx {
    pub fn new() -> Self {
        ConnCtx {
            selected_db: 0,
            authenticated: false,
            transaction: None,
            dirty_transaction: false,
            watch: HashMap::new(),
            resp3: false,
            client_name: None,
            nested: false,
            nested_sha: None,
        }
    }

    /// Are we inside a MULTI transaction?
    pub fn in_tx(&self) -> bool {
        self.transaction.is_some()
    }
}
