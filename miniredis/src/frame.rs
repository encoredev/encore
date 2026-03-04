use bytes::{Buf, Bytes};
use std::fmt;
use std::io::Cursor;

/// A RESP2/RESP3 protocol frame.
#[derive(Clone, Debug, PartialEq)]
pub enum Frame {
    /// Simple string: `+OK\r\n`
    Simple(String),
    /// Error: `-ERR message\r\n`
    Error(String),
    /// Integer: `:42\r\n`
    Integer(i64),
    /// Bulk string: `$5\r\nhello\r\n`
    Bulk(Bytes),
    /// Null: `$-1\r\n` (RESP2) or `_\r\n` (RESP3)
    Null,
    /// Array: `*2\r\n...`
    Array(Vec<Frame>),
    // ── RESP3 types ──────────────────────────────────────────────────
    /// Map: `%N\r\n...` (RESP3) or flat array `*2N\r\n...` (RESP2 fallback)
    Map(Vec<(Frame, Frame)>),
    /// Set: `~N\r\n...` (RESP3) or array `*N\r\n...` (RESP2 fallback)
    Set(Vec<Frame>),
    /// Push: `>N\r\n...` (RESP3) or array `*N\r\n...` (RESP2 fallback)
    Push(Vec<Frame>),
    /// Double: `,3.14\r\n` (RESP3) or bulk string (RESP2 fallback)
    Double(f64),
}

/// Errors that can occur during frame parsing.
#[derive(Debug)]
pub enum FrameError {
    /// Not enough data is available to parse a full frame.
    Incomplete,
    /// Invalid frame data.
    Protocol(String),
}

impl Frame {
    // ── Response builder helpers ──────────────────────────────────────

    /// `+OK\r\n`
    pub fn ok() -> Frame {
        Frame::Simple("OK".into())
    }

    /// `-{msg}\r\n`
    pub fn error(msg: impl Into<String>) -> Frame {
        Frame::Error(msg.into())
    }

    /// `:{n}\r\n`
    pub fn integer(n: i64) -> Frame {
        Frame::Integer(n)
    }

    /// Bulk string from bytes.
    pub fn bulk(data: impl Into<Bytes>) -> Frame {
        Frame::Bulk(data.into())
    }

    /// Bulk string from a str.
    pub fn bulk_string(s: &str) -> Frame {
        Frame::Bulk(Bytes::from(s.to_owned()))
    }

    /// `$-1\r\n`
    pub fn null() -> Frame {
        Frame::Null
    }

    /// Build an Array of bulk strings.
    pub fn strings(strs: &[&str]) -> Frame {
        Frame::Array(
            strs.iter()
                .map(|s| Frame::Bulk(Bytes::from(s.to_string())))
                .collect(),
        )
    }

    // ── Validation (no allocation) ───────────────────────────────────

    /// Check whether a complete frame can be read from `src` without
    /// allocating. Advances the cursor past the frame on success.
    pub fn check(src: &mut Cursor<&[u8]>) -> Result<(), FrameError> {
        match get_u8(src)? {
            b'+' | b'-' | b':' => {
                skip_line(src)?;
                Ok(())
            }
            b'$' => {
                let len = get_line_as_int(src)?;
                if len < 0 {
                    // Null bulk string `$-1\r\n`
                    Ok(())
                } else {
                    // Skip `len` bytes + `\r\n`
                    let len = len as usize;
                    skip(src, len + 2)?;
                    Ok(())
                }
            }
            b'*' => {
                let count = get_line_as_int(src)?;
                if count < 0 {
                    // Null array
                    Ok(())
                } else {
                    for _ in 0..count {
                        Frame::check(src)?;
                    }
                    Ok(())
                }
            }
            // RESP3: Map
            b'%' => {
                let count = get_line_as_int(src)?;
                for _ in 0..count {
                    Frame::check(src)?; // key
                    Frame::check(src)?; // value
                }
                Ok(())
            }
            // RESP3: Set or Push
            b'~' | b'>' => {
                let count = get_line_as_int(src)?;
                for _ in 0..count {
                    Frame::check(src)?;
                }
                Ok(())
            }
            // RESP3: Double
            b',' => {
                skip_line(src)?;
                Ok(())
            }
            // RESP3: Null
            b'_' => {
                skip_line(src)?;
                Ok(())
            }
            b => Err(FrameError::Protocol(format!(
                "invalid frame type byte: `{}`",
                b as char
            ))),
        }
    }

    // ── Parsing (allocates) ──────────────────────────────────────────

    /// Parse a single frame from `src`.
    pub fn parse(src: &mut Cursor<&[u8]>) -> Result<Frame, FrameError> {
        match get_u8(src)? {
            b'+' => {
                let line = get_line(src)?;
                let s = String::from_utf8(line.to_vec())
                    .map_err(|e| FrameError::Protocol(e.to_string()))?;
                Ok(Frame::Simple(s))
            }
            b'-' => {
                let line = get_line(src)?;
                let s = String::from_utf8(line.to_vec())
                    .map_err(|e| FrameError::Protocol(e.to_string()))?;
                Ok(Frame::Error(s))
            }
            b':' => {
                let n = get_line_as_int(src)?;
                Ok(Frame::Integer(n))
            }
            b'$' => {
                let len = get_line_as_int(src)?;
                if len < 0 {
                    Ok(Frame::Null)
                } else {
                    let len = len as usize;
                    if src.remaining() < len + 2 {
                        return Err(FrameError::Incomplete);
                    }
                    let data =
                        Bytes::copy_from_slice(&src.get_ref()[src.position() as usize..][..len]);
                    skip(src, len + 2)?;
                    Ok(Frame::Bulk(data))
                }
            }
            b'*' => {
                let count = get_line_as_int(src)?;
                if count < 0 {
                    Ok(Frame::Null)
                } else {
                    let mut frames = Vec::with_capacity(count as usize);
                    for _ in 0..count {
                        frames.push(Frame::parse(src)?);
                    }
                    Ok(Frame::Array(frames))
                }
            }
            // RESP3: Map
            b'%' => {
                let count = get_line_as_int(src)?;
                let mut pairs = Vec::with_capacity(count as usize);
                for _ in 0..count {
                    let key = Frame::parse(src)?;
                    let value = Frame::parse(src)?;
                    pairs.push((key, value));
                }
                Ok(Frame::Map(pairs))
            }
            // RESP3: Set
            b'~' => {
                let count = get_line_as_int(src)?;
                let mut items = Vec::with_capacity(count as usize);
                for _ in 0..count {
                    items.push(Frame::parse(src)?);
                }
                Ok(Frame::Set(items))
            }
            // RESP3: Push
            b'>' => {
                let count = get_line_as_int(src)?;
                let mut items = Vec::with_capacity(count as usize);
                for _ in 0..count {
                    items.push(Frame::parse(src)?);
                }
                Ok(Frame::Push(items))
            }
            // RESP3: Double
            b',' => {
                let line = get_line(src)?;
                let s =
                    std::str::from_utf8(line).map_err(|e| FrameError::Protocol(e.to_string()))?;
                let f: f64 = match s {
                    "inf" => f64::INFINITY,
                    "-inf" => f64::NEG_INFINITY,
                    "nan" => f64::NAN,
                    _ => s.parse().map_err(|e: std::num::ParseFloatError| {
                        FrameError::Protocol(e.to_string())
                    })?,
                };
                Ok(Frame::Double(f))
            }
            // RESP3: Null
            b'_' => {
                skip_line(src)?;
                Ok(Frame::Null)
            }
            b => Err(FrameError::Protocol(format!(
                "invalid frame type byte: `{}`",
                b as char
            ))),
        }
    }

    // ── Serialization ────────────────────────────────────────────────

    /// Serialize this frame as RESP2 into a byte vector.
    pub fn serialize(&self) -> Vec<u8> {
        let mut buf = Vec::new();
        self.write_to_buf(&mut buf, false);
        buf
    }

    /// Serialize this frame into a byte vector, using RESP3 encoding if `resp3` is true.
    pub fn serialize_resp(&self, resp3: bool) -> Vec<u8> {
        let mut buf = Vec::new();
        self.write_to_buf(&mut buf, resp3);
        buf
    }

    /// Write this frame into a byte buffer.
    /// When `resp3` is true, RESP3-specific types use their native wire format.
    /// When false, they degrade to RESP2 equivalents.
    pub fn write_to_buf(&self, buf: &mut Vec<u8>, resp3: bool) {
        match self {
            Frame::Simple(s) => {
                buf.push(b'+');
                buf.extend_from_slice(s.as_bytes());
                buf.extend_from_slice(b"\r\n");
            }
            Frame::Error(s) => {
                buf.push(b'-');
                buf.extend_from_slice(s.as_bytes());
                buf.extend_from_slice(b"\r\n");
            }
            Frame::Integer(n) => {
                buf.push(b':');
                buf.extend_from_slice(n.to_string().as_bytes());
                buf.extend_from_slice(b"\r\n");
            }
            Frame::Bulk(data) => {
                buf.push(b'$');
                buf.extend_from_slice(data.len().to_string().as_bytes());
                buf.extend_from_slice(b"\r\n");
                buf.extend_from_slice(data);
                buf.extend_from_slice(b"\r\n");
            }
            Frame::Null => {
                if resp3 {
                    buf.extend_from_slice(b"_\r\n");
                } else {
                    buf.extend_from_slice(b"$-1\r\n");
                }
            }
            Frame::Array(frames) => {
                buf.push(b'*');
                buf.extend_from_slice(frames.len().to_string().as_bytes());
                buf.extend_from_slice(b"\r\n");
                for frame in frames {
                    frame.write_to_buf(buf, resp3);
                }
            }
            Frame::Map(pairs) => {
                if resp3 {
                    buf.push(b'%');
                    buf.extend_from_slice(pairs.len().to_string().as_bytes());
                    buf.extend_from_slice(b"\r\n");
                } else {
                    // RESP2 fallback: flat array with 2*N elements
                    buf.push(b'*');
                    buf.extend_from_slice((pairs.len() * 2).to_string().as_bytes());
                    buf.extend_from_slice(b"\r\n");
                }
                for (k, v) in pairs {
                    k.write_to_buf(buf, resp3);
                    v.write_to_buf(buf, resp3);
                }
            }
            Frame::Set(items) => {
                if resp3 {
                    buf.push(b'~');
                } else {
                    buf.push(b'*');
                }
                buf.extend_from_slice(items.len().to_string().as_bytes());
                buf.extend_from_slice(b"\r\n");
                for item in items {
                    item.write_to_buf(buf, resp3);
                }
            }
            Frame::Push(items) => {
                if resp3 {
                    buf.push(b'>');
                } else {
                    buf.push(b'*');
                }
                buf.extend_from_slice(items.len().to_string().as_bytes());
                buf.extend_from_slice(b"\r\n");
                for item in items {
                    item.write_to_buf(buf, resp3);
                }
            }
            Frame::Double(f) => {
                let s = format_double(*f);
                if resp3 {
                    buf.push(b',');
                    buf.extend_from_slice(s.as_bytes());
                    buf.extend_from_slice(b"\r\n");
                } else {
                    // RESP2 fallback: bulk string
                    buf.push(b'$');
                    buf.extend_from_slice(s.len().to_string().as_bytes());
                    buf.extend_from_slice(b"\r\n");
                    buf.extend_from_slice(s.as_bytes());
                    buf.extend_from_slice(b"\r\n");
                }
            }
        }
    }
}

impl fmt::Display for Frame {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Frame::Simple(s) => write!(f, "+{}", s),
            Frame::Error(s) => write!(f, "-{}", s),
            Frame::Integer(n) => write!(f, ":{}", n),
            Frame::Bulk(data) => match std::str::from_utf8(data) {
                Ok(s) => write!(f, "${}", s),
                Err(_) => write!(f, "${:?}", data),
            },
            Frame::Null => write!(f, "(nil)"),
            Frame::Array(frames) => {
                write!(f, "[")?;
                for (i, frame) in frames.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}", frame)?;
                }
                write!(f, "]")
            }
            Frame::Map(pairs) => {
                write!(f, "{{")?;
                for (i, (k, v)) in pairs.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}: {}", k, v)?;
                }
                write!(f, "}}")
            }
            Frame::Set(items) => {
                write!(f, "~[")?;
                for (i, item) in items.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}", item)?;
                }
                write!(f, "]")
            }
            Frame::Push(items) => {
                write!(f, ">[")?;
                for (i, item) in items.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}", item)?;
                }
                write!(f, "]")
            }
            Frame::Double(v) => write!(f, ",{}", format_double(*v)),
        }
    }
}

/// Format a f64 for RESP3 Double wire encoding.
fn format_double(f: f64) -> String {
    if f.is_infinite() {
        if f.is_sign_positive() {
            "inf".to_string()
        } else {
            "-inf".to_string()
        }
    } else if f.is_nan() {
        "nan".to_string()
    } else {
        // Use ryu for shortest representation
        let mut buf = ryu::Buffer::new();
        let s = buf.format(f);
        s.to_string()
    }
}

impl std::error::Error for FrameError {}

impl fmt::Display for FrameError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            FrameError::Incomplete => write!(f, "incomplete frame"),
            FrameError::Protocol(msg) => write!(f, "protocol error: {}", msg),
        }
    }
}

// ── Private helpers ──────────────────────────────────────────────────

/// Peek at and consume a single byte.
fn get_u8(src: &mut Cursor<&[u8]>) -> Result<u8, FrameError> {
    if !src.has_remaining() {
        return Err(FrameError::Incomplete);
    }
    Ok(src.get_u8())
}

/// Skip `n` bytes in the cursor.
fn skip(src: &mut Cursor<&[u8]>, n: usize) -> Result<(), FrameError> {
    if src.remaining() < n {
        return Err(FrameError::Incomplete);
    }
    src.advance(n);
    Ok(())
}

/// Read until `\r\n`, return the bytes before the delimiter.
/// Advances the cursor past the `\r\n`.
fn get_line<'a>(src: &mut Cursor<&'a [u8]>) -> Result<&'a [u8], FrameError> {
    let start = src.position() as usize;
    let end = src.get_ref().len();

    for i in start..end.saturating_sub(1) {
        if src.get_ref()[i] == b'\r' && src.get_ref()[i + 1] == b'\n' {
            let line = &src.get_ref()[start..i];
            src.set_position((i + 2) as u64);
            return Ok(line);
        }
    }

    Err(FrameError::Incomplete)
}

/// Skip until after `\r\n`.
fn skip_line(src: &mut Cursor<&[u8]>) -> Result<(), FrameError> {
    get_line(src)?;
    Ok(())
}

/// Read a line and parse it as an i64.
fn get_line_as_int(src: &mut Cursor<&[u8]>) -> Result<i64, FrameError> {
    let line = get_line(src)?;
    let s = std::str::from_utf8(line).map_err(|e| FrameError::Protocol(e.to_string()))?;
    s.parse::<i64>()
        .map_err(|e| FrameError::Protocol(e.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_simple_string() {
        let data = b"+OK\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Simple("OK".into()));
    }

    #[test]
    fn parse_error() {
        let data = b"-ERR unknown command\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Error("ERR unknown command".into()));
    }

    #[test]
    fn parse_integer() {
        let data = b":42\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Integer(42));
    }

    #[test]
    fn parse_negative_integer() {
        let data = b":-1\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Integer(-1));
    }

    #[test]
    fn parse_bulk_string() {
        let data = b"$5\r\nhello\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Bulk(Bytes::from("hello")));
    }

    #[test]
    fn parse_empty_bulk_string() {
        let data = b"$0\r\n\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Bulk(Bytes::from("")));
    }

    #[test]
    fn parse_null() {
        let data = b"$-1\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Null);
    }

    #[test]
    fn parse_array() {
        let data = b"*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(
            frame,
            Frame::Array(vec![
                Frame::Bulk(Bytes::from("foo")),
                Frame::Bulk(Bytes::from("bar")),
            ])
        );
    }

    #[test]
    fn parse_empty_array() {
        let data = b"*0\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Array(vec![]));
    }

    #[test]
    fn parse_nested_array() {
        let data = b"*2\r\n*2\r\n:1\r\n:2\r\n*1\r\n+OK\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(
            frame,
            Frame::Array(vec![
                Frame::Array(vec![Frame::Integer(1), Frame::Integer(2)]),
                Frame::Array(vec![Frame::Simple("OK".into())]),
            ])
        );
    }

    #[test]
    fn check_incomplete() {
        let data = b"$5\r\nhel";
        let mut cursor = Cursor::new(&data[..]);
        assert!(matches!(
            Frame::check(&mut cursor),
            Err(FrameError::Incomplete)
        ));
    }

    #[test]
    fn check_complete() {
        let data = b"+OK\r\n";
        let mut cursor = Cursor::new(&data[..]);
        assert!(Frame::check(&mut cursor).is_ok());
    }

    #[test]
    fn round_trip_simple() {
        let frame = Frame::Simple("OK".into());
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_error() {
        let frame = Frame::Error("ERR something went wrong".into());
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_integer() {
        let frame = Frame::Integer(-999);
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_bulk() {
        let frame = Frame::Bulk(Bytes::from("hello world"));
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_null() {
        let frame = Frame::Null;
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_array() {
        let frame = Frame::Array(vec![
            Frame::Bulk(Bytes::from("SET")),
            Frame::Bulk(Bytes::from("key")),
            Frame::Bulk(Bytes::from("value")),
        ]);
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn round_trip_nested_array() {
        let frame = Frame::Array(vec![
            Frame::Integer(1),
            Frame::Array(vec![Frame::Simple("inner".into()), Frame::Null]),
            Frame::Bulk(Bytes::from("end")),
        ]);
        let bytes = frame.serialize();
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn helper_ok() {
        assert_eq!(Frame::ok(), Frame::Simple("OK".into()));
    }

    #[test]
    fn helper_strings() {
        let frame = Frame::strings(&["a", "b", "c"]);
        assert_eq!(
            frame,
            Frame::Array(vec![
                Frame::Bulk(Bytes::from("a")),
                Frame::Bulk(Bytes::from("b")),
                Frame::Bulk(Bytes::from("c")),
            ])
        );
    }

    #[test]
    fn serialize_null() {
        assert_eq!(Frame::Null.serialize(), b"$-1\r\n");
    }

    #[test]
    fn parse_protocol_error() {
        let data = b"!invalid\r\n";
        let mut cursor = Cursor::new(&data[..]);
        assert!(matches!(
            Frame::parse(&mut cursor),
            Err(FrameError::Protocol(_))
        ));
    }

    #[test]
    fn parse_multiple_frames_sequentially() {
        let data = b"+OK\r\n:42\r\n$3\r\nfoo\r\n";
        let mut cursor = Cursor::new(&data[..]);

        let f1 = Frame::parse(&mut cursor).unwrap();
        assert_eq!(f1, Frame::Simple("OK".into()));

        let f2 = Frame::parse(&mut cursor).unwrap();
        assert_eq!(f2, Frame::Integer(42));

        let f3 = Frame::parse(&mut cursor).unwrap();
        assert_eq!(f3, Frame::Bulk(Bytes::from("foo")));
    }

    #[test]
    fn parse_binary_bulk_string() {
        let mut data = Vec::new();
        data.extend_from_slice(b"$6\r\n");
        data.extend_from_slice(b"he\r\nlo");
        data.extend_from_slice(b"\r\n");

        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Bulk(Bytes::from(&b"he\r\nlo"[..])));
    }

    // ── RESP3 tests ─────────────────────────────────────────────────

    #[test]
    fn resp3_null_serialization() {
        // RESP2 null
        assert_eq!(Frame::Null.serialize(), b"$-1\r\n");
        // RESP3 null
        assert_eq!(Frame::Null.serialize_resp(true), b"_\r\n");
    }

    #[test]
    fn resp3_parse_null() {
        let data = b"_\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Null);
    }

    #[test]
    fn resp3_double_serialization() {
        // RESP3 double
        let bytes = Frame::Double(1.23).serialize_resp(true);
        assert_eq!(bytes, b",1.23\r\n");

        // RESP2 fallback: bulk string
        let bytes = Frame::Double(1.23).serialize_resp(false);
        assert_eq!(bytes, b"$4\r\n1.23\r\n");
    }

    #[test]
    fn resp3_double_inf() {
        let bytes = Frame::Double(f64::INFINITY).serialize_resp(true);
        assert_eq!(bytes, b",inf\r\n");

        let bytes = Frame::Double(f64::NEG_INFINITY).serialize_resp(true);
        assert_eq!(bytes, b",-inf\r\n");
    }

    #[test]
    fn resp3_parse_double() {
        let data = b",1.23\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Double(1.23));
    }

    #[test]
    fn resp3_parse_double_inf() {
        let data = b",inf\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Double(f64::INFINITY));

        let data = b",-inf\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, Frame::Double(f64::NEG_INFINITY));
    }

    #[test]
    fn resp3_map_serialization() {
        let frame = Frame::Map(vec![
            (Frame::bulk_string("key1"), Frame::bulk_string("val1")),
            (Frame::bulk_string("key2"), Frame::Integer(42)),
        ]);

        // RESP3: %2\r\n...
        let bytes = frame.serialize_resp(true);
        let expected = b"%2\r\n$4\r\nkey1\r\n$4\r\nval1\r\n$4\r\nkey2\r\n:42\r\n";
        assert_eq!(bytes, expected);

        // RESP2 fallback: *4\r\n... (flat array)
        let bytes = frame.serialize_resp(false);
        let expected = b"*4\r\n$4\r\nkey1\r\n$4\r\nval1\r\n$4\r\nkey2\r\n:42\r\n";
        assert_eq!(bytes, expected);
    }

    #[test]
    fn resp3_parse_map() {
        let data = b"%2\r\n$4\r\nkey1\r\n$4\r\nval1\r\n$4\r\nkey2\r\n:42\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(
            frame,
            Frame::Map(vec![
                (
                    Frame::Bulk(Bytes::from("key1")),
                    Frame::Bulk(Bytes::from("val1"))
                ),
                (Frame::Bulk(Bytes::from("key2")), Frame::Integer(42)),
            ])
        );
    }

    #[test]
    fn resp3_set_serialization() {
        let frame = Frame::Set(vec![Frame::bulk_string("a"), Frame::bulk_string("b")]);

        // RESP3: ~2\r\n...
        let bytes = frame.serialize_resp(true);
        assert_eq!(bytes, b"~2\r\n$1\r\na\r\n$1\r\nb\r\n");

        // RESP2 fallback: *2\r\n...
        let bytes = frame.serialize_resp(false);
        assert_eq!(bytes, b"*2\r\n$1\r\na\r\n$1\r\nb\r\n");
    }

    #[test]
    fn resp3_parse_set() {
        let data = b"~2\r\n$1\r\na\r\n$1\r\nb\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(
            frame,
            Frame::Set(vec![
                Frame::Bulk(Bytes::from("a")),
                Frame::Bulk(Bytes::from("b")),
            ])
        );
    }

    #[test]
    fn resp3_push_serialization() {
        let frame = Frame::Push(vec![
            Frame::bulk_string("message"),
            Frame::bulk_string("chan"),
            Frame::bulk_string("data"),
        ]);

        // RESP3: >3\r\n...
        let bytes = frame.serialize_resp(true);
        assert_eq!(
            bytes,
            b">3\r\n$7\r\nmessage\r\n$4\r\nchan\r\n$4\r\ndata\r\n"
        );

        // RESP2 fallback: *3\r\n...
        let bytes = frame.serialize_resp(false);
        assert_eq!(
            bytes,
            b"*3\r\n$7\r\nmessage\r\n$4\r\nchan\r\n$4\r\ndata\r\n"
        );
    }

    #[test]
    fn resp3_parse_push() {
        let data = b">3\r\n$7\r\nmessage\r\n$4\r\nchan\r\n$4\r\ndata\r\n";
        let mut cursor = Cursor::new(&data[..]);
        let frame = Frame::parse(&mut cursor).unwrap();
        assert_eq!(
            frame,
            Frame::Push(vec![
                Frame::Bulk(Bytes::from("message")),
                Frame::Bulk(Bytes::from("chan")),
                Frame::Bulk(Bytes::from("data")),
            ])
        );
    }

    #[test]
    fn resp3_round_trip_map() {
        let frame = Frame::Map(vec![
            (Frame::bulk_string("a"), Frame::Integer(1)),
            (Frame::bulk_string("b"), Frame::Integer(2)),
        ]);
        let bytes = frame.serialize_resp(true);
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }

    #[test]
    fn resp3_round_trip_double() {
        let frame = Frame::Double(42.5);
        let bytes = frame.serialize_resp(true);
        let mut cursor = Cursor::new(&bytes[..]);
        let parsed = Frame::parse(&mut cursor).unwrap();
        assert_eq!(frame, parsed);
    }
}
