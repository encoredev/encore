use std::collections::HashMap;

// === Error types ===

/// Errors that can occur during trace event parsing.
#[derive(Debug, thiserror::Error)]
pub enum ParseError {
    /// Reached end of stream at a clean event boundary (no more events).
    #[error("end of stream")]
    EndOfStream,

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("unknown event type: 0x{0:02x}")]
    UnknownEventType(u8),

    #[error("unexpected end of event data")]
    UnexpectedEof,

    #[error("parse error: {0}")]
    InvalidData(String),
}

// === Basic types ===

/// A timestamp represented as seconds and nanoseconds since the Unix epoch.
#[derive(Debug, Clone, PartialEq)]
pub struct Timestamp {
    pub seconds: i64,
    pub nanos: i32,
}

/// A 128-bit trace identifier.
#[derive(Debug, Clone, PartialEq)]
pub struct TraceId {
    pub high: u64,
    pub low: u64,
}

impl TraceId {
    pub fn is_zero(&self) -> bool {
        self.high == 0 && self.low == 0
    }
}

/// Converts monotonic nanotimes to real wall-clock timestamps.
#[derive(Debug, Clone)]
pub struct TimeAnchor {
    pub real: Timestamp,
    pub mono_nanos: i64,
}

impl TimeAnchor {
    /// Convert a monotonic nanotime to a real wall-clock timestamp.
    pub fn to_real(&self, nanotime: i64) -> Timestamp {
        let delta_nanos = nanotime - self.mono_nanos;
        let total_nanos = self.real.nanos as i64 + delta_nanos;
        let extra_seconds = total_nanos.div_euclid(1_000_000_000);
        let remaining_nanos = total_nanos.rem_euclid(1_000_000_000);
        Timestamp {
            seconds: self.real.seconds + extra_seconds,
            nanos: remaining_nanos as i32,
        }
    }
}

/// A stack trace with optional PC values and formatted frames.
#[derive(Debug, Clone, PartialEq)]
pub struct StackTrace {
    /// Delta-encoded program counter values.
    pub pcs: Vec<i64>,
    /// Formatted stack frames (optional).
    pub frames: Vec<StackFrame>,
}

/// A single frame in a stack trace.
#[derive(Debug, Clone, PartialEq)]
pub struct StackFrame {
    pub filename: String,
    pub line: i32,
    pub func_name: String,
}

/// An error with an optional stack trace.
#[derive(Debug, Clone, PartialEq)]
pub struct TracedError {
    pub msg: String,
    pub stack: Option<StackTrace>,
}

// === Status codes ===

/// gRPC-style status codes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum StatusCode {
    Ok = 0,
    Canceled = 1,
    Unknown = 2,
    InvalidArgument = 3,
    DeadlineExceeded = 4,
    NotFound = 5,
    AlreadyExists = 6,
    PermissionDenied = 7,
    ResourceExhausted = 8,
    FailedPrecondition = 9,
    Aborted = 10,
    OutOfRange = 11,
    Unimplemented = 12,
    Internal = 13,
    Unavailable = 14,
    DataLoss = 15,
    Unauthenticated = 16,
}

impl StatusCode {
    pub(crate) fn from_byte(b: u8) -> Self {
        match b {
            0 => Self::Ok,
            1 => Self::Canceled,
            2 => Self::Unknown,
            3 => Self::InvalidArgument,
            4 => Self::DeadlineExceeded,
            5 => Self::NotFound,
            6 => Self::AlreadyExists,
            7 => Self::PermissionDenied,
            8 => Self::ResourceExhausted,
            9 => Self::FailedPrecondition,
            10 => Self::Aborted,
            11 => Self::OutOfRange,
            12 => Self::Unimplemented,
            13 => Self::Internal,
            14 => Self::Unavailable,
            15 => Self::DataLoss,
            16 => Self::Unauthenticated,
            _ => Self::Unknown,
        }
    }
}

// === Top-level event types ===

/// A parsed trace event.
#[derive(Debug, Clone, PartialEq)]
pub struct TraceEvent {
    pub trace_id: TraceId,
    pub span_id: u64,
    pub event_id: u64,
    pub event_time: Timestamp,
    pub event: Event,
}

/// The type of a trace event.
#[derive(Debug, Clone, PartialEq)]
pub enum Event {
    SpanStart(SpanStart),
    SpanEnd(SpanEnd),
    SpanEvent(SpanEvent),
}

// === Span start ===

#[derive(Debug, Clone, PartialEq)]
pub struct SpanStart {
    pub goid: u32,
    pub parent_trace_id: Option<TraceId>,
    pub parent_span_id: Option<u64>,
    pub def_loc: Option<u32>,
    pub caller_event_id: Option<u64>,
    pub external_correlation_id: Option<String>,
    pub data: SpanStartData,
}

#[derive(Debug, Clone, PartialEq)]
pub enum SpanStartData {
    Request(RequestSpanStart),
    Auth(AuthSpanStart),
    PubsubMessage(PubsubMessageSpanStart),
    Test(TestSpanStart),
}

#[derive(Debug, Clone, PartialEq)]
pub struct RequestSpanStart {
    pub service_name: String,
    pub endpoint_name: String,
    pub http_method: String,
    pub path: String,
    pub path_params: Vec<String>,
    pub request_headers: HashMap<String, String>,
    pub request_payload: Vec<u8>,
    pub ext_correlation_id: Option<String>,
    pub uid: Option<String>,
    pub mocked: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuthSpanStart {
    pub service_name: String,
    pub endpoint_name: String,
    pub auth_payload: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PubsubMessageSpanStart {
    pub service_name: String,
    pub topic_name: String,
    pub subscription_name: String,
    pub message_id: String,
    pub attempt: u32,
    pub publish_time: Timestamp,
    pub message_payload: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TestSpanStart {
    pub service_name: String,
    pub test_name: String,
    pub uid: String,
    pub test_file: String,
    pub test_line: u32,
}

// === Span end ===

#[derive(Debug, Clone, PartialEq)]
pub struct SpanEnd {
    pub duration_nanos: u64,
    pub status_code: StatusCode,
    pub error: Option<TracedError>,
    pub panic_stack: Option<StackTrace>,
    pub parent_trace_id: Option<TraceId>,
    pub parent_span_id: Option<u64>,
    pub data: SpanEndData,
}

#[derive(Debug, Clone, PartialEq)]
pub enum SpanEndData {
    Request(RequestSpanEnd),
    Auth(AuthSpanEnd),
    PubsubMessage(PubsubMessageSpanEnd),
    Test(TestSpanEnd),
}

#[derive(Debug, Clone, PartialEq)]
pub struct RequestSpanEnd {
    pub service_name: String,
    pub endpoint_name: String,
    pub http_status_code: u32,
    pub response_headers: HashMap<String, String>,
    pub response_payload: Vec<u8>,
    pub caller_event_id: Option<u64>,
    pub uid: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuthSpanEnd {
    pub service_name: String,
    pub endpoint_name: String,
    pub uid: String,
    pub user_data: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PubsubMessageSpanEnd {
    pub service_name: String,
    pub topic_name: String,
    pub subscription_name: String,
    pub message_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TestSpanEnd {
    pub service_name: String,
    pub test_name: String,
    pub failed: bool,
    pub skipped: bool,
    pub uid: Option<String>,
}

// === Span events ===

#[derive(Debug, Clone, PartialEq)]
pub struct SpanEvent {
    pub goid: u32,
    pub def_loc: Option<u32>,
    pub correlation_event_id: Option<u64>,
    pub data: SpanEventData,
}

#[derive(Debug, Clone, PartialEq)]
pub enum SpanEventData {
    RpcCallStart(RpcCallStart),
    RpcCallEnd(RpcCallEnd),
    DbQueryStart(DbQueryStart),
    DbQueryEnd(DbQueryEnd),
    DbTransactionStart(DbTransactionStart),
    DbTransactionEnd(DbTransactionEnd),
    PubsubPublishStart(PubsubPublishStart),
    PubsubPublishEnd(PubsubPublishEnd),
    HttpCallStart(HttpCallStart),
    HttpCallEnd(HttpCallEnd),
    LogMessage(LogMessage),
    ServiceInitStart(ServiceInitStart),
    ServiceInitEnd(ServiceInitEnd),
    CacheCallStart(CacheCallStart),
    CacheCallEnd(CacheCallEnd),
    BodyStream(BodyStream),
    BucketObjectUploadStart(BucketObjectUploadStart),
    BucketObjectUploadEnd(BucketObjectUploadEnd),
    BucketObjectDownloadStart(BucketObjectDownloadStart),
    BucketObjectDownloadEnd(BucketObjectDownloadEnd),
    BucketObjectGetAttrsStart(BucketObjectGetAttrsStart),
    BucketObjectGetAttrsEnd(BucketObjectGetAttrsEnd),
    BucketListObjectsStart(BucketListObjectsStart),
    BucketListObjectsEnd(BucketListObjectsEnd),
    BucketDeleteObjectsStart(BucketDeleteObjectsStart),
    BucketDeleteObjectsEnd(BucketDeleteObjectsEnd),
}

// === RPC types ===

#[derive(Debug, Clone, PartialEq)]
pub struct RpcCallStart {
    pub target_service_name: String,
    pub target_endpoint_name: String,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RpcCallEnd {
    pub err: Option<TracedError>,
}

// === DB types ===

#[derive(Debug, Clone, PartialEq)]
pub struct DbQueryStart {
    pub query: String,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DbQueryEnd {
    pub err: Option<TracedError>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DbTransactionStart {
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DbTransactionCompletion {
    Rollback,
    Commit,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DbTransactionEnd {
    pub completion: DbTransactionCompletion,
    pub stack: Option<StackTrace>,
    pub err: Option<TracedError>,
}

// === Pubsub types ===

#[derive(Debug, Clone, PartialEq)]
pub struct PubsubPublishStart {
    pub topic: String,
    pub message: Vec<u8>,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PubsubPublishEnd {
    pub message_id: Option<String>,
    pub err: Option<TracedError>,
}

// === Service init types ===

#[derive(Debug, Clone, PartialEq)]
pub struct ServiceInitStart {
    pub service: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ServiceInitEnd {
    pub err: Option<TracedError>,
}

// === HTTP call types ===

#[derive(Debug, Clone, PartialEq)]
pub struct HttpCallStart {
    pub correlation_parent_span_id: u64,
    pub method: String,
    pub url: String,
    pub stack: Option<StackTrace>,
    pub start_nanotime: i64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpCallEnd {
    pub status_code: Option<u32>,
    pub err: Option<TracedError>,
    pub trace_events: Vec<HttpTraceEvent>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpTraceEvent {
    pub nanotime: i64,
    pub data: HttpTraceEventData,
}

#[derive(Debug, Clone, PartialEq)]
pub enum HttpTraceEventData {
    GetConn(HttpGetConn),
    GotConn(HttpGotConn),
    GotFirstResponseByte,
    Got1xxResponse(HttpGot1xxResponse),
    DnsStart(HttpDnsStart),
    DnsDone(HttpDnsDone),
    ConnectStart(HttpConnectStart),
    ConnectDone(HttpConnectDone),
    TlsHandshakeStart,
    TlsHandshakeDone(HttpTlsHandshakeDone),
    WroteHeaders,
    WroteRequest(HttpWroteRequest),
    Wait100Continue,
    ClosedBody(HttpClosedBody),
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpGetConn {
    pub host_port: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpGotConn {
    pub reused: bool,
    pub was_idle: bool,
    pub idle_duration_ns: i64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpGot1xxResponse {
    pub code: i32,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpDnsStart {
    pub host: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpDnsDone {
    pub err: Vec<u8>,
    pub addrs: Vec<DnsAddr>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DnsAddr {
    pub ip: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpConnectStart {
    pub network: String,
    pub addr: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpConnectDone {
    pub network: String,
    pub addr: String,
    pub err: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpTlsHandshakeDone {
    pub err: Vec<u8>,
    pub tls_version: u32,
    pub cipher_suite: u32,
    pub server_name: String,
    pub negotiated_protocol: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpWroteRequest {
    pub err: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HttpClosedBody {
    pub err: Vec<u8>,
}

// === Cache types ===

#[derive(Debug, Clone, PartialEq)]
pub struct CacheCallStart {
    pub operation: String,
    pub write: bool,
    pub stack: Option<StackTrace>,
    pub keys: Vec<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CacheResult {
    Unknown,
    Ok,
    NoSuchKey,
    Conflict,
    Err,
}

impl CacheResult {
    pub(crate) fn from_byte(b: u8) -> Self {
        match b {
            1 => Self::Ok,
            2 => Self::NoSuchKey,
            3 => Self::Conflict,
            4 => Self::Err,
            _ => Self::Unknown,
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct CacheCallEnd {
    pub result: CacheResult,
    pub err: Option<TracedError>,
}

// === Body stream ===

#[derive(Debug, Clone, PartialEq)]
pub struct BodyStream {
    pub is_response: bool,
    pub overflowed: bool,
    pub data: Vec<u8>,
}

// === Log types ===

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogLevel {
    Trace,
    Debug,
    Info,
    Warn,
    Error,
}

impl LogLevel {
    /// Parse from the binary wire format byte value.
    pub(crate) fn from_wire_byte(b: u8) -> Self {
        match b {
            0 => Self::Trace,
            1 => Self::Debug,
            2 => Self::Info,
            3 => Self::Warn,
            4 => Self::Error,
            _ => Self::Trace,
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct LogMessage {
    pub level: LogLevel,
    pub msg: String,
    pub fields: Vec<LogField>,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct LogField {
    pub key: String,
    pub value: LogFieldValue,
}

#[derive(Debug, Clone, PartialEq)]
pub enum LogFieldValue {
    Error(TracedError),
    Str(String),
    Bool(bool),
    Time(Timestamp),
    Duration(i64),
    Uuid(Vec<u8>),
    Json(Vec<u8>),
    Int(i64),
    Uint(u64),
    Float32(f32),
    Float64(f64),
}

// === Bucket types ===

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectAttributes {
    pub size: Option<u64>,
    pub version: Option<String>,
    pub etag: Option<String>,
    pub content_type: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectUploadStart {
    pub bucket: String,
    pub object: String,
    pub attrs: BucketObjectAttributes,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectUploadEnd {
    pub size: Option<u64>,
    pub version: Option<String>,
    pub err: Option<TracedError>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectDownloadStart {
    pub bucket: String,
    pub object: String,
    pub version: Option<String>,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectDownloadEnd {
    pub size: Option<u64>,
    pub err: Option<TracedError>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectGetAttrsStart {
    pub bucket: String,
    pub object: String,
    pub version: Option<String>,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketObjectGetAttrsEnd {
    pub err: Option<TracedError>,
    pub attrs: Option<BucketObjectAttributes>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketListObjectsStart {
    pub bucket: String,
    pub prefix: Option<String>,
    pub stack: Option<StackTrace>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketListObjectsEnd {
    pub err: Option<TracedError>,
    pub observed: u64,
    pub has_more: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketDeleteObjectEntry {
    pub object: String,
    pub version: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketDeleteObjectsStart {
    pub bucket: String,
    pub stack: Option<StackTrace>,
    pub entries: Vec<BucketDeleteObjectEntry>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BucketDeleteObjectsEnd {
    pub err: Option<TracedError>,
}
