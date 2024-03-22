use std::convert::Infallible;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};

use crate::api::reqauth::platform;
use bytes::Bytes;

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
        let mut inner = Box::new(InnerTraceEventStream {
            rx: self.rx,
            anchor: self.anchor.clone(),
            current: None,
        });
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
            // Wait for at least one entry on rx before we open an HTTP request.
            {
                let Some(event) = inner.rx.recv().await else {
                    // The stream is closed. This only happens if all senders have been dropped,
                    // which should never happen in regular use.
                    return;
                };
                inner.current = Some(StreamingTraceEvent {
                    event,
                    next: EventStreamState::Header,
                });
            };

            // Construct the body stream.
            let mut no_data_ticker = tokio::time::interval(std::time::Duration::from_millis(1000));
            no_data_ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);

            let stream = TraceEventStream {
                inner: inner.as_mut() as *mut InnerTraceEventStream,
                num_events_this_tick: 1,
                no_data_ticker,
            };

            let req = self
                .http_client
                .post(self.config.trace_endpoint.clone())
                .headers(trace_headers.clone())
                .build();
            let mut req = match req {
                Ok(req) => req,
                Err(err) => {
                    log::error!("failed to build trace request: {:?}", err);
                    continue;
                }
            };

            if let Err(err) = self
                .config
                .platform_validator
                .sign_outgoing_request(&mut req)
            {
                log::error!("failed to sign trace request: {:?}", err);
                continue;
            }

            *req.body_mut() = Some(reqwest::Body::wrap_stream(stream));

            let result = self.http_client.execute(req).await;
            match result {
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
        }
    }
}

struct TraceEventStream {
    inner: *mut InnerTraceEventStream,

    // The number of events received since the last tick.
    num_events_this_tick: usize,

    /// Ticks to detect when there is no data to close the stream.
    no_data_ticker: tokio::time::Interval,
}

// Safety: the TraceEventStream only contains `poll_next` which requires a mutable reference
// to self. Therefore it is never called concurrently. The lifetime of inner is guaranteed
// to exceed the lifetime of the stream.
unsafe impl Send for TraceEventStream {}
unsafe impl Sync for TraceEventStream {}

struct InnerTraceEventStream {
    rx: tokio::sync::mpsc::UnboundedReceiver<TraceEvent>,
    anchor: TimeAnchor,

    /// Current item received from rx and being streamed.
    current: Option<StreamingTraceEvent>,
}

impl futures_core::stream::Stream for TraceEventStream {
    type Item = Result<Bytes, Infallible>;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context) -> Poll<Option<Self::Item>> {
        // Safety: the inner pointer is boxed and never moved, and is kept alive
        // by the start_reporting method for the lifetime of the stream.
        let inner = unsafe { &mut *self.inner };

        {
            // If we have a current item, return it.
            if inner.current.is_some() {
                let next = inner.current.as_ref().unwrap().next.clone();
                return match next {
                    EventStreamState::Header => {
                        inner.current.as_mut().unwrap().next = EventStreamState::Data;
                        Poll::Ready(Some(Ok(inner
                            .current
                            .as_ref()
                            .unwrap()
                            .header(&inner.anchor))))
                    }
                    EventStreamState::Data => {
                        let data = inner.current.as_ref().unwrap().event.data.clone(); // cheap clone
                        inner.current = None;
                        Poll::Ready(Some(Ok(data)))
                    }
                };
            }
        }

        // Check if the no-data-ticker is ready.
        match self.no_data_ticker.poll_tick(cx) {
            Poll::Ready(_) => {
                // If we have received no events since the last tick, close the stream.
                if self.num_events_this_tick == 0 {
                    return Poll::Ready(None);
                }
                self.num_events_this_tick = 0;

                // Call the ticker again to schedule a wake-up for the next tick.
                _ = self.no_data_ticker.poll_tick(cx);
            }
            Poll::Pending => {}
        }

        // If we have no current item, poll the receiver for a new trace event.
        {
            match inner.rx.poll_recv(cx) {
                Poll::Ready(Some(event)) => {
                    self.num_events_this_tick += 1;
                    inner.current = Some(StreamingTraceEvent {
                        event,
                        next: EventStreamState::Header,
                    });
                    self.poll_next(cx)
                }
                Poll::Ready(None) => Poll::Ready(None),
                Poll::Pending => Poll::Pending,
            }
        }
    }
}

/// Represents a trace event that is being streamed.
#[derive(Debug)]
struct StreamingTraceEvent {
    /// The event itself.
    event: TraceEvent,
    /// The next part of the event to be sent.
    next: EventStreamState,
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

/// Represents the piece of data to be sent next.
#[derive(Debug, Clone, Copy)]
enum EventStreamState {
    Header,
    Data,
}
