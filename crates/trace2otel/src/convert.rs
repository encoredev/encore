//! Conversion functions from Encore trace types to OpenTelemetry protobuf types.

use encore_traceparser::types::*;

use crate::otel_common::{any_value, AnyValue, InstrumentationScope, KeyValue};
use crate::otel_trace::{self, span, status, ResourceSpans, ScopeSpans, TracesData};

// ============================================================
// Public API
// ============================================================

/// Read all trace events from a stream and produce an OTel TracesData.
pub fn convert_trace_stream(
    reader: &mut impl std::io::Read,
    time_anchor: &TimeAnchor,
    version: u16,
) -> Result<TracesData, ParseError> {
    let mut collector = crate::SpanCollector::new();

    loop {
        match encore_traceparser::parse_event(reader, time_anchor, version) {
            Ok(event) => collector.add_event(event),
            Err(ParseError::EndOfStream) => break,
            Err(e) => return Err(e),
        }
    }

    Ok(collector.into_traces_data())
}

/// Convert a matched span start + end + events into a single OTel Span.
///
/// Returns `None` if the events don't contain valid SpanStart/SpanEnd data.
pub fn convert_span(
    start_event: &TraceEvent,
    end_event: &TraceEvent,
    span_events: &[TraceEvent],
) -> Option<otel_trace::Span> {
    let start = match &start_event.event {
        Event::SpanStart(s) => s,
        _ => return None,
    };
    let end = match &end_event.event {
        Event::SpanEnd(e) => e,
        _ => return None,
    };

    let start_nanos = timestamp_to_nanos(&start_event.event_time);
    let end_nanos = start_nanos + end.duration_nanos;

    let parent_span_id = start
        .parent_span_id
        .map(|id| id.to_le_bytes().to_vec())
        .unwrap_or_default();

    let name = span_name(&start.data, &end.data);
    let kind = span_kind(&start.data);

    let mut attributes = build_start_attributes(start);
    attributes.extend(build_end_attributes(end));

    let events: Vec<_> = span_events
        .iter()
        .filter_map(|e| {
            if let Event::SpanEvent(se) = &e.event {
                Some(convert_span_event(se, timestamp_to_nanos(&e.event_time)))
            } else {
                None
            }
        })
        .collect();

    let otel_status = convert_status(end.status_code, &end.error);

    Some(otel_trace::Span {
        trace_id: trace_id_to_bytes(&start_event.trace_id),
        span_id: span_id_to_bytes(start_event.span_id),
        trace_state: String::new(),
        parent_span_id,
        flags: 0,
        name,
        kind: kind as i32,
        start_time_unix_nano: start_nanos,
        end_time_unix_nano: end_nanos,
        attributes,
        dropped_attributes_count: 0,
        events,
        dropped_events_count: 0,
        links: vec![],
        dropped_links_count: 0,
        status: Some(otel_status),
    })
}

/// Wrap a list of completed OTel spans into a TracesData envelope.
pub fn wrap_traces_data(spans: Vec<otel_trace::Span>) -> TracesData {
    TracesData {
        resource_spans: vec![ResourceSpans {
            resource: None,
            scope_spans: vec![ScopeSpans {
                scope: Some(InstrumentationScope {
                    name: "encore".to_string(),
                    version: String::new(),
                    attributes: vec![],
                    dropped_attributes_count: 0,
                }),
                spans,
                schema_url: String::new(),
            }],
            schema_url: String::new(),
        }],
    }
}

// ============================================================
// Type conversion helpers
// ============================================================

/// Convert an Encore TraceId to 16-byte OTel trace_id.
pub fn trace_id_to_bytes(id: &TraceId) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(16);
    bytes.extend_from_slice(&id.low.to_le_bytes());
    bytes.extend_from_slice(&id.high.to_le_bytes());
    bytes
}

/// Convert a span_id u64 to 8-byte OTel span_id.
pub fn span_id_to_bytes(id: u64) -> Vec<u8> {
    id.to_le_bytes().to_vec()
}

/// Convert an Encore Timestamp to nanoseconds since Unix epoch.
pub fn timestamp_to_nanos(ts: &Timestamp) -> u64 {
    if ts.seconds < 0 {
        return 0;
    }
    ts.seconds as u64 * 1_000_000_000 + ts.nanos as u64
}

// ============================================================
// Attribute builders
// ============================================================

fn str_attr(key: &str, value: &str) -> KeyValue {
    KeyValue {
        key: key.to_string(),
        value: Some(AnyValue {
            value: Some(any_value::Value::StringValue(value.to_string())),
        }),
    }
}

fn int_attr(key: &str, value: i64) -> KeyValue {
    KeyValue {
        key: key.to_string(),
        value: Some(AnyValue {
            value: Some(any_value::Value::IntValue(value)),
        }),
    }
}

fn bool_attr(key: &str, value: bool) -> KeyValue {
    KeyValue {
        key: key.to_string(),
        value: Some(AnyValue {
            value: Some(any_value::Value::BoolValue(value)),
        }),
    }
}

// ============================================================
// Span naming and kind
// ============================================================

fn span_name(start_data: &SpanStartData, end_data: &SpanEndData) -> String {
    match start_data {
        SpanStartData::Request(r) => format!("{}.{}", r.service_name, r.endpoint_name),
        SpanStartData::Auth(a) => format!("auth:{}.{}", a.service_name, a.endpoint_name),
        SpanStartData::PubsubMessage(p) => {
            format!("{}/{}", p.topic_name, p.subscription_name)
        }
        SpanStartData::Test(t) => {
            // Include service name from end if available for richer context.
            let svc = match end_data {
                SpanEndData::Test(te) if !te.service_name.is_empty() => &te.service_name,
                _ => &t.service_name,
            };
            if svc.is_empty() {
                format!("test:{}", t.test_name)
            } else {
                format!("test:{}.{}", svc, t.test_name)
            }
        }
    }
}

fn span_kind(data: &SpanStartData) -> span::SpanKind {
    match data {
        SpanStartData::Request(_) => span::SpanKind::Server,
        SpanStartData::Auth(_) => span::SpanKind::Internal,
        SpanStartData::PubsubMessage(_) => span::SpanKind::Consumer,
        SpanStartData::Test(_) => span::SpanKind::Internal,
    }
}

// ============================================================
// Span attributes from start/end data
// ============================================================

fn build_start_attributes(start: &SpanStart) -> Vec<KeyValue> {
    let mut attrs = Vec::new();

    if let Some(ref ext_id) = start.external_correlation_id {
        attrs.push(str_attr("encore.correlation_id", ext_id));
    }

    match &start.data {
        SpanStartData::Request(r) => {
            attrs.push(str_attr("service.name", &r.service_name));
            attrs.push(str_attr("rpc.system", "encore"));
            attrs.push(str_attr("rpc.service", &r.service_name));
            attrs.push(str_attr("rpc.method", &r.endpoint_name));
            if !r.http_method.is_empty() {
                attrs.push(str_attr("http.request.method", &r.http_method));
            }
            if !r.path.is_empty() {
                attrs.push(str_attr("url.path", &r.path));
            }
            if r.mocked {
                attrs.push(bool_attr("encore.request.mocked", true));
            }
            if let Some(ref uid) = r.uid {
                attrs.push(str_attr("enduser.id", uid));
            }
        }
        SpanStartData::Auth(a) => {
            attrs.push(str_attr("service.name", &a.service_name));
            attrs.push(str_attr("rpc.system", "encore"));
            attrs.push(str_attr("rpc.service", &a.service_name));
            attrs.push(str_attr("rpc.method", &a.endpoint_name));
        }
        SpanStartData::PubsubMessage(p) => {
            attrs.push(str_attr("service.name", &p.service_name));
            attrs.push(str_attr("messaging.system", "encore"));
            attrs.push(str_attr("messaging.destination.name", &p.topic_name));
            if !p.subscription_name.is_empty() {
                attrs.push(str_attr(
                    "messaging.consumer.group.name",
                    &p.subscription_name,
                ));
            }
            if !p.message_id.is_empty() {
                attrs.push(str_attr("messaging.message.id", &p.message_id));
            }
            if p.attempt > 0 {
                attrs.push(int_attr(
                    "messaging.operation.retry.count",
                    (p.attempt - 1) as i64,
                ));
            }
        }
        SpanStartData::Test(t) => {
            if !t.service_name.is_empty() {
                attrs.push(str_attr("service.name", &t.service_name));
            }
            attrs.push(str_attr("encore.test.name", &t.test_name));
            if !t.test_file.is_empty() {
                attrs.push(str_attr("code.filepath", &t.test_file));
            }
            if t.test_line > 0 {
                attrs.push(int_attr("code.lineno", t.test_line as i64));
            }
        }
    }

    attrs
}

fn build_end_attributes(end: &SpanEnd) -> Vec<KeyValue> {
    let mut attrs = Vec::new();

    // Add Encore-specific status code as attribute for detailed inspection.
    if end.status_code != StatusCode::Ok && end.status_code != StatusCode::Unknown {
        attrs.push(str_attr(
            "encore.status_code",
            status_code_name(end.status_code),
        ));
    }

    match &end.data {
        SpanEndData::Request(r) => {
            if r.http_status_code > 0 {
                attrs.push(int_attr(
                    "http.response.status_code",
                    r.http_status_code as i64,
                ));
            }
            if let Some(ref uid) = r.uid {
                attrs.push(str_attr("enduser.id", uid));
            }
        }
        SpanEndData::Auth(a) => {
            if !a.uid.is_empty() {
                attrs.push(str_attr("enduser.id", &a.uid));
            }
        }
        SpanEndData::PubsubMessage(p) => {
            if !p.message_id.is_empty() {
                attrs.push(str_attr("messaging.message.id", &p.message_id));
            }
        }
        SpanEndData::Test(t) => {
            if t.failed {
                attrs.push(bool_attr("encore.test.failed", true));
            }
            if t.skipped {
                attrs.push(bool_attr("encore.test.skipped", true));
            }
        }
    }

    attrs
}

// ============================================================
// Status conversion
// ============================================================

fn convert_status(code: StatusCode, error: &Option<TracedError>) -> otel_trace::Status {
    if let Some(err) = error {
        return otel_trace::Status {
            code: status::StatusCode::Error as i32,
            message: err.msg.clone(),
        };
    }

    match code {
        StatusCode::Ok => otel_trace::Status {
            code: status::StatusCode::Ok as i32,
            message: String::new(),
        },
        StatusCode::Unknown => otel_trace::Status {
            code: status::StatusCode::Unset as i32,
            message: String::new(),
        },
        _ => otel_trace::Status {
            code: status::StatusCode::Error as i32,
            message: format!("{:?}", code),
        },
    }
}

fn status_code_name(code: StatusCode) -> &'static str {
    match code {
        StatusCode::Ok => "OK",
        StatusCode::Canceled => "CANCELED",
        StatusCode::Unknown => "UNKNOWN",
        StatusCode::InvalidArgument => "INVALID_ARGUMENT",
        StatusCode::DeadlineExceeded => "DEADLINE_EXCEEDED",
        StatusCode::NotFound => "NOT_FOUND",
        StatusCode::AlreadyExists => "ALREADY_EXISTS",
        StatusCode::PermissionDenied => "PERMISSION_DENIED",
        StatusCode::ResourceExhausted => "RESOURCE_EXHAUSTED",
        StatusCode::FailedPrecondition => "FAILED_PRECONDITION",
        StatusCode::Aborted => "ABORTED",
        StatusCode::OutOfRange => "OUT_OF_RANGE",
        StatusCode::Unimplemented => "UNIMPLEMENTED",
        StatusCode::Internal => "INTERNAL",
        StatusCode::Unavailable => "UNAVAILABLE",
        StatusCode::DataLoss => "DATA_LOSS",
        StatusCode::Unauthenticated => "UNAUTHENTICATED",
    }
}

// ============================================================
// SpanEvent → OTel Event conversion
// ============================================================

fn convert_span_event(event: &SpanEvent, time_nanos: u64) -> span::Event {
    let (name, attrs) = span_event_name_and_attrs(&event.data);
    span::Event {
        time_unix_nano: time_nanos,
        name,
        attributes: attrs,
        dropped_attributes_count: 0,
    }
}

fn span_event_name_and_attrs(data: &SpanEventData) -> (String, Vec<KeyValue>) {
    match data {
        SpanEventData::LogMessage(log) => {
            let mut attrs = vec![
                str_attr("log.severity", log_level_str(log.level)),
                str_attr("log.message", &log.msg),
            ];
            for field in &log.fields {
                if let Some(attr) = log_field_to_attr(field) {
                    attrs.push(attr);
                }
            }
            ("log".to_string(), attrs)
        }

        SpanEventData::RpcCallStart(rpc) => {
            let attrs = vec![
                str_attr("rpc.system", "encore"),
                str_attr("rpc.service", &rpc.target_service_name),
                str_attr("rpc.method", &rpc.target_endpoint_name),
            ];
            ("encore.rpc.call.start".to_string(), attrs)
        }
        SpanEventData::RpcCallEnd(rpc) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = rpc.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.rpc.call.end".to_string(), attrs)
        }

        SpanEventData::DbQueryStart(db) => {
            let attrs = vec![str_attr("db.query.text", &db.query)];
            ("encore.db.query.start".to_string(), attrs)
        }
        SpanEventData::DbQueryEnd(db) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = db.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.db.query.end".to_string(), attrs)
        }

        SpanEventData::DbTransactionStart(_) => {
            ("encore.db.tx.start".to_string(), vec![])
        }
        SpanEventData::DbTransactionEnd(tx) => {
            let mut attrs = vec![str_attr(
                "db.operation",
                match tx.completion {
                    DbTransactionCompletion::Commit => "COMMIT",
                    DbTransactionCompletion::Rollback => "ROLLBACK",
                },
            )];
            if let Some(ref err) = tx.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.db.tx.end".to_string(), attrs)
        }

        SpanEventData::HttpCallStart(http) => {
            let attrs = vec![
                str_attr("http.request.method", &http.method),
                str_attr("url.full", &http.url),
            ];
            ("encore.http.call.start".to_string(), attrs)
        }
        SpanEventData::HttpCallEnd(http) => {
            let mut attrs = Vec::new();
            if let Some(code) = http.status_code {
                attrs.push(int_attr("http.response.status_code", code as i64));
            }
            if let Some(ref err) = http.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.http.call.end".to_string(), attrs)
        }

        SpanEventData::PubsubPublishStart(pub_ev) => {
            let attrs = vec![
                str_attr("messaging.system", "encore"),
                str_attr("messaging.destination.name", &pub_ev.topic),
            ];
            ("encore.pubsub.publish.start".to_string(), attrs)
        }
        SpanEventData::PubsubPublishEnd(pub_ev) => {
            let mut attrs = Vec::new();
            if let Some(ref id) = pub_ev.message_id {
                attrs.push(str_attr("messaging.message.id", id));
            }
            if let Some(ref err) = pub_ev.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.pubsub.publish.end".to_string(), attrs)
        }

        SpanEventData::ServiceInitStart(svc) => {
            let attrs = vec![str_attr("service.name", &svc.service)];
            ("encore.service.init.start".to_string(), attrs)
        }
        SpanEventData::ServiceInitEnd(svc) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = svc.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.service.init.end".to_string(), attrs)
        }

        SpanEventData::CacheCallStart(cache) => {
            let mut attrs = vec![str_attr("db.system", "cache")];
            attrs.push(str_attr("db.operation.name", &cache.operation));
            if cache.write {
                attrs.push(bool_attr("encore.cache.write", true));
            }
            ("encore.cache.call.start".to_string(), attrs)
        }
        SpanEventData::CacheCallEnd(cache) => {
            let mut attrs = vec![str_attr(
                "encore.cache.result",
                match cache.result {
                    CacheResult::Ok => "OK",
                    CacheResult::NoSuchKey => "NO_SUCH_KEY",
                    CacheResult::Conflict => "CONFLICT",
                    CacheResult::Err => "ERR",
                    CacheResult::Unknown => "UNKNOWN",
                },
            )];
            if let Some(ref err) = cache.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.cache.call.end".to_string(), attrs)
        }

        SpanEventData::BodyStream(bs) => {
            let mut attrs = Vec::new();
            if bs.is_response {
                attrs.push(bool_attr("encore.body.is_response", true));
            }
            if bs.overflowed {
                attrs.push(bool_attr("encore.body.overflowed", true));
            }
            ("encore.body.stream".to_string(), attrs)
        }

        // Bucket operations
        SpanEventData::BucketObjectUploadStart(b) => {
            let attrs = vec![
                str_attr("encore.bucket.name", &b.bucket),
                str_attr("encore.bucket.object", &b.object),
            ];
            ("encore.bucket.upload.start".to_string(), attrs)
        }
        SpanEventData::BucketObjectUploadEnd(b) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = b.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.bucket.upload.end".to_string(), attrs)
        }
        SpanEventData::BucketObjectDownloadStart(b) => {
            let attrs = vec![
                str_attr("encore.bucket.name", &b.bucket),
                str_attr("encore.bucket.object", &b.object),
            ];
            ("encore.bucket.download.start".to_string(), attrs)
        }
        SpanEventData::BucketObjectDownloadEnd(b) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = b.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.bucket.download.end".to_string(), attrs)
        }
        SpanEventData::BucketObjectGetAttrsStart(b) => {
            let attrs = vec![
                str_attr("encore.bucket.name", &b.bucket),
                str_attr("encore.bucket.object", &b.object),
            ];
            ("encore.bucket.getattrs.start".to_string(), attrs)
        }
        SpanEventData::BucketObjectGetAttrsEnd(b) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = b.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.bucket.getattrs.end".to_string(), attrs)
        }
        SpanEventData::BucketListObjectsStart(b) => {
            let attrs = vec![str_attr("encore.bucket.name", &b.bucket)];
            ("encore.bucket.list.start".to_string(), attrs)
        }
        SpanEventData::BucketListObjectsEnd(b) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = b.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.bucket.list.end".to_string(), attrs)
        }
        SpanEventData::BucketDeleteObjectsStart(b) => {
            let attrs = vec![str_attr("encore.bucket.name", &b.bucket)];
            ("encore.bucket.delete.start".to_string(), attrs)
        }
        SpanEventData::BucketDeleteObjectsEnd(b) => {
            let mut attrs = Vec::new();
            if let Some(ref err) = b.err {
                attrs.push(str_attr("error.message", &err.msg));
            }
            ("encore.bucket.delete.end".to_string(), attrs)
        }
    }
}

fn log_level_str(level: LogLevel) -> &'static str {
    match level {
        LogLevel::Trace => "TRACE",
        LogLevel::Debug => "DEBUG",
        LogLevel::Info => "INFO",
        LogLevel::Warn => "WARN",
        LogLevel::Error => "ERROR",
    }
}

fn log_field_to_attr(field: &LogField) -> Option<KeyValue> {
    let value = match &field.value {
        LogFieldValue::Str(s) => any_value::Value::StringValue(s.clone()),
        LogFieldValue::Bool(b) => any_value::Value::BoolValue(*b),
        LogFieldValue::Int(i) => any_value::Value::IntValue(*i),
        LogFieldValue::Uint(u) => any_value::Value::IntValue(*u as i64),
        LogFieldValue::Float32(f) => any_value::Value::DoubleValue(*f as f64),
        LogFieldValue::Float64(f) => any_value::Value::DoubleValue(*f),
        LogFieldValue::Duration(d) => any_value::Value::IntValue(*d),
        LogFieldValue::Error(err) => any_value::Value::StringValue(err.msg.clone()),
        LogFieldValue::Json(bytes) => {
            any_value::Value::StringValue(String::from_utf8_lossy(bytes).into_owned())
        }
        LogFieldValue::Uuid(bytes) => {
            any_value::Value::BytesValue(bytes.clone())
        }
        LogFieldValue::Time(ts) => {
            any_value::Value::IntValue(timestamp_to_nanos(ts) as i64)
        }
    };

    Some(KeyValue {
        key: field.key.clone(),
        value: Some(AnyValue { value: Some(value) }),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_trace_id_to_bytes() {
        let id = TraceId { low: 1, high: 2 };
        let bytes = trace_id_to_bytes(&id);
        assert_eq!(bytes.len(), 16);
        assert_eq!(&bytes[0..8], &1u64.to_le_bytes());
        assert_eq!(&bytes[8..16], &2u64.to_le_bytes());
    }

    #[test]
    fn test_span_id_to_bytes() {
        let bytes = span_id_to_bytes(42);
        assert_eq!(bytes.len(), 8);
        assert_eq!(bytes, 42u64.to_le_bytes());
    }

    #[test]
    fn test_timestamp_to_nanos() {
        let ts = Timestamp {
            seconds: 1,
            nanos: 500,
        };
        assert_eq!(timestamp_to_nanos(&ts), 1_000_000_500);
    }

    #[test]
    fn test_timestamp_to_nanos_negative() {
        let ts = Timestamp {
            seconds: -1,
            nanos: 0,
        };
        assert_eq!(timestamp_to_nanos(&ts), 0);
    }

    #[test]
    fn test_convert_status_ok() {
        let status = convert_status(StatusCode::Ok, &None);
        assert_eq!(status.code, status::StatusCode::Ok as i32);
        assert!(status.message.is_empty());
    }

    #[test]
    fn test_convert_status_error() {
        let err = Some(TracedError {
            msg: "something failed".to_string(),
            stack: None,
        });
        let status = convert_status(StatusCode::Internal, &err);
        assert_eq!(status.code, status::StatusCode::Error as i32);
        assert_eq!(status.message, "something failed");
    }

    #[test]
    fn test_convert_status_unknown_no_error() {
        let status = convert_status(StatusCode::Unknown, &None);
        assert_eq!(status.code, status::StatusCode::Unset as i32);
    }

    #[test]
    fn test_span_name_request() {
        let start = SpanStartData::Request(RequestSpanStart {
            service_name: "myservice".to_string(),
            endpoint_name: "MyEndpoint".to_string(),
            http_method: String::new(),
            path: String::new(),
            path_params: vec![],
            request_headers: Default::default(),
            request_payload: vec![],
            ext_correlation_id: None,
            uid: None,
            mocked: false,
        });
        let end = SpanEndData::Request(RequestSpanEnd {
            service_name: "myservice".to_string(),
            endpoint_name: "MyEndpoint".to_string(),
            http_status_code: 200,
            response_headers: Default::default(),
            response_payload: vec![],
            caller_event_id: None,
            uid: None,
        });
        assert_eq!(span_name(&start, &end), "myservice.MyEndpoint");
    }

    #[test]
    fn test_span_kind_mapping() {
        assert_eq!(
            span_kind(&SpanStartData::Request(RequestSpanStart {
                service_name: String::new(),
                endpoint_name: String::new(),
                http_method: String::new(),
                path: String::new(),
                path_params: vec![],
                request_headers: Default::default(),
                request_payload: vec![],
                ext_correlation_id: None,
                uid: None,
                mocked: false,
            })),
            span::SpanKind::Server
        );
    }

    #[test]
    fn test_convert_span_roundtrip() {
        let start_event = TraceEvent {
            trace_id: TraceId { low: 1, high: 2 },
            span_id: 100,
            event_id: 1,
            event_time: Timestamp {
                seconds: 1700000000,
                nanos: 0,
            },
            event: Event::SpanStart(SpanStart {
                goid: 1,
                parent_trace_id: None,
                parent_span_id: None,
                def_loc: None,
                caller_event_id: None,
                external_correlation_id: None,
                data: SpanStartData::Request(RequestSpanStart {
                    service_name: "svc".to_string(),
                    endpoint_name: "Ep".to_string(),
                    http_method: "POST".to_string(),
                    path: "/api/ep".to_string(),
                    path_params: vec![],
                    request_headers: Default::default(),
                    request_payload: vec![],
                    ext_correlation_id: None,
                    uid: None,
                    mocked: false,
                }),
            }),
        };

        let end_event = TraceEvent {
            trace_id: TraceId { low: 1, high: 2 },
            span_id: 100,
            event_id: 2,
            event_time: Timestamp {
                seconds: 1700000001,
                nanos: 0,
            },
            event: Event::SpanEnd(SpanEnd {
                duration_nanos: 1_000_000_000,
                status_code: StatusCode::Ok,
                error: None,
                panic_stack: None,
                parent_trace_id: None,
                parent_span_id: None,
                data: SpanEndData::Request(RequestSpanEnd {
                    service_name: "svc".to_string(),
                    endpoint_name: "Ep".to_string(),
                    http_status_code: 200,
                    response_headers: Default::default(),
                    response_payload: vec![],
                    caller_event_id: None,
                    uid: None,
                }),
            }),
        };

        let span = convert_span(&start_event, &end_event, &[]).unwrap();

        assert_eq!(span.name, "svc.Ep");
        assert_eq!(span.kind, span::SpanKind::Server as i32);
        assert_eq!(
            span.start_time_unix_nano,
            1_700_000_000 * 1_000_000_000
        );
        assert_eq!(
            span.end_time_unix_nano,
            1_700_000_001 * 1_000_000_000
        );
        assert_eq!(span.status.as_ref().unwrap().code, status::StatusCode::Ok as i32);

        // Check attributes
        let attr_keys: Vec<_> = span.attributes.iter().map(|a| a.key.as_str()).collect();
        assert!(attr_keys.contains(&"service.name"));
        assert!(attr_keys.contains(&"rpc.system"));
        assert!(attr_keys.contains(&"http.request.method"));
        assert!(attr_keys.contains(&"url.path"));
        assert!(attr_keys.contains(&"http.response.status_code"));
    }
}
