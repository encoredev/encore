//! Stateful collector that buffers trace events and produces complete OTel spans.

use std::collections::HashMap;

use encore_traceparser::types::{Event, TraceEvent};

use crate::convert;
use crate::otel_trace;

/// A key that uniquely identifies a span within a trace stream.
#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct SpanKey {
    trace_id_low: u64,
    trace_id_high: u64,
    span_id: u64,
}

impl SpanKey {
    fn from_event(event: &TraceEvent) -> Self {
        SpanKey {
            trace_id_low: event.trace_id.low,
            trace_id_high: event.trace_id.high,
            span_id: event.span_id,
        }
    }
}

/// Collects trace events and produces complete OpenTelemetry spans.
///
/// Events are buffered until a matching span start and end are received,
/// at which point a complete OTel span is produced.
///
/// # Example
///
/// ```no_run
/// use encore_trace2otel::SpanCollector;
///
/// let mut collector = SpanCollector::new();
///
/// // Feed trace events as they arrive
/// // collector.add_event(event);
///
/// // Retrieve completed spans
/// let spans = collector.take_completed();
///
/// // Or convert everything to TracesData
/// let traces_data = collector.into_traces_data();
/// ```
pub struct SpanCollector {
    /// Pending span starts, waiting for their corresponding end events.
    pending_starts: HashMap<SpanKey, TraceEvent>,
    /// Span events buffered for each pending span.
    pending_events: HashMap<SpanKey, Vec<TraceEvent>>,
    /// Completed OTel spans ready for consumption.
    completed: Vec<otel_trace::Span>,
}

impl SpanCollector {
    pub fn new() -> Self {
        SpanCollector {
            pending_starts: HashMap::new(),
            pending_events: HashMap::new(),
            completed: Vec::new(),
        }
    }

    /// Add a trace event to the collector.
    ///
    /// When a SpanEnd is received that matches a buffered SpanStart,
    /// a complete OTel Span is produced and added to the completed list.
    pub fn add_event(&mut self, event: TraceEvent) {
        let key = SpanKey::from_event(&event);

        match &event.event {
            Event::SpanStart(_) => {
                self.pending_starts.insert(key, event);
            }
            Event::SpanEnd(_) => {
                let start = self.pending_starts.remove(&key);
                let events = self.pending_events.remove(&key).unwrap_or_default();

                if let Some(start_event) = start {
                    if let Some(span) = convert::convert_span(&start_event, &event, &events) {
                        self.completed.push(span);
                    }
                }
            }
            Event::SpanEvent(_) => {
                self.pending_events.entry(key).or_default().push(event);
            }
        }
    }

    /// Take all completed spans, leaving the collector empty.
    pub fn take_completed(&mut self) -> Vec<otel_trace::Span> {
        std::mem::take(&mut self.completed)
    }

    /// Consume the collector and return all completed spans as an OTel TracesData.
    pub fn into_traces_data(self) -> otel_trace::TracesData {
        convert::wrap_traces_data(self.completed)
    }
}

impl Default for SpanCollector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use encore_traceparser::types::*;

    fn make_start_event(trace_low: u64, span_id: u64) -> TraceEvent {
        TraceEvent {
            trace_id: TraceId {
                low: trace_low,
                high: 0,
            },
            span_id,
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
                    http_method: "GET".to_string(),
                    path: "/".to_string(),
                    path_params: vec![],
                    request_headers: Default::default(),
                    request_payload: vec![],
                    ext_correlation_id: None,
                    uid: None,
                    mocked: false,
                }),
            }),
        }
    }

    fn make_end_event(trace_low: u64, span_id: u64) -> TraceEvent {
        TraceEvent {
            trace_id: TraceId {
                low: trace_low,
                high: 0,
            },
            span_id,
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
        }
    }

    fn make_span_event(trace_low: u64, span_id: u64) -> TraceEvent {
        TraceEvent {
            trace_id: TraceId {
                low: trace_low,
                high: 0,
            },
            span_id,
            event_id: 3,
            event_time: Timestamp {
                seconds: 1700000000,
                nanos: 500_000_000,
            },
            event: Event::SpanEvent(SpanEvent {
                goid: 1,
                def_loc: None,
                correlation_event_id: None,
                data: SpanEventData::LogMessage(LogMessage {
                    level: LogLevel::Info,
                    msg: "hello".to_string(),
                    fields: vec![],
                    stack: None,
                }),
            }),
        }
    }

    #[test]
    fn test_collector_basic() {
        let mut c = SpanCollector::new();
        c.add_event(make_start_event(1, 100));
        assert!(c.take_completed().is_empty());

        c.add_event(make_end_event(1, 100));
        let spans = c.take_completed();
        assert_eq!(spans.len(), 1);
        assert_eq!(spans[0].name, "svc.Ep");
    }

    #[test]
    fn test_collector_with_events() {
        let mut c = SpanCollector::new();
        c.add_event(make_start_event(1, 100));
        c.add_event(make_span_event(1, 100));
        c.add_event(make_end_event(1, 100));

        let spans = c.take_completed();
        assert_eq!(spans.len(), 1);
        assert_eq!(spans[0].events.len(), 1);
        assert_eq!(spans[0].events[0].name, "log");
    }

    #[test]
    fn test_collector_multiple_spans() {
        let mut c = SpanCollector::new();
        c.add_event(make_start_event(1, 100));
        c.add_event(make_start_event(1, 200));
        c.add_event(make_end_event(1, 100));
        c.add_event(make_end_event(1, 200));

        let spans = c.take_completed();
        assert_eq!(spans.len(), 2);
    }

    #[test]
    fn test_collector_end_without_start() {
        let mut c = SpanCollector::new();
        // End without a matching start should be silently dropped.
        c.add_event(make_end_event(1, 999));
        assert!(c.take_completed().is_empty());
    }

    #[test]
    fn test_into_traces_data() {
        let mut c = SpanCollector::new();
        c.add_event(make_start_event(1, 100));
        c.add_event(make_end_event(1, 100));

        let data = c.into_traces_data();
        assert_eq!(data.resource_spans.len(), 1);
        assert_eq!(data.resource_spans[0].scope_spans.len(), 1);
        assert_eq!(data.resource_spans[0].scope_spans[0].spans.len(), 1);

        let scope = data.resource_spans[0].scope_spans[0]
            .scope
            .as_ref()
            .unwrap();
        assert_eq!(scope.name, "encore");
    }
}
