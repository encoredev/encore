use crate::types::{ParseError, Timestamp, TraceId};

/// Parsed event header.
pub(crate) struct Header {
    pub event_type: u8,
    pub event_id: u64,
    pub nanotime: i64,
    pub trace_id: TraceId,
    pub span_id: u64,
    pub data_len: u32,
}

const HEADER_REST_SIZE: usize = 44;

/// Read the event header from a stream reader.
/// Returns `ParseError::EndOfStream` if there are no more events (clean EOF).
pub(crate) fn read_header(reader: &mut impl std::io::Read) -> Result<Header, ParseError> {
    // Read the event type byte. EOF here means no more events.
    let mut type_byte = [0u8; 1];
    match reader.read_exact(&mut type_byte) {
        Ok(()) => {}
        Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
            return Err(ParseError::EndOfStream);
        }
        Err(e) => return Err(ParseError::Io(e)),
    }

    // Read the remaining 44 bytes of the header.
    let mut buf = [0u8; HEADER_REST_SIZE];
    reader.read_exact(&mut buf)?;

    let event_id = u64::from_le_bytes(buf[0..8].try_into().unwrap());
    let nanotime_raw = u64::from_le_bytes(buf[8..16].try_into().unwrap());
    let nanotime = zigzag_decode_i64(nanotime_raw);
    let trace_id_low = u64::from_le_bytes(buf[16..24].try_into().unwrap());
    let trace_id_high = u64::from_le_bytes(buf[24..32].try_into().unwrap());
    let span_id = u64::from_le_bytes(buf[32..40].try_into().unwrap());
    let data_len = u32::from_le_bytes(buf[40..44].try_into().unwrap());

    Ok(Header {
        event_type: type_byte[0],
        event_id,
        nanotime,
        trace_id: TraceId {
            high: trace_id_high,
            low: trace_id_low,
        },
        span_id,
        data_len,
    })
}

/// Read the event body from a stream reader.
pub(crate) fn read_body(
    reader: &mut impl std::io::Read,
    len: u32,
) -> Result<Vec<u8>, ParseError> {
    let mut body = vec![0u8; len as usize];
    reader.read_exact(&mut body)?;
    Ok(body)
}

/// A cursor-based reader over a byte slice for parsing event data.
///
/// Uses "sticky error" semantics: once an error occurs, all subsequent reads
/// return zero/default values. The error is checked after parsing completes.
pub(crate) struct EventReader<'a> {
    data: &'a [u8],
    pos: usize,
    pub version: u16,
    err: bool,
}

impl<'a> EventReader<'a> {
    pub fn new(data: &'a [u8], version: u16) -> Self {
        Self {
            data,
            pos: 0,
            version,
            err: false,
        }
    }

    pub fn has_error(&self) -> bool {
        self.err
    }

    #[allow(dead_code)]
    pub fn bytes_read(&self) -> usize {
        self.pos
    }

    fn set_err(&mut self) {
        self.err = true;
    }

    fn ensure(&mut self, n: usize) -> bool {
        if self.err || self.pos + n > self.data.len() {
            self.set_err();
            false
        } else {
            true
        }
    }

    /// Read n bytes as a slice from the data.
    fn read_bytes_slice(&mut self, n: usize) -> &'a [u8] {
        if !self.ensure(n) {
            return &[];
        }
        let start = self.pos;
        self.pos += n;
        &self.data[start..self.pos]
    }

    /// Read a single byte.
    pub fn byte(&mut self) -> u8 {
        if !self.ensure(1) {
            return 0;
        }
        let b = self.data[self.pos];
        self.pos += 1;
        b
    }

    /// Read a boolean (single byte, 0 = false).
    pub fn bool_val(&mut self) -> bool {
        self.byte() != 0
    }

    /// Read n bytes into a new Vec.
    pub fn bytes(&mut self, n: usize) -> Vec<u8> {
        self.read_bytes_slice(n).to_vec()
    }

    /// Skip n bytes.
    #[allow(dead_code)]
    pub fn skip(&mut self, n: usize) {
        if !self.ensure(n) {
            return;
        }
        self.pos += n;
    }

    /// Read a little-endian u32.
    pub fn uint32(&mut self) -> u32 {
        let b = self.read_bytes_slice(4);
        if b.len() < 4 {
            return 0;
        }
        u32::from_le_bytes(b.try_into().unwrap())
    }

    /// Read a little-endian u64.
    pub fn uint64(&mut self) -> u64 {
        let b = self.read_bytes_slice(8);
        if b.len() < 8 {
            return 0;
        }
        u64::from_le_bytes(b.try_into().unwrap())
    }

    /// Read a zigzag-encoded little-endian i32.
    pub fn int32(&mut self) -> i32 {
        let u = self.uint32();
        zigzag_decode_i32(u)
    }

    /// Read a zigzag-encoded little-endian i64.
    pub fn int64(&mut self) -> i64 {
        let u = self.uint64();
        zigzag_decode_i64(u)
    }

    /// Read a variable-length unsigned integer.
    pub fn uvarint(&mut self) -> u64 {
        let mut result: u64 = 0;
        let mut shift: u32 = 0;
        loop {
            if self.err {
                return 0;
            }
            let b = self.byte();
            if self.err {
                return 0;
            }
            result |= ((b & 0x7F) as u64) << shift;
            if b & 0x80 == 0 {
                return result;
            }
            shift += 7;
            if shift >= 64 {
                self.set_err();
                return 0;
            }
        }
    }

    /// Read a zigzag-encoded variable-length signed integer.
    pub fn varint(&mut self) -> i64 {
        let u = self.uvarint();
        zigzag_decode_i64(u)
    }

    /// Read a little-endian f32.
    pub fn float32(&mut self) -> f32 {
        f32::from_bits(self.uint32())
    }

    /// Read a little-endian f64.
    pub fn float64(&mut self) -> f64 {
        f64::from_bits(self.uint64())
    }

    /// Read a length-prefixed UTF-8 string. Invalid UTF-8 is replaced.
    pub fn string(&mut self) -> String {
        let len = self.uvarint() as usize;
        if len == 0 {
            return String::new();
        }
        let bytes = self.read_bytes_slice(len);
        if self.err {
            return String::new();
        }
        String::from_utf8_lossy(bytes).into_owned()
    }

    /// Read a length-prefixed byte string.
    pub fn byte_string(&mut self) -> Vec<u8> {
        let len = self.uvarint() as usize;
        if len == 0 {
            return Vec::new();
        }
        self.read_bytes_slice(len).to_vec()
    }

    /// Read a string, returning None if empty.
    pub fn opt_string(&mut self) -> Option<String> {
        let s = self.string();
        if s.is_empty() {
            None
        } else {
            Some(s)
        }
    }

    /// Read a uvarint, returning None if zero.
    pub fn opt_uvarint(&mut self) -> Option<u64> {
        let v = self.uvarint();
        if v == 0 {
            None
        } else {
            Some(v)
        }
    }

    /// Read a varint duration (nanoseconds).
    pub fn duration(&mut self) -> i64 {
        self.varint()
    }

    /// Read a timestamp (i64 seconds + i32 nanoseconds).
    pub fn time(&mut self) -> Timestamp {
        let sec = self.int64();
        let nsec = self.int32();
        // Normalize nanoseconds to [0, 999_999_999].
        let total_nanos = nsec as i64;
        let extra_secs = total_nanos.div_euclid(1_000_000_000);
        let norm_nanos = total_nanos.rem_euclid(1_000_000_000);
        Timestamp {
            seconds: sec + extra_secs,
            nanos: norm_nanos as i32,
        }
    }

    /// Read an event ID (uvarint).
    pub fn event_id(&mut self) -> u64 {
        self.uvarint()
    }

    /// Read a trace ID (16 bytes: low u64 LE + high u64 LE).
    pub fn trace_id(&mut self) -> TraceId {
        let b = self.read_bytes_slice(16);
        if b.len() < 16 {
            return TraceId { high: 0, low: 0 };
        }
        TraceId {
            low: u64::from_le_bytes(b[0..8].try_into().unwrap()),
            high: u64::from_le_bytes(b[8..16].try_into().unwrap()),
        }
    }
}

/// Zigzag decode a u64 to i64.
fn zigzag_decode_i64(u: u64) -> i64 {
    if u & 1 == 0 {
        (u >> 1) as i64
    } else {
        !((u >> 1) as i64)
    }
}

/// Zigzag decode a u32 to i32.
fn zigzag_decode_i32(u: u32) -> i32 {
    if u & 1 == 0 {
        (u >> 1) as i32
    } else {
        !((u >> 1) as i32)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_zigzag_decode_i64() {
        assert_eq!(zigzag_decode_i64(0), 0);
        assert_eq!(zigzag_decode_i64(1), -1);
        assert_eq!(zigzag_decode_i64(2), 1);
        assert_eq!(zigzag_decode_i64(3), -2);
        assert_eq!(zigzag_decode_i64(4), 2);
        assert_eq!(zigzag_decode_i64(4294967294), 2147483647);
        assert_eq!(zigzag_decode_i64(4294967295), -2147483648);
    }

    #[test]
    fn test_zigzag_decode_i32() {
        assert_eq!(zigzag_decode_i32(0), 0);
        assert_eq!(zigzag_decode_i32(1), -1);
        assert_eq!(zigzag_decode_i32(2), 1);
        assert_eq!(zigzag_decode_i32(3), -2);
    }

    #[test]
    fn test_reader_byte() {
        let data = [0x42, 0xFF];
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.byte(), 0x42);
        assert_eq!(r.byte(), 0xFF);
        assert!(!r.has_error());
        // Reading past end sets error
        assert_eq!(r.byte(), 0);
        assert!(r.has_error());
    }

    #[test]
    fn test_reader_bool() {
        let data = [0x00, 0x01, 0xFF];
        let mut r = EventReader::new(&data, 17);
        assert!(!r.bool_val());
        assert!(r.bool_val());
        assert!(r.bool_val());
    }

    #[test]
    fn test_reader_uint32() {
        let data = 42u32.to_le_bytes();
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.uint32(), 42);
    }

    #[test]
    fn test_reader_uint64() {
        let data = 123456789u64.to_le_bytes();
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.uint64(), 123456789);
    }

    #[test]
    fn test_reader_uvarint() {
        // 0 => [0x00]
        let mut r = EventReader::new(&[0x00], 17);
        assert_eq!(r.uvarint(), 0);

        // 1 => [0x01]
        let mut r = EventReader::new(&[0x01], 17);
        assert_eq!(r.uvarint(), 1);

        // 127 => [0x7F]
        let mut r = EventReader::new(&[0x7F], 17);
        assert_eq!(r.uvarint(), 127);

        // 128 => [0x80, 0x01]
        let mut r = EventReader::new(&[0x80, 0x01], 17);
        assert_eq!(r.uvarint(), 128);

        // 300 => [0xAC, 0x02]
        let mut r = EventReader::new(&[0xAC, 0x02], 17);
        assert_eq!(r.uvarint(), 300);
    }

    #[test]
    fn test_reader_varint() {
        // 0 => uvarint(0)
        let mut r = EventReader::new(&[0x00], 17);
        assert_eq!(r.varint(), 0);

        // -1 => uvarint(1)
        let mut r = EventReader::new(&[0x01], 17);
        assert_eq!(r.varint(), -1);

        // 1 => uvarint(2)
        let mut r = EventReader::new(&[0x02], 17);
        assert_eq!(r.varint(), 1);

        // -2 => uvarint(3)
        let mut r = EventReader::new(&[0x03], 17);
        assert_eq!(r.varint(), -2);
    }

    #[test]
    fn test_reader_string() {
        // Length 5, then "hello"
        let data = [0x05, b'h', b'e', b'l', b'l', b'o'];
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.string(), "hello");

        // Empty string (length 0)
        let mut r = EventReader::new(&[0x00], 17);
        assert_eq!(r.string(), "");
    }

    #[test]
    fn test_reader_string_invalid_utf8() {
        // Length 3, then invalid UTF-8 bytes
        let data = [0x03, 0xFF, 0xFE, 0xFD];
        let mut r = EventReader::new(&data, 17);
        let s = r.string();
        assert!(!r.has_error());
        // Should contain replacement characters
        assert!(s.contains('\u{FFFD}'));
    }

    #[test]
    fn test_reader_byte_string() {
        let data = [0x03, 0x01, 0x02, 0x03];
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.byte_string(), vec![0x01, 0x02, 0x03]);

        // Empty byte string
        let mut r = EventReader::new(&[0x00], 17);
        assert!(r.byte_string().is_empty());
    }

    #[test]
    fn test_reader_float32() {
        let val: f32 = 3.14;
        let data = val.to_bits().to_le_bytes();
        let mut r = EventReader::new(&data, 17);
        assert!((r.float32() - 3.14).abs() < 0.001);
    }

    #[test]
    fn test_reader_float64() {
        let val: f64 = 3.14159265;
        let data = val.to_bits().to_le_bytes();
        let mut r = EventReader::new(&data, 17);
        assert!((r.float64() - 3.14159265).abs() < 0.0000001);
    }

    #[test]
    fn test_reader_trace_id() {
        let mut data = [0u8; 16];
        data[0..8].copy_from_slice(&42u64.to_le_bytes()); // low
        data[8..16].copy_from_slice(&99u64.to_le_bytes()); // high
        let mut r = EventReader::new(&data, 17);
        let tid = r.trace_id();
        assert_eq!(tid.low, 42);
        assert_eq!(tid.high, 99);
    }

    #[test]
    fn test_reader_opt_string() {
        // Non-empty → Some
        let data = [0x03, b'a', b'b', b'c'];
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.opt_string(), Some("abc".to_string()));

        // Empty → None
        let mut r = EventReader::new(&[0x00], 17);
        assert_eq!(r.opt_string(), None);
    }

    #[test]
    fn test_reader_opt_uvarint() {
        let mut r = EventReader::new(&[0x05], 17);
        assert_eq!(r.opt_uvarint(), Some(5));

        let mut r = EventReader::new(&[0x00], 17);
        assert_eq!(r.opt_uvarint(), None);
    }

    #[test]
    fn test_reader_time() {
        // seconds: zigzag(1000) = 2000 as u64 LE
        // nanos: zigzag(500) = 1000 as u32 LE
        let mut data = Vec::new();
        data.extend_from_slice(&2000u64.to_le_bytes()); // zigzag(1000)
        data.extend_from_slice(&1000u32.to_le_bytes()); // zigzag(500)
        let mut r = EventReader::new(&data, 17);
        let ts = r.time();
        assert_eq!(ts.seconds, 1000);
        assert_eq!(ts.nanos, 500);
    }

    #[test]
    fn test_sticky_error() {
        let data = [0x42];
        let mut r = EventReader::new(&data, 17);
        assert_eq!(r.byte(), 0x42);
        assert!(!r.has_error());

        // This should fail and set sticky error
        assert_eq!(r.byte(), 0);
        assert!(r.has_error());

        // All subsequent reads should also return defaults
        assert_eq!(r.uint32(), 0);
        assert!(r.has_error());
        assert_eq!(r.string(), "");
    }

    #[test]
    fn test_read_header() {
        let mut data = Vec::new();
        // Type byte
        data.push(0x12); // ServiceInitStart
        // EventID: 1 as u64 LE
        data.extend_from_slice(&1u64.to_le_bytes());
        // Nanotime: zigzag(100) = 200 as u64 LE
        data.extend_from_slice(&200u64.to_le_bytes());
        // TraceID: low=10, high=20
        data.extend_from_slice(&10u64.to_le_bytes());
        data.extend_from_slice(&20u64.to_le_bytes());
        // SpanID: 5 as u64 LE
        data.extend_from_slice(&5u64.to_le_bytes());
        // DataLen: 8 as u32 LE
        data.extend_from_slice(&8u32.to_le_bytes());

        let mut cursor = std::io::Cursor::new(&data);
        let header = read_header(&mut cursor).unwrap();
        assert_eq!(header.event_type, 0x12);
        assert_eq!(header.event_id, 1);
        assert_eq!(header.nanotime, 100);
        assert_eq!(header.trace_id.low, 10);
        assert_eq!(header.trace_id.high, 20);
        assert_eq!(header.span_id, 5);
        assert_eq!(header.data_len, 8);
    }

    #[test]
    fn test_read_header_eof() {
        let data: &[u8] = &[];
        let mut cursor = std::io::Cursor::new(data);
        let result = read_header(&mut cursor);
        assert!(matches!(result, Err(ParseError::EndOfStream)));
    }
}
