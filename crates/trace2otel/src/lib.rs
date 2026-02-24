//! Converts Encore trace events to OpenTelemetry protobuf format.
//!
//! This crate reads a stream of binary trace events (using `encore-traceparser`)
//! and produces OpenTelemetry `TracesData` protobufs suitable for export to
//! any OTel-compatible backend (Jaeger, Grafana Tempo, etc.).
//!
//! # Usage
//!
//! ```no_run
//! use encore_traceparser::{TimeAnchor, Timestamp};
//! use encore_trace2otel::convert_trace_stream;
//!
//! let time_anchor = TimeAnchor {
//!     real: Timestamp { seconds: 1700000000, nanos: 0 },
//!     mono_nanos: 0,
//! };
//!
//! let data: &[u8] = &[/* trace bytes */];
//! let mut cursor = std::io::Cursor::new(data);
//!
//! let traces_data = convert_trace_stream(&mut cursor, &time_anchor, 17).unwrap();
//! // Serialize with prost: prost::Message::encode(&traces_data, &mut buf)
//! ```

pub mod otel_common;
pub mod otel_resource;
pub mod otel_trace;

pub mod collector;
pub mod convert;

pub use collector::SpanCollector;
pub use convert::convert_trace_stream;
