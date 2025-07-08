use std::sync::Arc;

use crate::api::reqauth::platform;
use bytes::Bytes;
use futures::StreamExt;
use tokio::sync::mpsc::UnboundedReceiver;
use tokio_stream::wrappers::UnboundedReceiverStream;

use crate::model;
use crate::trace::eventbuf::signed_to_unsigned_i64;
use crate::trace::protocol::{EventType, TRACE_VERSION};
use crate::trace::time_anchor::TimeAnchor;
use crate::trace::Tracer;

pub struct ReporterConfig {
    pub app_id: String,
    pub env_id: String,
    pub deploy_id: String,
    pub app_commit: String,
    pub trace_endpoint: reqwest::Url,
    pub platform_validator: Arc<platform::RequestValidator>,
}

/// Sends traces to the trace server.
#[must_use]
pub struct Reporter {
    rx: tokio::sync::mpsc::UnboundedReceiver<TraceEvent>,
    anchor: TimeAnchor,
    http_client: reqwest::Client,
    config: ReporterConfig,
}

pub fn streaming_tracer(
    http_client: reqwest::Client,
    config: ReporterConfig,
) -> (Tracer, Reporter) {
    let (tx, rx) = tokio::sync::mpsc::unbounded_channel();
    let tracer = Tracer::new(tx);

    let anchor = TimeAnchor::new();
    let reporter = Reporter {
        rx,
        anchor,
        http_client,
        config,
    };
    (tracer, reporter)
}

#[derive(Debug)]
pub(super) struct TraceEvent {
    pub typ: EventType,
    pub id: model::TraceEventId,
    pub data: Bytes,
    pub span: model::SpanKey,
    pub ts: tokio::time::Instant,
}

impl Reporter {
    /// Starts reporting trace events to the trace server.
    ///
    /// This method runs in an infinite loop until all senders are dropped,
    /// continuously collecting trace events and sending them to the trace server.
    pub async fn start_reporting(mut self) {
        let trace_headers = match self.create_trace_headers(self.anchor.trace_header().as_str()) {
            Ok(trace_headers) => trace_headers,
            Err(err) => {
                log::error!("couldn't setup headers for tracing requests, exiting. Error: {err}");
                return;
            }
        };

        loop {
            let mut body_sender = None;

            let timeout_duration = std::time::Duration::from_millis(1000);
            let mut no_data_timeout = Box::pin(tokio::time::sleep(timeout_duration));

            loop {
                tokio::select! {
                    event = self.rx.recv() => {
                        match event {
                            Some(event) => {
                                // Wait for at least one entry on rx before we open a HTTP request.
                                if body_sender.is_none() {
                                    match self.setup_trace_request(&trace_headers) {
                                        Ok(sender) => body_sender = Some(sender),
                                        Err(err) => {
                                            log::error!("failed to create request: {err}");
                                            break;
                                        }
                                    }
                                }

                                // Add the event to the stream
                                if let Some(sender) = &body_sender {
                                    if let Err(err) = sender.send(event) {
                                        log::error!("failed to stream event: {err}");
                                        break;
                                    }
                                }

                                // Reset the timeout
                                no_data_timeout = Box::pin(tokio::time::sleep(timeout_duration));
                            }
                            None => {
                                // The stream is closed. This only happens if all senders have been dropped,
                                // which should never happen in regular use.
                                return;
                            }
                        }
                    }
                    _ = &mut no_data_timeout => {
                        // Timeout reached with no new events
                        if let Some(sender) = body_sender {
                            // Close the stream and wait for a new event
                            drop(sender);
                        }
                        break;
                    }
                }
            }
        }
    }

    fn create_body_stream(
        &self,
        rx: UnboundedReceiver<TraceEvent>,
    ) -> impl futures::Stream<Item = Result<Bytes, std::io::Error>> {
        let anchor = self.anchor.clone();
        UnboundedReceiverStream::new(rx).flat_map(move |event| {
            let streaming_event = StreamingTraceEvent { event };
            futures::stream::iter(vec![
                Ok::<_, std::io::Error>(streaming_event.header(&anchor)),
                Ok::<_, std::io::Error>(streaming_event.event.data),
            ])
        })
    }

    fn setup_trace_request(
        &self,
        trace_headers: &reqwest::header::HeaderMap,
    ) -> Result<tokio::sync::mpsc::UnboundedSender<TraceEvent>, String> {
        {
            let http_client: &reqwest::Client = &self.http_client;
            let endpoint: &reqwest::Url = &self.config.trace_endpoint;
            let validator: &Arc<platform::RequestValidator> = &self.config.platform_validator;
            // Create a channel for the streaming body
            let (tx, rx) = tokio::sync::mpsc::unbounded_channel::<TraceEvent>();

            let mut req = match http_client
                .post(endpoint.clone())
                .headers(trace_headers.clone())
                .body(reqwest::Body::wrap_stream(self.create_body_stream(rx)))
                .build()
            {
                Ok(req) => req,
                Err(err) => {
                    return Err(format!("failed to build trace request: {err:?}"));
                }
            };

            if let Err(err) = validator.sign_outgoing_request(&mut req) {
                return Err(format!("failed to sign trace request: {err:?}"));
            }

            // Start the request
            let request = http_client.execute(req);
            tokio::spawn(async move {
                match request.await {
                    Ok(resp) if !resp.status().is_success() => {
                        let status = resp.status();
                        let body = resp.text().await.unwrap_or_else(|_| String::new());
                        log::error!("failed to send trace: HTTP {}: {}", status, body);
                    }
                    Err(err) => {
                        log::error!("failed to send trace: {}", err);
                    }
                    _ => {}
                }
            });

            Ok(tx)
        }
    }

    fn create_trace_headers(
        &self,
        trace_time_anchor: &str,
    ) -> Result<reqwest::header::HeaderMap, reqwest::header::InvalidHeaderValue> {
        use reqwest::header::*;
        let mut headers = HeaderMap::new();

        headers.insert(
            "X-Encore-App-Id",
            HeaderValue::from_str(&self.config.app_id)?,
        );
        headers.insert(
            "X-Encore-Env-Id",
            HeaderValue::from_str(&self.config.env_id)?,
        );
        headers.insert(
            "X-Encore-Deploy-Id",
            HeaderValue::from_str(&self.config.deploy_id)?,
        );
        headers.insert(
            "X-Encore-App-Commit",
            HeaderValue::from_str(&self.config.app_commit)?,
        );
        headers.insert("X-Encore-Trace-Version", HeaderValue::from(TRACE_VERSION));
        headers.insert(
            "X-Encore-Trace-TimeAnchor",
            HeaderValue::from_str(trace_time_anchor)?,
        );

        Ok(headers)
    }
}

/// Represents a trace event that is being streamed.
#[derive(Debug)]
struct StreamingTraceEvent {
    /// The event itself.
    event: TraceEvent,
}

impl StreamingTraceEvent {
    fn header(&self, anchor: &TimeAnchor) -> Bytes {
        let event_type = self.event.typ;
        let event_id = self.event.id.0;
        let trace_id = &self.event.span.0 .0;
        let span_id = &self.event.span.1 .0;
        let ln = self.event.data.len();

        // Compute the timestamp, relative to the anchor's timestamp.
        let ts = self
            .event
            .ts
            .saturating_duration_since(anchor.instant)
            .as_nanos() as i64;
        let ts = signed_to_unsigned_i64(ts);

        Bytes::from(vec![
            // Event type, 1 byte
            event_type as u8,
            // Event ID, 8 bytes
            event_id as u8,
            (event_id >> 8) as u8,
            (event_id >> 16) as u8,
            (event_id >> 24) as u8,
            (event_id >> 32) as u8,
            (event_id >> 40) as u8,
            (event_id >> 48) as u8,
            (event_id >> 56) as u8,
            // Timestamp, 8 bytes
            ts as u8,
            (ts >> 8) as u8,
            (ts >> 16) as u8,
            (ts >> 24) as u8,
            (ts >> 32) as u8,
            (ts >> 40) as u8,
            (ts >> 48) as u8,
            (ts >> 56) as u8,
            // Trace ID, 16 bytes
            trace_id[0],
            trace_id[1],
            trace_id[2],
            trace_id[3],
            trace_id[4],
            trace_id[5],
            trace_id[6],
            trace_id[7],
            trace_id[8],
            trace_id[9],
            trace_id[10],
            trace_id[11],
            trace_id[12],
            trace_id[13],
            trace_id[14],
            trace_id[15],
            // Span ID, 8 bytes
            span_id[0],
            span_id[1],
            span_id[2],
            span_id[3],
            span_id[4],
            span_id[5],
            span_id[6],
            span_id[7],
            // Event data length, 4 bytes
            ln as u8,
            (ln >> 8) as u8,
            (ln >> 16) as u8,
            (ln >> 24) as u8,
        ])
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::reqauth::platform::RequestValidator;

    use tokio_stream::StreamExt;
    use url::Url;

    fn setup_reporter() -> Reporter {
        let (_tx, rx) = tokio::sync::mpsc::unbounded_channel();

        Reporter {
            rx,
            anchor: TimeAnchor::new(),
            http_client: reqwest::Client::new(),
            config: ReporterConfig {
                app_id: "test-app".to_string(),
                env_id: "test-env".to_string(),
                deploy_id: "test-deploy".to_string(),
                app_commit: "test-commit".to_string(),
                trace_endpoint: Url::parse("http://localhost:8080").unwrap(),
                platform_validator: RequestValidator::new_mock().into(),
            },
        }
    }

    #[tokio::test]
    async fn test_event_to_stream_empty_payload() {
        let reporter = setup_reporter();
        let (body_tx, body_rx) = tokio::sync::mpsc::unbounded_channel();
        let mut body_stream = Box::pin(reporter.create_body_stream(body_rx));

        // Create an event with empty data payload
        let event = TraceEvent {
            typ: EventType::LogMessage,
            id: model::TraceEventId(100),
            data: Bytes::from(vec![]), // Empty payload
            span: model::SpanKey(model::TraceId([5; 16]), model::SpanId([6; 8])),
            ts: tokio::time::Instant::now(),
        };

        // Send the event with empty payload
        body_tx
            .send(event)
            .expect("Failed to send event with empty payload");

        // Verify header was received
        let header = body_stream
            .next()
            .await
            .expect("Failed to receive header for empty payload")
            .expect("Header should be Ok");
        assert_eq!(header[0], EventType::LogMessage as u8);

        // Verify data length in header is zero
        let data_length = (header[41] as u32)
            | ((header[42] as u32) << 8)
            | ((header[43] as u32) << 16)
            | ((header[44] as u32) << 24);
        assert_eq!(data_length, 0, "Data length in header should be zero");

        // Verify empty data was received
        let data = body_stream
            .next()
            .await
            .expect("Failed to receive empty data payload")
            .expect("Data should be Ok");
        assert_eq!(data.len(), 0, "Data payload should be empty");

        drop(body_tx);

        // Verify stream is now empty
        assert!(
            body_stream.next().await.is_none(),
            "Stream should be empty after receiving header and data"
        );
    }

    #[tokio::test]
    async fn test_event_to_stream() {
        let reporter = setup_reporter();
        let (body_tx, body_rx) = tokio::sync::mpsc::unbounded_channel();
        let mut body_stream = Box::pin(reporter.create_body_stream(body_rx));

        // Use specific test values that we can verify in the header
        let event_id = 42u64;
        let trace_id = [
            0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE,
            0xFF, 0x00,
        ];
        let span_id = [0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22];
        let test_data = "hello world";
        let data_bytes = Bytes::from(test_data);

        let event = TraceEvent {
            typ: EventType::HTTPCallStart,
            id: model::TraceEventId(event_id),
            data: data_bytes.clone(),
            span: model::SpanKey(model::TraceId(trace_id), model::SpanId(span_id)),
            ts: tokio::time::Instant::now(),
        };

        body_tx.send(event).expect("Failed to send event to stream");

        let header = body_stream
            .next()
            .await
            .expect("Failed to receive header from stream")
            .expect("Header should be Ok");
        let data = body_stream
            .next()
            .await
            .expect("Failed to receive data from stream")
            .expect("Data should be Ok");

        // Verify that header has correct format and size
        assert_eq!(header.len(), 45, "Header should be exactly 45 bytes");

        // Verify event type
        assert_eq!(
            header[0],
            EventType::HTTPCallStart as u8,
            "Event type should match"
        );

        // Verify event ID (bytes 1-8)
        let header_event_id = (header[1] as u64)
            | ((header[2] as u64) << 8)
            | ((header[3] as u64) << 16)
            | ((header[4] as u64) << 24)
            | ((header[5] as u64) << 32)
            | ((header[6] as u64) << 40)
            | ((header[7] as u64) << 48)
            | ((header[8] as u64) << 56);
        assert_eq!(header_event_id, event_id, "Event ID in header should match");

        // Verify trace ID (bytes 17-32)
        for i in 0..16 {
            assert_eq!(
                header[17 + i],
                trace_id[i],
                "Trace ID byte {i} should match"
            );
        }

        // Verify span ID (bytes 33-40)
        for i in 0..8 {
            assert_eq!(header[33 + i], span_id[i], "Span ID byte {i} should match");
        }

        // Verify data length (bytes 41-44)
        let data_length = (header[41] as u32)
            | ((header[42] as u32) << 8)
            | ((header[43] as u32) << 16)
            | ((header[44] as u32) << 24);
        assert_eq!(
            data_length,
            test_data.len() as u32,
            "Data length in header should match"
        );

        // Verify data content
        assert_eq!(data, data_bytes, "Data payload should match original");
        assert_eq!(data.len(), test_data.len(), "Data length should match");

        drop(body_tx);
        // Verify channel is now empty
        assert!(
            body_stream.next().await.is_none(),
            "Channel should be empty after receiving header and data"
        );
    }

    #[tokio::test]
    async fn test_event_to_stream_large_payload() {
        let reporter = setup_reporter();
        let (body_tx, body_rx) = tokio::sync::mpsc::unbounded_channel();
        let mut body_stream = Box::pin(reporter.create_body_stream(body_rx));

        // Create a large payload (1MB)
        let large_data_size = 1024 * 1024;
        let large_data = vec![0xAA; large_data_size];

        let event = TraceEvent {
            typ: EventType::DBQueryStart,
            id: model::TraceEventId(42),
            data: Bytes::from(large_data.clone()),
            span: model::SpanKey(model::TraceId([3; 16]), model::SpanId([4; 8])),
            ts: tokio::time::Instant::now(),
        };

        // Send the event with large payload
        body_tx.send(event).expect("Failed to send event to stream");

        // Verify header was received
        let header = body_stream
            .next()
            .await
            .expect("Failed to receive header for large payload")
            .expect("Header should be Ok");
        assert_eq!(header[0], EventType::DBQueryStart as u8);

        // Extract the data length from the header (last 4 bytes)
        let data_length = (header[41] as u32)
            | ((header[42] as u32) << 8)
            | ((header[43] as u32) << 16)
            | ((header[44] as u32) << 24);

        // Verify the header correctly indicates the large size
        assert_eq!(data_length, large_data_size as u32);

        // Verify data was received and matches the original
        let data = body_stream
            .next()
            .await
            .expect("Failed to receive large payload data")
            .expect("Data should be Ok");
        assert_eq!(data.len(), large_data_size);
        assert_eq!(data[0], 0xAA);
        assert_eq!(data[large_data_size - 1], 0xAA);

        drop(body_tx);
        // Verify channel is now empty
        assert!(body_stream.next().await.is_none());
    }

    #[test]
    fn test_streaming_trace_event_header_format() {
        // Create a time anchor
        let anchor = TimeAnchor::new();

        // Create a trace event
        let event_type = EventType::LogMessage;
        let event_id = 42u64;
        let trace_id = model::TraceId([5; 16]);
        let span_id = model::SpanId([6; 8]);
        let data = Bytes::from(vec![10, 20, 30]);

        let event = TraceEvent {
            typ: event_type,
            id: model::TraceEventId(event_id),
            data: data.clone(),
            span: model::SpanKey(trace_id, span_id),
            ts: anchor.instant + std::time::Duration::from_nanos(123456789),
        };

        // Create a StreamingTraceEvent and get the header
        let streaming_event = StreamingTraceEvent { event };
        let header = streaming_event.header(&anchor);

        // Header should be exactly 45 bytes
        // 1 (type) + 8 (event id) + 8 (timestamp) + 16 (trace id) + 8 (span id) + 4 (data length)
        assert_eq!(header.len(), 45);

        // Check event type
        assert_eq!(header[0], event_type as u8);

        // Check event ID (little endian)
        assert_eq!(header[1], (event_id & 0xFF) as u8);
        assert_eq!(header[2], ((event_id >> 8) & 0xFF) as u8);
        assert_eq!(header[3], ((event_id >> 16) & 0xFF) as u8);
        assert_eq!(header[4], ((event_id >> 24) & 0xFF) as u8);
        assert_eq!(header[5], ((event_id >> 32) & 0xFF) as u8);
        assert_eq!(header[6], ((event_id >> 40) & 0xFF) as u8);
        assert_eq!(header[7], ((event_id >> 48) & 0xFF) as u8);
        assert_eq!(header[8], ((event_id >> 56) & 0xFF) as u8);

        // Check trace ID
        for i in 0..16 {
            assert_eq!(header[17 + i], 5);
        }

        // Check span ID
        for i in 0..8 {
            assert_eq!(header[33 + i], 6);
        }

        // Check data length (3 bytes, little endian)
        assert_eq!(header[41], 3);
        assert_eq!(header[42], 0);
        assert_eq!(header[43], 0);
        assert_eq!(header[44], 0);
    }

    #[test]
    fn test_streaming_trace_event_timestamp_encoding() {
        // Create a time anchor
        let anchor = TimeAnchor::new();

        // Create events with different timestamps
        let time_offsets = [
            0,             // Same as anchor
            1000,          // 1 microsecond
            1_000_000,     // 1 millisecond
            1_000_000_000, // 1 second
        ];

        for &offset_nanos in &time_offsets {
            let ts = anchor.instant + std::time::Duration::from_nanos(offset_nanos as u64);

            let event = TraceEvent {
                typ: EventType::LogMessage,
                id: model::TraceEventId(1),
                data: Bytes::from(vec![]),
                span: model::SpanKey(model::TraceId([0; 16]), model::SpanId([0; 8])),
                ts,
            };

            // Create a StreamingTraceEvent and get the header
            let streaming_event = StreamingTraceEvent { event };
            let header = streaming_event.header(&anchor);

            // Extract the timestamp from the header (bytes 9-16)
            let encoded_ts = (header[9] as u64)
                | ((header[10] as u64) << 8)
                | ((header[11] as u64) << 16)
                | ((header[12] as u64) << 24)
                | ((header[13] as u64) << 32)
                | ((header[14] as u64) << 40)
                | ((header[15] as u64) << 48)
                | ((header[16] as u64) << 56);

            // For non-negative timestamps, the encoded value should be the offset * 2
            assert_eq!(encoded_ts, (offset_nanos as u64) << 1);
        }
    }

    #[test]
    fn test_streaming_trace_event_different_event_types() {
        // Create a time anchor
        let anchor = TimeAnchor::new();

        // Test different event types
        let event_types = [
            EventType::RequestSpanStart,
            EventType::DBQueryStart,
            EventType::RPCCallEnd,
            EventType::LogMessage,
            EventType::TestEnd,
        ];

        for &event_type in &event_types {
            let event = TraceEvent {
                typ: event_type,
                id: model::TraceEventId(1),
                data: Bytes::from(vec![]),
                span: model::SpanKey(model::TraceId([0; 16]), model::SpanId([0; 8])),
                ts: anchor.instant,
            };

            // Create a StreamingTraceEvent and get the header
            let streaming_event = StreamingTraceEvent { event };
            let header = streaming_event.header(&anchor);

            // First byte should be the event type
            assert_eq!(header[0], event_type as u8);
        }
    }

    #[test]
    fn test_streaming_trace_event_data_length() {
        // Create a time anchor
        let anchor = TimeAnchor::new();

        // Test different data lengths
        let data_lengths = [0, 1, 10, 255, 256, 65535, 16777215];

        for &length in &data_lengths {
            // Create data of the specified length
            let data = Bytes::from(vec![0; length]);

            let event = TraceEvent {
                typ: EventType::LogMessage,
                id: model::TraceEventId(1),
                data,
                span: model::SpanKey(model::TraceId([0; 16]), model::SpanId([0; 8])),
                ts: anchor.instant,
            };

            // Create a StreamingTraceEvent and get the header
            let streaming_event = StreamingTraceEvent { event };
            let header = streaming_event.header(&anchor);

            // Last 4 bytes should contain the length in little-endian format
            let encoded_length = (header[41] as u32)
                | ((header[42] as u32) << 8)
                | ((header[43] as u32) << 16)
                | ((header[44] as u32) << 24);

            assert_eq!(encoded_length, length as u32);
        }
    }
}
