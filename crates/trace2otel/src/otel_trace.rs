//! Hand-written prost types matching the OpenTelemetry trace/v1 protobuf definitions.
//!
//! These correspond to `opentelemetry/proto/trace/v1/trace.proto`.

use crate::otel_common::{InstrumentationScope, KeyValue};
use crate::otel_resource::Resource;

/// TracesData represents the traces data that can be stored in a persistent storage,
/// or can be embedded by other protocols that transfer OTLP traces data.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct TracesData {
    #[prost(message, repeated, tag = "1")]
    pub resource_spans: ::prost::alloc::vec::Vec<ResourceSpans>,
}

/// A collection of ScopeSpans from a Resource.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ResourceSpans {
    #[prost(message, optional, tag = "1")]
    pub resource: ::core::option::Option<Resource>,
    #[prost(message, repeated, tag = "2")]
    pub scope_spans: ::prost::alloc::vec::Vec<ScopeSpans>,
    #[prost(string, tag = "3")]
    pub schema_url: ::prost::alloc::string::String,
}

/// A collection of Spans produced by an InstrumentationScope.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct ScopeSpans {
    #[prost(message, optional, tag = "1")]
    pub scope: ::core::option::Option<InstrumentationScope>,
    #[prost(message, repeated, tag = "2")]
    pub spans: ::prost::alloc::vec::Vec<Span>,
    #[prost(string, tag = "3")]
    pub schema_url: ::prost::alloc::string::String,
}

/// A Span represents a single operation performed by a single component of the system.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Span {
    /// A unique identifier for a trace (16 bytes).
    #[prost(bytes = "vec", tag = "1")]
    pub trace_id: ::prost::alloc::vec::Vec<u8>,
    /// A unique identifier for a span within a trace (8 bytes).
    #[prost(bytes = "vec", tag = "2")]
    pub span_id: ::prost::alloc::vec::Vec<u8>,
    /// W3C trace-context trace_state.
    #[prost(string, tag = "3")]
    pub trace_state: ::prost::alloc::string::String,
    /// The span_id of this span's parent span (8 bytes, empty if root).
    #[prost(bytes = "vec", tag = "4")]
    pub parent_span_id: ::prost::alloc::vec::Vec<u8>,
    /// Flags, a bit field (see SpanFlags).
    #[prost(fixed32, tag = "16")]
    pub flags: u32,
    /// A description of the span's operation.
    #[prost(string, tag = "5")]
    pub name: ::prost::alloc::string::String,
    /// Distinguishes between spans generated in a particular context.
    #[prost(enumeration = "span::SpanKind", tag = "6")]
    pub kind: i32,
    /// Start time in nanoseconds since Unix epoch.
    #[prost(fixed64, tag = "7")]
    pub start_time_unix_nano: u64,
    /// End time in nanoseconds since Unix epoch.
    #[prost(fixed64, tag = "8")]
    pub end_time_unix_nano: u64,
    /// Span attributes.
    #[prost(message, repeated, tag = "9")]
    pub attributes: ::prost::alloc::vec::Vec<KeyValue>,
    #[prost(uint32, tag = "10")]
    pub dropped_attributes_count: u32,
    /// Time-stamped events.
    #[prost(message, repeated, tag = "11")]
    pub events: ::prost::alloc::vec::Vec<span::Event>,
    #[prost(uint32, tag = "12")]
    pub dropped_events_count: u32,
    /// Links to other spans.
    #[prost(message, repeated, tag = "13")]
    pub links: ::prost::alloc::vec::Vec<span::Link>,
    #[prost(uint32, tag = "14")]
    pub dropped_links_count: u32,
    /// An optional final status for this span.
    #[prost(message, optional, tag = "15")]
    pub status: ::core::option::Option<Status>,
}

pub mod span {
    /// A time-stamped annotation of the span.
    #[derive(Clone, PartialEq, ::prost::Message)]
    pub struct Event {
        #[prost(fixed64, tag = "1")]
        pub time_unix_nano: u64,
        #[prost(string, tag = "2")]
        pub name: ::prost::alloc::string::String,
        #[prost(message, repeated, tag = "3")]
        pub attributes: ::prost::alloc::vec::Vec<crate::otel_common::KeyValue>,
        #[prost(uint32, tag = "4")]
        pub dropped_attributes_count: u32,
    }

    /// A pointer from the current span to another span.
    #[derive(Clone, PartialEq, ::prost::Message)]
    pub struct Link {
        #[prost(bytes = "vec", tag = "1")]
        pub trace_id: ::prost::alloc::vec::Vec<u8>,
        #[prost(bytes = "vec", tag = "2")]
        pub span_id: ::prost::alloc::vec::Vec<u8>,
        #[prost(string, tag = "3")]
        pub trace_state: ::prost::alloc::string::String,
        #[prost(message, repeated, tag = "4")]
        pub attributes: ::prost::alloc::vec::Vec<crate::otel_common::KeyValue>,
        #[prost(uint32, tag = "5")]
        pub dropped_attributes_count: u32,
        #[prost(fixed32, tag = "6")]
        pub flags: u32,
    }

    /// SpanKind is the type of span.
    #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
    #[repr(i32)]
    pub enum SpanKind {
        Unspecified = 0,
        Internal = 1,
        Server = 2,
        Client = 3,
        Producer = 4,
        Consumer = 5,
    }
}

/// The Status type defines a logical error model.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Status {
    /// A developer-facing human readable error message.
    #[prost(string, tag = "2")]
    pub message: ::prost::alloc::string::String,
    /// The status code.
    #[prost(enumeration = "status::StatusCode", tag = "3")]
    pub code: i32,
}

pub mod status {
    #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, ::prost::Enumeration)]
    #[repr(i32)]
    pub enum StatusCode {
        Unset = 0,
        Ok = 1,
        Error = 2,
    }
}
