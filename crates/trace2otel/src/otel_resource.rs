//! Hand-written prost types matching the OpenTelemetry resource/v1 protobuf definitions.
//!
//! These correspond to `opentelemetry/proto/resource/v1/resource.proto`.

use crate::otel_common::KeyValue;

/// Resource information.
#[derive(Clone, PartialEq, ::prost::Message)]
pub struct Resource {
    #[prost(message, repeated, tag = "1")]
    pub attributes: ::prost::alloc::vec::Vec<KeyValue>,
    #[prost(uint32, tag = "2")]
    pub dropped_attributes_count: u32,
}
