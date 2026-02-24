use std::collections::HashMap;

use crate::reader::{self, EventReader};
use crate::types::*;

// Event type constants (wire format byte values).
const REQUEST_SPAN_START: u8 = 0x01;
const REQUEST_SPAN_END: u8 = 0x02;
const AUTH_SPAN_START: u8 = 0x03;
const AUTH_SPAN_END: u8 = 0x04;
const PUBSUB_MESSAGE_SPAN_START: u8 = 0x05;
const PUBSUB_MESSAGE_SPAN_END: u8 = 0x06;
const DB_TRANSACTION_START: u8 = 0x07;
const DB_TRANSACTION_END: u8 = 0x08;
const DB_QUERY_START: u8 = 0x09;
const DB_QUERY_END: u8 = 0x0A;
const RPC_CALL_START: u8 = 0x0B;
const RPC_CALL_END: u8 = 0x0C;
const HTTP_CALL_START: u8 = 0x0D;
const HTTP_CALL_END: u8 = 0x0E;
const LOG_MESSAGE: u8 = 0x0F;
const PUBSUB_PUBLISH_START: u8 = 0x10;
const PUBSUB_PUBLISH_END: u8 = 0x11;
const SERVICE_INIT_START: u8 = 0x12;
const SERVICE_INIT_END: u8 = 0x13;
const CACHE_CALL_START: u8 = 0x14;
const CACHE_CALL_END: u8 = 0x15;
const BODY_STREAM: u8 = 0x16;
const TEST_START: u8 = 0x17;
const TEST_END: u8 = 0x18;
const BUCKET_OBJECT_UPLOAD_START: u8 = 0x19;
const BUCKET_OBJECT_UPLOAD_END: u8 = 0x1A;
const BUCKET_OBJECT_DOWNLOAD_START: u8 = 0x1B;
const BUCKET_OBJECT_DOWNLOAD_END: u8 = 0x1C;
const BUCKET_OBJECT_GET_ATTRS_START: u8 = 0x1D;
const BUCKET_OBJECT_GET_ATTRS_END: u8 = 0x1E;
const BUCKET_LIST_OBJECTS_START: u8 = 0x1F;
const BUCKET_LIST_OBJECTS_END: u8 = 0x20;
const BUCKET_DELETE_OBJECTS_START: u8 = 0x21;
const BUCKET_DELETE_OBJECTS_END: u8 = 0x22;

// HTTP trace event codes.
const HTTP_GET_CONN: u8 = 1;
const HTTP_GOT_CONN: u8 = 2;
const HTTP_GOT_FIRST_RESPONSE_BYTE: u8 = 3;
const HTTP_GOT_1XX_RESPONSE: u8 = 4;
const HTTP_DNS_START: u8 = 5;
const HTTP_DNS_DONE: u8 = 6;
const HTTP_CONNECT_START: u8 = 7;
const HTTP_CONNECT_DONE: u8 = 8;
const HTTP_TLS_HANDSHAKE_START: u8 = 9;
const HTTP_TLS_HANDSHAKE_DONE: u8 = 10;
const HTTP_WROTE_HEADERS: u8 = 11;
const HTTP_WROTE_REQUEST: u8 = 12;
const HTTP_WAIT_100_CONTINUE: u8 = 13;
const HTTP_CLOSED_BODY: u8 = 14;

// Log field type constants (wire format).
const LOG_FIELD_ERR: u8 = 1;
const LOG_FIELD_STRING: u8 = 2;
const LOG_FIELD_BOOL: u8 = 3;
const LOG_FIELD_TIME: u8 = 4;
const LOG_FIELD_DURATION: u8 = 5;
const LOG_FIELD_UUID: u8 = 6;
const LOG_FIELD_JSON: u8 = 7;
const LOG_FIELD_INT: u8 = 8;
const LOG_FIELD_UINT: u8 = 9;
const LOG_FIELD_FLOAT32: u8 = 10;
const LOG_FIELD_FLOAT64: u8 = 11;

/// Parse a single trace event from the reader.
///
/// Reads one complete event (header + body) from the stream.
/// Returns `ParseError::EndOfStream` when there are no more events.
pub fn parse_event(
    reader: &mut impl std::io::Read,
    time_anchor: &TimeAnchor,
    version: u16,
) -> Result<TraceEvent, ParseError> {
    let header = reader::read_header(reader)?;
    let body = reader::read_body(reader, header.data_len)?;
    let mut r = EventReader::new(&body, version);

    let event_time = time_anchor.to_real(header.nanotime);

    let event = match header.event_type {
        REQUEST_SPAN_START => Event::SpanStart(r.request_span_start()),
        REQUEST_SPAN_END => Event::SpanEnd(r.request_span_end()),
        AUTH_SPAN_START => Event::SpanStart(r.auth_span_start()),
        AUTH_SPAN_END => Event::SpanEnd(r.auth_span_end()),
        PUBSUB_MESSAGE_SPAN_START => Event::SpanStart(r.pubsub_message_span_start()),
        PUBSUB_MESSAGE_SPAN_END => Event::SpanEnd(r.pubsub_message_span_end()),
        TEST_START => Event::SpanStart(r.test_span_start()),
        TEST_END => Event::SpanEnd(r.test_span_end()),
        other => Event::SpanEvent(r.span_event(other)?),
    };

    if r.has_error() {
        return Err(ParseError::UnexpectedEof);
    }

    Ok(TraceEvent {
        trace_id: header.trace_id,
        span_id: header.span_id,
        event_id: header.event_id,
        event_time,
        event,
    })
}

// === Internal helpers ===

fn non_zero_u32(val: u32) -> Option<u32> {
    if val == 0 {
        None
    } else {
        Some(val)
    }
}

fn non_zero_u64(val: u64) -> Option<u64> {
    if val == 0 {
        None
    } else {
        Some(val)
    }
}

/// Common span start fields.
struct SpanStartCommon {
    goid: u32,
    parent_trace_id: Option<TraceId>,
    parent_span_id: Option<u64>,
    def_loc: Option<u32>,
    caller_event_id: Option<u64>,
    ext_correlation_id: Option<String>,
}

/// Common span end fields.
struct SpanEndCommon {
    duration_nanos: u64,
    status_code: StatusCode,
    error: Option<TracedError>,
    panic_stack: Option<StackTrace>,
    parent_trace_id: Option<TraceId>,
    parent_span_id: Option<u64>,
}

// === Event-specific parsing methods on EventReader ===

impl EventReader<'_> {
    // --- Common parsers ---

    fn span_start_common(&mut self) -> SpanStartCommon {
        let goid = self.uvarint() as u32;
        let parent_trace_id = self.trace_id();
        let parent_span_id = self.uint64();
        let def_loc = self.uvarint() as u32;
        let caller_event_id = self.uvarint();
        let ext_correlation_id = self.string();

        SpanStartCommon {
            goid,
            parent_trace_id: if !parent_trace_id.is_zero() {
                Some(parent_trace_id)
            } else {
                None
            },
            parent_span_id: non_zero_u64(parent_span_id),
            def_loc: non_zero_u32(def_loc),
            caller_event_id: non_zero_u64(caller_event_id),
            ext_correlation_id: if ext_correlation_id.is_empty() {
                None
            } else {
                Some(ext_correlation_id)
            },
        }
    }

    fn span_end_common(&mut self) -> SpanEndCommon {
        let dur = self.duration();
        let duration_nanos = if dur < 0 { 0 } else { dur as u64 };

        let (status_code, error) = if self.version >= 17 {
            let status = StatusCode::from_byte(self.byte());
            let err = self.err_with_stack();
            (status, err)
        } else {
            let err = self.err_with_stack();
            let status = if err.is_some() {
                StatusCode::Unknown
            } else {
                StatusCode::Ok
            };
            (status, err)
        };

        let panic_stack = self.formatted_stack();
        let parent_trace_id = self.trace_id();
        let parent_span_id = self.uint64();

        SpanEndCommon {
            duration_nanos,
            status_code,
            error,
            panic_stack,
            parent_trace_id: if !parent_trace_id.is_zero() {
                Some(parent_trace_id)
            } else {
                None
            },
            parent_span_id: non_zero_u64(parent_span_id),
        }
    }

    fn headers(&mut self) -> HashMap<String, String> {
        let n = self.uvarint() as usize;
        if n == 0 {
            return HashMap::new();
        }
        let mut headers = HashMap::with_capacity(n);
        for _ in 0..n {
            let key = self.string();
            let value = self.string();
            headers.insert(key, value);
        }
        headers
    }

    fn stack(&mut self) -> Option<StackTrace> {
        let n = self.byte() as usize;
        if n == 0 {
            return None;
        }

        let mut diffs = Vec::with_capacity(n);
        for _ in 0..n {
            diffs.push(self.varint());
        }

        Some(StackTrace {
            pcs: diffs,
            frames: Vec::new(),
        })
    }

    fn formatted_stack(&mut self) -> Option<StackTrace> {
        let n = self.byte() as usize;
        if n == 0 {
            return None;
        }

        let mut frames = Vec::with_capacity(n);
        for _ in 0..n {
            frames.push(StackFrame {
                filename: self.string(),
                line: self.uvarint() as i32,
                func_name: self.string(),
            });
        }

        Some(StackTrace {
            pcs: Vec::new(),
            frames,
        })
    }

    fn err_with_stack(&mut self) -> Option<TracedError> {
        let msg = self.string();
        if msg.is_empty() {
            return None;
        }
        let stack = self.stack();
        Some(TracedError { msg, stack })
    }

    fn bucket_object_attrs(&mut self) -> BucketObjectAttributes {
        BucketObjectAttributes {
            size: self.opt_uvarint(),
            version: self.opt_string(),
            etag: self.opt_string(),
            content_type: self.opt_string(),
        }
    }

    // --- Span starts ---

    fn request_span_start(&mut self) -> SpanStart {
        let c = self.span_start_common();

        let service_name = self.string();
        let endpoint_name = self.string();
        let http_method = self.string();
        let path = self.string();

        let n = self.uvarint() as usize;
        let mut path_params = Vec::with_capacity(n);
        for _ in 0..n {
            path_params.push(self.string());
        }

        let request_headers = self.headers();
        let request_payload = self.byte_string();
        let ext_correlation_id = self.opt_string();
        let uid = self.opt_string();
        let mocked = if self.version >= 15 {
            self.bool_val()
        } else {
            false
        };

        SpanStart {
            goid: c.goid,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            def_loc: c.def_loc,
            caller_event_id: c.caller_event_id,
            external_correlation_id: c.ext_correlation_id,
            data: SpanStartData::Request(RequestSpanStart {
                service_name,
                endpoint_name,
                http_method,
                path,
                path_params,
                request_headers,
                request_payload,
                ext_correlation_id,
                uid,
                mocked,
            }),
        }
    }

    fn auth_span_start(&mut self) -> SpanStart {
        let c = self.span_start_common();

        SpanStart {
            goid: c.goid,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            def_loc: c.def_loc,
            caller_event_id: c.caller_event_id,
            external_correlation_id: c.ext_correlation_id,
            data: SpanStartData::Auth(AuthSpanStart {
                service_name: self.string(),
                endpoint_name: self.string(),
                auth_payload: self.byte_string(),
            }),
        }
    }

    fn pubsub_message_span_start(&mut self) -> SpanStart {
        let c = self.span_start_common();

        SpanStart {
            goid: c.goid,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            def_loc: c.def_loc,
            caller_event_id: c.caller_event_id,
            external_correlation_id: c.ext_correlation_id,
            data: SpanStartData::PubsubMessage(PubsubMessageSpanStart {
                service_name: self.string(),
                topic_name: self.string(),
                subscription_name: self.string(),
                message_id: self.string(),
                attempt: self.uvarint() as u32,
                publish_time: self.time(),
                message_payload: self.byte_string(),
            }),
        }
    }

    fn test_span_start(&mut self) -> SpanStart {
        let c = self.span_start_common();

        SpanStart {
            goid: c.goid,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            def_loc: c.def_loc,
            caller_event_id: c.caller_event_id,
            external_correlation_id: c.ext_correlation_id,
            data: SpanStartData::Test(TestSpanStart {
                service_name: self.string(),
                test_name: self.string(),
                uid: self.string(),
                test_file: self.string(),
                test_line: self.uint32(),
            }),
        }
    }

    // --- Span ends ---

    fn request_span_end(&mut self) -> SpanEnd {
        let c = self.span_end_common();

        let service_name = self.string();
        let endpoint_name = self.string();
        let http_status_code = self.uvarint() as u32;
        let response_headers = self.headers();
        let response_payload = self.byte_string();

        let caller_event_id = if self.version >= 16 {
            non_zero_u64(self.event_id())
        } else {
            None
        };

        let uid = if self.version >= 17 {
            self.opt_string()
        } else {
            None
        };

        SpanEnd {
            duration_nanos: c.duration_nanos,
            status_code: c.status_code,
            error: c.error,
            panic_stack: c.panic_stack,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            data: SpanEndData::Request(RequestSpanEnd {
                service_name,
                endpoint_name,
                http_status_code,
                response_headers,
                response_payload,
                caller_event_id,
                uid,
            }),
        }
    }

    fn auth_span_end(&mut self) -> SpanEnd {
        let c = self.span_end_common();

        SpanEnd {
            duration_nanos: c.duration_nanos,
            status_code: c.status_code,
            error: c.error,
            panic_stack: c.panic_stack,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            data: SpanEndData::Auth(AuthSpanEnd {
                service_name: self.string(),
                endpoint_name: self.string(),
                uid: self.string(),
                user_data: self.byte_string(),
            }),
        }
    }

    fn pubsub_message_span_end(&mut self) -> SpanEnd {
        let c = self.span_end_common();

        let service_name = self.string();
        let topic_name = self.string();
        let subscription_name = self.string();
        let message_id = if self.version >= 17 {
            self.string()
        } else {
            String::new()
        };

        SpanEnd {
            duration_nanos: c.duration_nanos,
            status_code: c.status_code,
            error: c.error,
            panic_stack: c.panic_stack,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            data: SpanEndData::PubsubMessage(PubsubMessageSpanEnd {
                service_name,
                topic_name,
                subscription_name,
                message_id,
            }),
        }
    }

    fn test_span_end(&mut self) -> SpanEnd {
        let c = self.span_end_common();

        let service_name = self.string();
        let test_name = self.string();
        let failed = self.bool_val();
        let skipped = self.bool_val();
        let uid = if self.version >= 17 {
            self.opt_string()
        } else {
            None
        };

        SpanEnd {
            duration_nanos: c.duration_nanos,
            status_code: c.status_code,
            error: c.error,
            panic_stack: c.panic_stack,
            parent_trace_id: c.parent_trace_id,
            parent_span_id: c.parent_span_id,
            data: SpanEndData::Test(TestSpanEnd {
                service_name,
                test_name,
                failed,
                skipped,
                uid,
            }),
        }
    }

    // --- Span events ---

    fn span_event(&mut self, event_type: u8) -> Result<SpanEvent, ParseError> {
        let def_loc = self.uvarint() as u32;
        let goid = self.uvarint() as u32;
        let correlation_event_id = self.event_id();

        let data = match event_type {
            RPC_CALL_START => SpanEventData::RpcCallStart(self.rpc_call_start()),
            RPC_CALL_END => SpanEventData::RpcCallEnd(self.rpc_call_end()),
            DB_QUERY_START => SpanEventData::DbQueryStart(self.db_query_start()),
            DB_QUERY_END => SpanEventData::DbQueryEnd(self.db_query_end()),
            DB_TRANSACTION_START => SpanEventData::DbTransactionStart(self.db_transaction_start()),
            DB_TRANSACTION_END => SpanEventData::DbTransactionEnd(self.db_transaction_end()),
            PUBSUB_PUBLISH_START => SpanEventData::PubsubPublishStart(self.pubsub_publish_start()),
            PUBSUB_PUBLISH_END => SpanEventData::PubsubPublishEnd(self.pubsub_publish_end()),
            HTTP_CALL_START => SpanEventData::HttpCallStart(self.http_call_start()),
            HTTP_CALL_END => SpanEventData::HttpCallEnd(self.http_call_end()),
            LOG_MESSAGE => SpanEventData::LogMessage(self.log_message()),
            SERVICE_INIT_START => SpanEventData::ServiceInitStart(self.service_init_start()),
            SERVICE_INIT_END => SpanEventData::ServiceInitEnd(self.service_init_end()),
            CACHE_CALL_START => SpanEventData::CacheCallStart(self.cache_call_start()),
            CACHE_CALL_END => SpanEventData::CacheCallEnd(self.cache_call_end()),
            BODY_STREAM => SpanEventData::BodyStream(self.body_stream()),
            BUCKET_OBJECT_UPLOAD_START => {
                SpanEventData::BucketObjectUploadStart(self.bucket_object_upload_start())
            }
            BUCKET_OBJECT_UPLOAD_END => {
                SpanEventData::BucketObjectUploadEnd(self.bucket_object_upload_end())
            }
            BUCKET_OBJECT_DOWNLOAD_START => {
                SpanEventData::BucketObjectDownloadStart(self.bucket_object_download_start())
            }
            BUCKET_OBJECT_DOWNLOAD_END => {
                SpanEventData::BucketObjectDownloadEnd(self.bucket_object_download_end())
            }
            BUCKET_OBJECT_GET_ATTRS_START => {
                SpanEventData::BucketObjectGetAttrsStart(self.bucket_object_get_attrs_start())
            }
            BUCKET_OBJECT_GET_ATTRS_END => {
                SpanEventData::BucketObjectGetAttrsEnd(self.bucket_object_get_attrs_end())
            }
            BUCKET_LIST_OBJECTS_START => {
                SpanEventData::BucketListObjectsStart(self.bucket_list_objects_start())
            }
            BUCKET_LIST_OBJECTS_END => {
                SpanEventData::BucketListObjectsEnd(self.bucket_list_objects_end())
            }
            BUCKET_DELETE_OBJECTS_START => {
                SpanEventData::BucketDeleteObjectsStart(self.bucket_delete_objects_start())
            }
            BUCKET_DELETE_OBJECTS_END => {
                SpanEventData::BucketDeleteObjectsEnd(self.bucket_delete_objects_end())
            }
            other => return Err(ParseError::UnknownEventType(other)),
        };

        Ok(SpanEvent {
            goid,
            def_loc: non_zero_u32(def_loc),
            correlation_event_id: non_zero_u64(correlation_event_id),
            data,
        })
    }

    // --- RPC ---

    fn rpc_call_start(&mut self) -> RpcCallStart {
        RpcCallStart {
            target_service_name: self.string(),
            target_endpoint_name: self.string(),
            stack: self.stack(),
        }
    }

    fn rpc_call_end(&mut self) -> RpcCallEnd {
        RpcCallEnd {
            err: self.err_with_stack(),
        }
    }

    // --- DB ---

    fn db_query_start(&mut self) -> DbQueryStart {
        DbQueryStart {
            query: self.string(),
            stack: self.stack(),
        }
    }

    fn db_query_end(&mut self) -> DbQueryEnd {
        DbQueryEnd {
            err: self.err_with_stack(),
        }
    }

    fn db_transaction_start(&mut self) -> DbTransactionStart {
        DbTransactionStart {
            stack: self.stack(),
        }
    }

    fn db_transaction_end(&mut self) -> DbTransactionEnd {
        let completion = if self.bool_val() {
            DbTransactionCompletion::Commit
        } else {
            DbTransactionCompletion::Rollback
        };
        DbTransactionEnd {
            completion,
            stack: self.stack(),
            err: self.err_with_stack(),
        }
    }

    // --- Pubsub ---

    fn pubsub_publish_start(&mut self) -> PubsubPublishStart {
        PubsubPublishStart {
            topic: self.string(),
            message: self.byte_string(),
            stack: self.stack(),
        }
    }

    fn pubsub_publish_end(&mut self) -> PubsubPublishEnd {
        PubsubPublishEnd {
            message_id: self.opt_string(),
            err: self.err_with_stack(),
        }
    }

    // --- Service init ---

    fn service_init_start(&mut self) -> ServiceInitStart {
        ServiceInitStart {
            service: self.string(),
        }
    }

    fn service_init_end(&mut self) -> ServiceInitEnd {
        ServiceInitEnd {
            err: self.err_with_stack(),
        }
    }

    // --- HTTP call ---

    fn http_call_start(&mut self) -> HttpCallStart {
        HttpCallStart {
            correlation_parent_span_id: self.uint64(),
            method: self.string(),
            url: self.string(),
            stack: self.stack(),
            start_nanotime: self.int64(),
        }
    }

    fn http_call_end(&mut self) -> HttpCallEnd {
        let status_code_raw = self.uvarint() as u32;
        let status_code = non_zero_u32(status_code_raw);
        let err = self.err_with_stack();

        let n = self.uvarint() as usize;
        let mut trace_events = Vec::with_capacity(n);
        for _ in 0..n {
            if let Some(ev) = self.http_trace_event() {
                trace_events.push(ev);
            }
        }

        HttpCallEnd {
            status_code,
            err,
            trace_events,
        }
    }

    fn http_trace_event(&mut self) -> Option<HttpTraceEvent> {
        let code = self.byte();
        let nanotime = self.int64();

        let data = match code {
            HTTP_GET_CONN => HttpTraceEventData::GetConn(HttpGetConn {
                host_port: self.string(),
            }),
            HTTP_GOT_CONN => HttpTraceEventData::GotConn(HttpGotConn {
                reused: self.bool_val(),
                was_idle: self.bool_val(),
                idle_duration_ns: self.int64(),
            }),
            HTTP_GOT_FIRST_RESPONSE_BYTE => HttpTraceEventData::GotFirstResponseByte,
            HTTP_GOT_1XX_RESPONSE => HttpTraceEventData::Got1xxResponse(HttpGot1xxResponse {
                code: self.varint() as i32,
            }),
            HTTP_DNS_START => HttpTraceEventData::DnsStart(HttpDnsStart {
                host: self.string(),
            }),
            HTTP_DNS_DONE => {
                let err = self.byte_string();
                let addr_count = self.uvarint() as usize;
                let mut addrs = Vec::with_capacity(addr_count);
                for _ in 0..addr_count {
                    addrs.push(DnsAddr {
                        ip: self.byte_string(),
                    });
                }
                HttpTraceEventData::DnsDone(HttpDnsDone { err, addrs })
            }
            HTTP_CONNECT_START => HttpTraceEventData::ConnectStart(HttpConnectStart {
                network: self.string(),
                addr: self.string(),
            }),
            HTTP_CONNECT_DONE => HttpTraceEventData::ConnectDone(HttpConnectDone {
                network: self.string(),
                addr: self.string(),
                err: self.byte_string(),
            }),
            HTTP_TLS_HANDSHAKE_START => HttpTraceEventData::TlsHandshakeStart,
            HTTP_TLS_HANDSHAKE_DONE => HttpTraceEventData::TlsHandshakeDone(HttpTlsHandshakeDone {
                err: self.byte_string(),
                tls_version: self.uint32(),
                cipher_suite: self.uint32(),
                server_name: self.string(),
                negotiated_protocol: self.string(),
            }),
            HTTP_WROTE_HEADERS => HttpTraceEventData::WroteHeaders,
            HTTP_WROTE_REQUEST => HttpTraceEventData::WroteRequest(HttpWroteRequest {
                err: self.byte_string(),
            }),
            HTTP_WAIT_100_CONTINUE => HttpTraceEventData::Wait100Continue,
            HTTP_CLOSED_BODY => HttpTraceEventData::ClosedBody(HttpClosedBody {
                err: self.byte_string(),
            }),
            _ => return None,
        };

        Some(HttpTraceEvent { nanotime, data })
    }

    // --- Cache ---

    fn cache_call_start(&mut self) -> CacheCallStart {
        let operation = self.string();
        let write = self.bool_val();
        let stack = self.stack();
        let n = self.uvarint() as usize;
        let mut keys = Vec::with_capacity(n);
        for _ in 0..n {
            keys.push(self.string());
        }
        CacheCallStart {
            operation,
            write,
            stack,
            keys,
        }
    }

    fn cache_call_end(&mut self) -> CacheCallEnd {
        let result = CacheResult::from_byte(self.byte());
        CacheCallEnd {
            result,
            err: self.err_with_stack(),
        }
    }

    // --- Body stream ---

    fn body_stream(&mut self) -> BodyStream {
        let flags = self.byte();
        let data = self.byte_string();
        BodyStream {
            is_response: flags & 0b01 == 0b01,
            overflowed: flags & 0b10 == 0b10,
            data,
        }
    }

    // --- Log ---

    fn log_message(&mut self) -> LogMessage {
        let level = LogLevel::from_wire_byte(self.byte());
        let msg = self.string();

        let n = self.uvarint() as usize;
        let mut fields = Vec::with_capacity(n.min(64));
        for _ in 0..n {
            if let Some(f) = self.log_field() {
                fields.push(f);
            }
        }

        let stack = self.stack();

        LogMessage {
            level,
            msg,
            fields,
            stack,
        }
    }

    fn log_field(&mut self) -> Option<LogField> {
        let typ = self.byte();
        let key = self.string();

        let value = match typ {
            LOG_FIELD_ERR => {
                let err = self.err_with_stack().unwrap_or(TracedError {
                    msg: String::new(),
                    stack: None,
                });
                LogFieldValue::Error(err)
            }
            LOG_FIELD_STRING => LogFieldValue::Str(self.string()),
            LOG_FIELD_BOOL => LogFieldValue::Bool(self.bool_val()),
            LOG_FIELD_TIME => LogFieldValue::Time(self.time()),
            LOG_FIELD_DURATION => LogFieldValue::Duration(self.int64()),
            LOG_FIELD_UUID => LogFieldValue::Uuid(self.bytes(16)),
            LOG_FIELD_JSON => {
                let val = self.byte_string();
                let err = self.err_with_stack();
                if let Some(e) = err {
                    LogFieldValue::Error(e)
                } else {
                    LogFieldValue::Json(val)
                }
            }
            LOG_FIELD_INT => LogFieldValue::Int(self.varint()),
            LOG_FIELD_UINT => LogFieldValue::Uint(self.uvarint()),
            LOG_FIELD_FLOAT32 => LogFieldValue::Float32(self.float32()),
            LOG_FIELD_FLOAT64 => LogFieldValue::Float64(self.float64()),
            _ => return None,
        };

        Some(LogField { key, value })
    }

    // --- Bucket operations ---

    fn bucket_object_upload_start(&mut self) -> BucketObjectUploadStart {
        BucketObjectUploadStart {
            bucket: self.string(),
            object: self.string(),
            attrs: self.bucket_object_attrs(),
            stack: self.stack(),
        }
    }

    fn bucket_object_upload_end(&mut self) -> BucketObjectUploadEnd {
        BucketObjectUploadEnd {
            size: self.opt_uvarint(),
            version: self.opt_string(),
            err: self.err_with_stack(),
        }
    }

    fn bucket_object_download_start(&mut self) -> BucketObjectDownloadStart {
        BucketObjectDownloadStart {
            bucket: self.string(),
            object: self.string(),
            version: self.opt_string(),
            stack: self.stack(),
        }
    }

    fn bucket_object_download_end(&mut self) -> BucketObjectDownloadEnd {
        BucketObjectDownloadEnd {
            size: self.opt_uvarint(),
            err: self.err_with_stack(),
        }
    }

    fn bucket_object_get_attrs_start(&mut self) -> BucketObjectGetAttrsStart {
        BucketObjectGetAttrsStart {
            bucket: self.string(),
            object: self.string(),
            version: self.opt_string(),
            stack: self.stack(),
        }
    }

    fn bucket_object_get_attrs_end(&mut self) -> BucketObjectGetAttrsEnd {
        let err = self.err_with_stack();
        let attrs = if err.is_none() {
            Some(self.bucket_object_attrs())
        } else {
            None
        };
        BucketObjectGetAttrsEnd { err, attrs }
    }

    fn bucket_list_objects_start(&mut self) -> BucketListObjectsStart {
        BucketListObjectsStart {
            bucket: self.string(),
            prefix: self.opt_string(),
            stack: self.stack(),
        }
    }

    fn bucket_list_objects_end(&mut self) -> BucketListObjectsEnd {
        BucketListObjectsEnd {
            err: self.err_with_stack(),
            observed: self.uvarint(),
            has_more: self.bool_val(),
        }
    }

    fn bucket_delete_objects_start(&mut self) -> BucketDeleteObjectsStart {
        let bucket = self.string();
        let stack = self.stack();
        let n = self.uvarint() as usize;
        let mut entries = Vec::with_capacity(n);
        for _ in 0..n {
            entries.push(BucketDeleteObjectEntry {
                object: self.string(),
                version: self.opt_string(),
            });
        }
        BucketDeleteObjectsStart {
            bucket,
            stack,
            entries,
        }
    }

    fn bucket_delete_objects_end(&mut self) -> BucketDeleteObjectsEnd {
        BucketDeleteObjectsEnd {
            err: self.err_with_stack(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Helper to build a complete binary event.
    fn build_event(event_type: u8, body: &[u8]) -> Vec<u8> {
        let mut data = Vec::new();
        // Type byte
        data.push(event_type);
        // EventID: 1
        data.extend_from_slice(&1u64.to_le_bytes());
        // Nanotime: zigzag(1000) = 2000
        data.extend_from_slice(&2000u64.to_le_bytes());
        // TraceID: low=10, high=20
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&20u64.to_le_bytes());
        // SpanID: 5
        data.extend_from_slice(&5u64.to_le_bytes());
        // DataLen
        data.extend_from_slice(&(body.len() as u32).to_le_bytes());
        // Body
        data.extend_from_slice(body);
        data
    }

    fn test_time_anchor() -> TimeAnchor {
        TimeAnchor {
            real: Timestamp {
                seconds: 1700000000,
                nanos: 0,
            },
            mono_nanos: 0,
        }
    }

    #[test]
    fn test_parse_service_init_start() {
        // Body: defLoc(0) + goid(0) + correlationEventID(0) + service("myservice")
        let mut body = Vec::new();
        body.push(0x00); // defLoc
        body.push(0x00); // goid
        body.push(0x00); // correlationEventID
        body.push(9); // string length
        body.extend_from_slice(b"myservice");

        let data = build_event(SERVICE_INIT_START, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        assert_eq!(event.trace_id, TraceId { high: 20, low: 10 });
        assert_eq!(event.span_id, 5);
        assert_eq!(event.event_id, 1);

        match &event.event {
            Event::SpanEvent(se) => {
                assert_eq!(se.goid, 0);
                assert_eq!(se.def_loc, None);
                assert_eq!(se.correlation_event_id, None);
                match &se.data {
                    SpanEventData::ServiceInitStart(s) => {
                        assert_eq!(s.service, "myservice");
                    }
                    other => panic!("expected ServiceInitStart, got {:?}", other),
                }
            }
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_service_init_end_no_error() {
        // Body: defLoc(0) + goid(0) + correlationEventID(0) + empty error string
        let body = vec![0x00, 0x00, 0x00, 0x00]; // defLoc, goid, corrID, empty string

        let data = build_event(SERVICE_INIT_END, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::ServiceInitEnd(e) => {
                    assert!(e.err.is_none());
                }
                other => panic!("expected ServiceInitEnd, got {:?}", other),
            },
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_service_init_end_with_error() {
        // Body: defLoc(0) + goid(0) + correlationEventID(0) + error("oops") + stack(0)
        let mut body = Vec::new();
        body.push(0x00); // defLoc
        body.push(0x00); // goid
        body.push(0x00); // correlationEventID
        body.push(4); // error string length
        body.extend_from_slice(b"oops");
        body.push(0x00); // stack trace: 0 PCs

        let data = build_event(SERVICE_INIT_END, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::ServiceInitEnd(e) => {
                    let err = e.err.as_ref().unwrap();
                    assert_eq!(err.msg, "oops");
                    assert!(err.stack.is_none());
                }
                other => panic!("expected ServiceInitEnd, got {:?}", other),
            },
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_log_message() {
        let mut body = Vec::new();
        // Span event header: defLoc(0) + goid(0) + correlationEventID(0)
        body.push(0x00);
        body.push(0x00);
        body.push(0x00);
        // LogMessage: level=2 (Info)
        body.push(2);
        // msg = "hello world"
        body.push(11);
        body.extend_from_slice(b"hello world");
        // fields count = 1
        body.push(1);
        // field: type=STRING(2), key="key1", value="val1"
        body.push(LOG_FIELD_STRING);
        body.push(4);
        body.extend_from_slice(b"key1");
        body.push(4);
        body.extend_from_slice(b"val1");
        // stack: 0
        body.push(0);

        let data = build_event(LOG_MESSAGE, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::LogMessage(log) => {
                    assert_eq!(log.level, LogLevel::Info);
                    assert_eq!(log.msg, "hello world");
                    assert_eq!(log.fields.len(), 1);
                    assert_eq!(log.fields[0].key, "key1");
                    assert_eq!(log.fields[0].value, LogFieldValue::Str("val1".to_string()));
                    assert!(log.stack.is_none());
                }
                other => panic!("expected LogMessage, got {:?}", other),
            },
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_rpc_call_start() {
        let mut body = Vec::new();
        // Span event header
        body.push(0x05); // defLoc = 5
        body.push(0x0A); // goid = 10
        body.push(0x00); // correlationEventID = 0
        // RpcCallStart
        body.push(7);
        body.extend_from_slice(b"svc-foo");
        body.push(9);
        body.extend_from_slice(b"DoRequest");
        body.push(0); // no stack

        let data = build_event(RPC_CALL_START, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => {
                assert_eq!(se.goid, 10);
                assert_eq!(se.def_loc, Some(5));
                match &se.data {
                    SpanEventData::RpcCallStart(rpc) => {
                        assert_eq!(rpc.target_service_name, "svc-foo");
                        assert_eq!(rpc.target_endpoint_name, "DoRequest");
                        assert!(rpc.stack.is_none());
                    }
                    other => panic!("expected RpcCallStart, got {:?}", other),
                }
            }
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_body_stream() {
        let mut body = Vec::new();
        // Span event header
        body.push(0x00);
        body.push(0x00);
        body.push(0x00);
        // BodyStream: flags = 0b01 (is_response)
        body.push(0b01);
        // data = [0xDE, 0xAD]
        body.push(2);
        body.extend_from_slice(&[0xDE, 0xAD]);

        let data = build_event(BODY_STREAM, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::BodyStream(bs) => {
                    assert!(bs.is_response);
                    assert!(!bs.overflowed);
                    assert_eq!(bs.data, vec![0xDE, 0xAD]);
                }
                other => panic!("expected BodyStream, got {:?}", other),
            },
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_unknown_event_type() {
        let body = vec![0x00, 0x00, 0x00]; // minimal span event header
        let data = build_event(0xFF, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let result = parse_event(&mut cursor, &test_time_anchor(), 17);
        assert!(matches!(result, Err(ParseError::UnknownEventType(0xFF))));
    }

    #[test]
    fn test_parse_end_of_stream() {
        let data: &[u8] = &[];
        let mut cursor = std::io::Cursor::new(data);
        let result = parse_event(&mut cursor, &test_time_anchor(), 17);
        assert!(matches!(result, Err(ParseError::EndOfStream)));
    }

    #[test]
    fn test_parse_multiple_events() {
        // Build two events back-to-back
        let body1 = {
            let mut b = Vec::new();
            b.push(0x00);
            b.push(0x00);
            b.push(0x00);
            b.push(4);
            b.extend_from_slice(b"svc1");
            b
        };
        let body2 = {
            let mut b = Vec::new();
            b.push(0x00);
            b.push(0x00);
            b.push(0x00);
            b.push(4);
            b.extend_from_slice(b"svc2");
            b
        };

        let mut data = build_event(SERVICE_INIT_START, &body1);
        data.extend_from_slice(&build_event(SERVICE_INIT_START, &body2));

        let mut cursor = std::io::Cursor::new(&data);
        let ta = test_time_anchor();

        let ev1 = parse_event(&mut cursor, &ta, 17).unwrap();
        let ev2 = parse_event(&mut cursor, &ta, 17).unwrap();
        let ev3 = parse_event(&mut cursor, &ta, 17);

        // First event
        match &ev1.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::ServiceInitStart(s) => assert_eq!(s.service, "svc1"),
                _ => panic!("wrong event data"),
            },
            _ => panic!("wrong event type"),
        }

        // Second event
        match &ev2.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::ServiceInitStart(s) => assert_eq!(s.service, "svc2"),
                _ => panic!("wrong event data"),
            },
            _ => panic!("wrong event type"),
        }

        // Third read: end of stream
        assert!(matches!(ev3, Err(ParseError::EndOfStream)));
    }

    #[test]
    fn test_parse_request_span_start() {
        let mut body = Vec::new();
        // span_start_common: goid(1), parent_trace_id(zeros), parent_span_id(0),
        //   def_loc(0), caller_event_id(0), ext_correlation_id("")
        body.push(0x01); // goid = 1
        body.extend_from_slice(&[0u8; 16]); // parent trace ID (zeros)
        body.extend_from_slice(&0u64.to_le_bytes()); // parent span ID
        body.push(0x00); // def_loc = 0
        body.push(0x00); // caller_event_id = 0
        body.push(0x00); // ext_correlation_id = ""

        // RequestSpanStart fields
        body.push(3);
        body.extend_from_slice(b"svc"); // service_name
        body.push(2);
        body.extend_from_slice(b"Ep"); // endpoint_name
        body.push(3);
        body.extend_from_slice(b"GET"); // http_method
        body.push(5);
        body.extend_from_slice(b"/test"); // path
        body.push(0x00); // path_params count = 0
        body.push(0x00); // headers count = 0
        body.push(0x00); // request_payload = empty
        body.push(0x00); // ext_correlation_id = ""
        body.push(0x00); // uid = ""
        body.push(0x00); // mocked = false

        let data = build_event(REQUEST_SPAN_START, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanStart(ss) => {
                assert_eq!(ss.goid, 1);
                assert!(ss.parent_trace_id.is_none());
                match &ss.data {
                    SpanStartData::Request(req) => {
                        assert_eq!(req.service_name, "svc");
                        assert_eq!(req.endpoint_name, "Ep");
                        assert_eq!(req.http_method, "GET");
                        assert_eq!(req.path, "/test");
                        assert!(req.path_params.is_empty());
                        assert!(req.request_headers.is_empty());
                        assert!(!req.mocked);
                    }
                    other => panic!("expected Request, got {:?}", other),
                }
            }
            other => panic!("expected SpanStart, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_request_span_end() {
        let mut body = Vec::new();
        // span_end_common: duration (varint 5000 → uvarint 10000 → [0x90, 0x4E])
        body.push(0x90);
        body.push(0x4E);
        // version >= 17: status_code byte = 0 (OK)
        body.push(0x00);
        // error = "" (no error)
        body.push(0x00);
        // panic_stack = none
        body.push(0x00);
        // parent trace ID (zeros)
        body.extend_from_slice(&[0u8; 16]);
        // parent span ID = 0
        body.extend_from_slice(&0u64.to_le_bytes());

        // RequestSpanEnd fields
        body.push(3);
        body.extend_from_slice(b"svc");
        body.push(2);
        body.extend_from_slice(b"Ep");
        body.push(0xC8);
        body.push(0x01); // uvarint(200) = [0xC8, 0x01]
        body.push(0x00); // response headers count = 0
        body.push(0x00); // response payload = empty
        // version >= 16: caller_event_id
        body.push(0x00);
        // version >= 17: uid
        body.push(0x00);

        let data = build_event(REQUEST_SPAN_END, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEnd(se) => {
                assert_eq!(se.duration_nanos, 5000);
                assert_eq!(se.status_code, StatusCode::Ok);
                assert!(se.error.is_none());
                match &se.data {
                    SpanEndData::Request(req) => {
                        assert_eq!(req.service_name, "svc");
                        assert_eq!(req.endpoint_name, "Ep");
                        assert_eq!(req.http_status_code, 200);
                    }
                    other => panic!("expected Request, got {:?}", other),
                }
            }
            other => panic!("expected SpanEnd, got {:?}", other),
        }
    }

    #[test]
    fn test_parse_cache_call() {
        let mut body = Vec::new();
        // Span event header
        body.push(0x00);
        body.push(0x00);
        body.push(0x00);
        // CacheCallStart
        body.push(3);
        body.extend_from_slice(b"Get"); // operation
        body.push(0x00); // write = false
        body.push(0x00); // stack = none
        body.push(0x02); // 2 keys
        body.push(4);
        body.extend_from_slice(b"key1");
        body.push(4);
        body.extend_from_slice(b"key2");

        let data = build_event(CACHE_CALL_START, &body);
        let mut cursor = std::io::Cursor::new(&data);
        let event = parse_event(&mut cursor, &test_time_anchor(), 17).unwrap();

        match &event.event {
            Event::SpanEvent(se) => match &se.data {
                SpanEventData::CacheCallStart(cc) => {
                    assert_eq!(cc.operation, "Get");
                    assert!(!cc.write);
                    assert!(cc.stack.is_none());
                    assert_eq!(cc.keys, vec!["key1", "key2"]);
                }
                other => panic!("expected CacheCallStart, got {:?}", other),
            },
            other => panic!("expected SpanEvent, got {:?}", other),
        }
    }

    #[test]
    fn test_time_anchor_conversion() {
        let ta = TimeAnchor {
            real: Timestamp {
                seconds: 1000,
                nanos: 500_000_000,
            },
            mono_nanos: 100,
        };

        // Forward: nanotime=200 → delta=100ns
        let ts = ta.to_real(200);
        assert_eq!(ts.seconds, 1000);
        assert_eq!(ts.nanos, 500_000_100);

        // Backward: nanotime=50 → delta=-50ns
        let ts = ta.to_real(50);
        assert_eq!(ts.seconds, 1000);
        assert_eq!(ts.nanos, 499_999_950);
    }
}
