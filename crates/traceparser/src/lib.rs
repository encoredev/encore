//! Trace event parser for Encore's binary trace protocol.
//!
//! This crate parses the binary trace format emitted by Encore runtimes
//! (both Go and Rust) into structured Rust types.
//!
//! # Protocol
//!
//! Each trace event is encoded as a 45-byte header followed by a variable-length body:
//!
//! | Offset | Size | Field       |
//! |--------|------|-------------|
//! | 0      | 1    | Event type  |
//! | 1      | 8    | Event ID    |
//! | 9      | 8    | Nanotime    |
//! | 17     | 16   | Trace ID    |
//! | 33     | 8    | Span ID     |
//! | 41     | 4    | Data length |
//! | 45     | N    | Event data  |
//!
//! # Usage
//!
//! ```no_run
//! use encore_traceparser::{parse_event, TimeAnchor, Timestamp};
//!
//! let time_anchor = TimeAnchor {
//!     real: Timestamp { seconds: 1700000000, nanos: 0 },
//!     mono_nanos: 0,
//! };
//!
//! let data: &[u8] = &[/* trace bytes */];
//! let mut cursor = std::io::Cursor::new(data);
//!
//! loop {
//!     match parse_event(&mut cursor, &time_anchor, 17) {
//!         Ok(event) => println!("{:?}", event),
//!         Err(encore_traceparser::ParseError::EndOfStream) => break,
//!         Err(e) => eprintln!("parse error: {}", e),
//!     }
//! }
//! ```

pub mod types;
mod parser;
mod reader;

pub use parser::parse_event;
pub use types::{ParseError, TimeAnchor, Timestamp, TraceId};
