#![allow(dead_code)]

use crate::api;

use bytes::{BufMut, Bytes, BytesMut};

/// A buffer for encoding trace events.
pub struct EventBuffer {
    scratch: [u8; 10],
    buf: BytesMut,
}

impl AsRef<[u8]> for EventBuffer {
    fn as_ref(&self) -> &[u8] {
        &self.buf
    }
}

impl EventBuffer {
    pub fn with_capacity(size: usize) -> Self {
        EventBuffer {
            scratch: [0; 10],
            buf: BytesMut::with_capacity(size),
        }
    }

    pub(super) fn freeze(self) -> Bytes {
        self.buf.freeze()
    }

    /// Writes a single byte.
    #[inline]
    pub fn byte(&mut self, byte: u8) {
        self.buf.reserve(1);
        self.buf.put_u8(byte);
    }

    /// Writes a known number of bytes.
    #[inline]
    pub fn bytes<const N: usize>(&mut self, bytes: &[u8; N]) {
        self.buf.reserve(N);
        self.buf.put_slice(bytes);
    }

    /// Ensures the buffer has enough capacity for `additional` bytes.
    /// Used to avoid additional allocations.
    #[inline]
    pub fn reserve(&mut self, additional: usize) {
        self.buf.reserve(additional);
    }

    /// Writes a variable-length string.
    #[inline]
    pub fn str<S: AsRef<str>>(&mut self, str: S) {
        self.byte_string(str.as_ref().as_bytes());
    }

    /// Writes a variable-length byte string.
    #[inline]
    pub fn byte_string(&mut self, bytes: &[u8]) {
        // 10 bytes is the maximum length of a uvarint.
        self.buf.reserve(10 + bytes.len());

        self.uvarint(bytes.len() as u64);
        self.buf.extend_from_slice(bytes);
    }

    /// Writes a variable-length truncated byte string.
    /// If truncation is necessary, the `truncation_suffix` is appended to the end of the string,
    /// leading to the final length being `max_len + truncation_suffix.len()`.
    #[inline]
    pub fn truncated_byte_string(
        &mut self,
        bytes: &[u8],
        max_len: usize,
        truncation_suffix: &[u8],
    ) {
        if bytes.len() <= max_len {
            self.byte_string(bytes);
        } else {
            let combined_len = max_len + truncation_suffix.len();
            self.uvarint(combined_len as u64);
            self.buf.reserve(combined_len);
            self.buf.put_slice(&bytes[..max_len]);
            self.buf.put_slice(truncation_suffix);
        }
    }

    /// Writes a single boolean bit.
    #[inline]
    pub fn bool(&mut self, b: bool) {
        self.byte(if b { 1 } else { 0 });
    }

    /// Writes the current system time.
    #[inline]
    pub fn system_time_now(&mut self) {
        self.system_time(std::time::SystemTime::now());
    }

    /// Writes a system time.
    #[inline]
    pub fn system_time(&mut self, time: std::time::SystemTime) {
        let duration = time
            .duration_since(std::time::UNIX_EPOCH)
            .expect("time is before UNIX_EPOCH");
        self.buf.reserve(8 + 4);
        self.i64(duration.as_secs() as i64);
        self.i32(duration.subsec_nanos() as i32);
    }

    /// Writes an UTC timestamp.
    #[inline]
    pub fn time(&mut self, time: &chrono::DateTime<chrono::Utc>) {
        self.buf.reserve(8 + 4);
        self.i64(time.timestamp());
        self.i32(time.timestamp_subsec_nanos() as i32);
    }

    /// Writes a variable-length signed integer.
    #[inline]
    pub fn ivarint<I: Into<i64>>(&mut self, i: I) {
        self.uvarint(signed_to_unsigned_i64(i.into()));
    }

    /// Writes a variable-length unsigned integer.
    #[inline]
    pub fn uvarint<U: Into<u64>>(&mut self, u: U) {
        let mut u: u64 = u.into();
        let mut i = 0;
        while u >= 0x80 {
            self.scratch[i] = (u as u8) | 0x80;
            u >>= 7;
            i += 1;
        }
        self.scratch[i] = u as u8;
        i += 1;
        self.buf.extend_from_slice(&self.scratch[..i]);
    }

    /// Writes a float, always as 8 bytes.
    #[inline]
    pub fn f64(&mut self, f: f64) {
        let data: [u8; 8] = f.to_le_bytes();
        self.buf.extend_from_slice(&data);
    }

    /// Writes a signed integer, always as 8 bytes.
    #[inline]
    pub fn i64(&mut self, i: i64) {
        self.u64(signed_to_unsigned_i64(i));
    }

    /// Writes an unsigned integer, always as 8 bytes.
    #[inline]
    pub fn u64(&mut self, u: u64) {
        let data: [u8; 8] = u.to_le_bytes();
        self.buf.extend_from_slice(&data);
    }

    /// Writes a signed integer, always as 4 bytes.
    #[inline]
    pub fn i32(&mut self, i: i32) {
        self.u32(signed_to_unsigned_i32(i));
    }

    /// Writes an unsigned integer, always as 4 bytes.
    #[inline]
    pub fn u32(&mut self, u: u32) {
        let data: [u8; 4] = u.to_le_bytes();
        self.buf.extend_from_slice(&data);
    }

    /// Writes a duration.
    #[inline]
    pub fn duration(&mut self, duration: std::time::Duration) {
        // The current trace protocol only supports durations that fit in an i64.
        // If the duration exceeds that, truncate it to the maximum value.
        //
        // Note: Rust's duration type is for positive durations only, so we only consider
        // the positive range of i64 here.
        let nanos = duration.as_nanos();
        let nanos: i64 = if nanos > std::i64::MAX as u128 {
            std::i64::MAX
        } else {
            nanos as i64
        };
        self.ivarint(nanos);
    }

    #[inline]
    pub fn api_err_with_legacy_stack(&mut self, err: Option<&api::Error>) {
        match err {
            Some(err) => {
                let err = serde_json::to_string_pretty(err).unwrap_or("unknown error".to_string());
                self.str(err);
                self.nyi_stack_pcs()
            }
            None => self.str(""),
        }
    }

    #[inline]
    pub fn err_with_legacy_stack<E: std::fmt::Display>(&mut self, err: Option<&E>) {
        match err {
            Some(err) => {
                let err = err.to_string();
                self.str(&err);
                self.nyi_stack_pcs()
            }
            None => self.str(""),
        }
    }

    /// Adds the stack pcs to the buffer.
    /// It's not supported for the new runtime yet, so it just writes 0 frames.
    #[inline]
    pub fn nyi_stack_pcs(&mut self) {
        self.byte(0);
    }

    #[inline]
    pub fn nyi_formatted_stack(&mut self) {
        self.byte(0);
    }
}

#[inline]
pub(super) fn signed_to_unsigned_i64(i: i64) -> u64 {
    if i < 0 {
        ((!(i as u64)) << 1) | 1 // complement i, bit 0 is 1
    } else {
        (i as u64) << 1 // do not complement i, bit 0 is 0
    }
}

#[inline]
pub(super) fn signed_to_unsigned_i32(i: i32) -> u32 {
    if i < 0 {
        ((!(i as u32)) << 1) | 1 // complement i, bit 0 is 1
    } else {
        (i as u32) << 1 // do not complement i, bit 0 is 0
    }
}
