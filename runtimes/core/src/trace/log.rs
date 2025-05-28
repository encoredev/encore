use std::sync::Arc;

use crate::api::reqauth::platform;
use bytes::Bytes;
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
    pub async fn start_reporting(self) {
        let mut rx = self.rx;
        let anchor = self.anchor.clone();
        let trace_time_anchor = self.anchor.trace_header();

        let trace_headers = {
            use reqwest::header::*;
            let mut headers = HeaderMap::new();
            headers.insert(
                "X-Encore-App-Id",
                HeaderValue::from_str(&self.config.app_id).unwrap(),
            );
            headers.insert(
                "X-Encore-Env-Id",
                HeaderValue::from_str(&self.config.env_id).unwrap(),
            );
            headers.insert(
                "X-Encore-Deploy-Id",
                HeaderValue::from_str(&self.config.deploy_id).unwrap(),
            );
            headers.insert(
                "X-Encore-App-Commit",
                HeaderValue::from_str(&self.config.app_commit).unwrap(),
            );
            headers.insert("X-Encore-Trace-Version", HeaderValue::from(TRACE_VERSION));
            headers.insert(
                "X-Encore-Trace-TimeAnchor",
                HeaderValue::from_str(&trace_time_anchor).unwrap(),
            );

            headers
        };

        loop {
            let mut body_sender = None;

            let timeout_duration = std::time::Duration::from_millis(10000);
            let timeout_future = Box::pin(tokio::time::sleep(timeout_duration));
            let mut no_data_timeout = timeout_future;

            loop {
                tokio::select! {
                    event = rx.recv() => {
                        match event {
                            Some(event) => {
                                // Wait for at least one entry on rx before we open a HTTP request.
                                if body_sender.is_none() {
                                    // Create a channel for the streaming body
                                    let (tx, rx) = tokio::sync::mpsc::unbounded_channel::<Result<bytes::Bytes, std::convert::Infallible>>();
                                    let body = reqwest::Body::wrap_stream(UnboundedReceiverStream::new(rx));

                                    let mut req = self.http_client
                                        .post(self.config.trace_endpoint.clone())
                                        .headers(trace_headers.clone())
                                        .body(body)
                                        .build()
                                        .unwrap_or_else(|err| {
                                            log::error!("failed to build trace request: {:?}", err);
                                            panic!("Failed to build trace request");
                                        });

                                    if let Err(err) = self.config.platform_validator.sign_outgoing_request(&mut req) {
                                        log::error!("failed to sign trace request: {:?}", err);
                                        continue;
                                    }

                                    // Start the request
                                    let request = self.http_client.execute(req);
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

                                    body_sender = Some(tx);
                                }

                                // Add the event to the stream
                                if let Some(sender) = &body_sender {
                                    let streaming_event = StreamingTraceEvent {
                                        event,
                                    };

                                    // Send header
                                    let header = streaming_event.header(&anchor);
                                    if let Err(err) = sender.send(Ok(header)) {
                                        log::error!("failed to send trace header: {:?}", err);
                                        break;
                                    }

                                    // Send data
                                    let data = streaming_event.event.data.clone();
                                    if let Err(err) = sender.send(Ok(data)) {
                                        log::error!("failed to send trace data: {:?}", err);
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
