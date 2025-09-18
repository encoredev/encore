//! Conversions to and from Postgres's binary format for the numeric type.
use byteorder::{BigEndian, ReadBytesExt};
use bytes::{BufMut, BytesMut};
use std::boxed::Box as StdBox;
use std::error::Error;
use std::str::{self, FromStr};

/// Serializes a `NUMERIC` value.
#[inline]
pub fn numeric_to_sql(v: Numeric, buf: &mut BytesMut) {
    let num_digits = v.digits.len() as u16;
    buf.put_u16(num_digits);
    buf.put_i16(v.weight);
    buf.put_u16(v.sign.into_u16());
    buf.put_u16(v.scale);

    for digit in v.digits {
        buf.put_i16(digit);
    }
}

/// Deserializes a `NUMERIC` value.
#[inline]
pub fn numeric_from_sql(mut buf: &[u8]) -> Result<Numeric, StdBox<dyn Error + Sync + Send>> {
    let num_digits = buf.read_u16::<BigEndian>()?;
    let mut digits = Vec::with_capacity(num_digits.into());

    let weight = buf.read_i16::<BigEndian>()?;
    let sign = NumericSign::try_from_u16(buf.read_u16::<BigEndian>()?)?;

    let scale = buf.read_u16::<BigEndian>()?;

    for _ in 0..num_digits {
        digits.push(buf.read_i16::<BigEndian>()?);
    }

    Ok(Numeric {
        sign,
        scale,
        weight,
        digits,
    })
}

/// Numeric sign
#[derive(Debug, Copy, Clone, PartialEq, Eq)]
pub enum NumericSign {
    /// Positive number
    Positive,
    /// Negative number
    Negative,
    /// Not a number
    NaN,
    /// Positive infinity
    PositiveInfinity,
    /// Negative infinity
    NegativeInfinity,
}

impl NumericSign {
    #[inline]
    fn try_from_u16(sign: u16) -> Result<NumericSign, StdBox<dyn Error + Sync + Send>> {
        match sign {
            0x0000 => Ok(NumericSign::Positive),
            0x4000 => Ok(NumericSign::Negative),
            0xC000 => Ok(NumericSign::NaN),
            0xD000 => Ok(NumericSign::PositiveInfinity),
            0xF000 => Ok(NumericSign::NegativeInfinity),
            _ => Err("invalid sign in numeric value".into()),
        }
    }

    #[inline]
    fn into_u16(self) -> u16 {
        match self {
            NumericSign::Positive => 0x0000,
            NumericSign::Negative => 0x4000,
            NumericSign::NaN => 0xC000,
            NumericSign::PositiveInfinity => 0xD000,
            NumericSign::NegativeInfinity => 0xF000,
        }
    }
}

/// A Posgres numeric
#[derive(Debug, PartialEq, Eq)]
pub struct Numeric {
    sign: NumericSign,
    scale: u16,
    weight: i16,
    digits: Vec<i16>,
}

impl Numeric {
    /// Returns the number of digits.
    #[inline]
    pub fn num_digits(&self) -> usize {
        self.digits.len()
    }

    /// Returns the weight of the numeric value.
    #[inline]
    pub fn weight(&self) -> i16 {
        self.weight
    }

    /// Returns the scale of the numeric value.
    #[inline]
    pub fn scale(&self) -> u16 {
        self.scale
    }

    /// Returns the sign of the numeric value.
    #[inline]
    pub fn sign(&self) -> NumericSign {
        self.sign
    }

    fn nan() -> Self {
        Self {
            sign: NumericSign::NaN,
            scale: 0,
            weight: 0,
            digits: vec![],
        }
    }

    fn infinity() -> Self {
        Self {
            sign: NumericSign::PositiveInfinity,
            scale: 0,
            weight: 0,
            digits: vec![],
        }
    }

    fn negative_infinity() -> Self {
        Self {
            sign: NumericSign::NegativeInfinity,
            scale: 0,
            weight: 0,
            digits: vec![],
        }
    }
}

impl std::fmt::Display for Numeric {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self.sign {
            NumericSign::NaN => write!(f, "NaN"),
            NumericSign::PositiveInfinity => write!(f, "Infinity"),
            NumericSign::NegativeInfinity => write!(f, "-Infinity"),
            NumericSign::Negative => {
                write!(f, "-")?;
                format_positive_number(self, f)
            }
            NumericSign::Positive => format_positive_number(self, f),
        }
    }
}

impl FromStr for Numeric {
    type Err = ParseError;

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        if let Some(special) = parse_special_values(value)? {
            return Ok(special);
        }

        let components = parse_number_components(value)?;

        if components.exponent == 0 {
            parse_simple_number(&components)
        } else {
            parse_scientific_number(&components)
        }
    }
}

/// Parse a simple number without scientific notation (fast path)
fn parse_simple_number(components: &ParsedComponents<'_>) -> Result<Numeric, ParseError> {
    let mut digits = Vec::new();
    let mut weight;
    let scale = components.decimal_part.len() as u16;

    let integer_digits = build_integer_digits_from_bytes(components.integer_part)?;
    let has_integer = !integer_digits.is_empty();

    if has_integer {
        // Calculate weight from non-zero integer digits
        let non_zero_start = components
            .integer_part
            .iter()
            .position(|&b| b != b'0')
            .unwrap_or(0);
        let significant_len = components.integer_part.len() - non_zero_start;
        weight = ((significant_len - 1) / 4) as i16;
        digits.extend(integer_digits);
    } else {
        weight = if !components.decimal_part.is_empty() {
            -1
        } else {
            0
        };
    }

    if !components.decimal_part.is_empty() {
        if has_integer {
            let decimal_digits = build_decimal_digits_from_bytes(components.decimal_part)?;
            digits.extend(decimal_digits);
        } else {
            process_decimal_digits(components.decimal_part, &mut digits, &mut weight)?;
        }
    }

    // Normalize zeros
    normalize_leading_zeros(&mut digits, &mut weight);
    normalize_trailing_zeros(&mut digits);

    Ok(Numeric {
        sign: components.sign,
        scale,
        weight,
        digits,
    })
}

/// Parse a number with scientific notation
fn parse_scientific_number(components: &ParsedComponents<'_>) -> Result<Numeric, ParseError> {
    let layout = ScientificLayout::new(
        components.integer_part,
        components.decimal_part,
        components.exponent,
    );

    // Get digit vectors from the scientific layout
    let effective_integer = layout.integer_digits();
    let effective_decimal = layout.decimal_digits();

    let mut digits = Vec::new();
    let mut weight;

    // Calculate scale based on decimal digits from the layout
    let effective_scale = effective_decimal.len() as u16;

    let scale = components.calculate_scale(effective_scale);

    // Process integer and decimal parts
    let integer_digits = build_integer_digits_from_bytes(&effective_integer)?;
    let has_integer = !integer_digits.is_empty();

    if has_integer {
        // Calculate weight from non-zero integer digits
        let non_zero_start = effective_integer
            .iter()
            .position(|&b| b != b'0')
            .unwrap_or(0);
        let significant_len = effective_integer.len() - non_zero_start;
        weight = ((significant_len - 1) / 4) as i16;
        digits.extend(integer_digits);
    } else {
        weight = if !effective_decimal.is_empty() { -1 } else { 0 };
    }

    // Process decimal part
    if !effective_decimal.is_empty() {
        if has_integer {
            // Mixed number - just add decimal digits
            let decimal_digits = build_decimal_digits_from_bytes(&effective_decimal)?;
            digits.extend(decimal_digits);
        } else {
            process_decimal_digits(&effective_decimal, &mut digits, &mut weight)?;
        }
    }

    // Normalize zeros
    normalize_leading_zeros(&mut digits, &mut weight);
    normalize_trailing_zeros(&mut digits);

    Ok(Numeric {
        sign: components.sign,
        scale,
        weight,
        digits,
    })
}

fn format_positive_number(n: &Numeric, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    format_integer_part(n, f)?;
    format_decimal_part(n, f)?;
    Ok(())
}

fn format_integer_part(n: &Numeric, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    if n.weight() < 0 {
        write!(f, "0")?;
        return Ok(());
    }

    for i in 0..=n.weight() {
        let d = n.digits.get(i as usize).unwrap_or(&0);
        if i == 0 {
            write!(f, "{d}")?;
        } else {
            write!(f, "{d:04}")?;
        }
    }

    Ok(())
}

fn format_decimal_part(n: &Numeric, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    if n.scale() == 0 {
        return Ok(());
    }

    write!(f, ".")?;

    let mut remaining_decimals = n.scale();

    let mut curr_weight = -1;
    while curr_weight > n.weight() {
        write!(f, "0000")?;
        curr_weight -= 1;
        remaining_decimals = remaining_decimals.saturating_sub(4);
    }

    if remaining_decimals == 0 {
        return Ok(());
    }

    let decimal_idx = if n.weight() < 0 { 0 } else { n.weight() + 1 } as usize;
    for i in decimal_idx..n.digits.len() {
        let d = n.digits.get(i).unwrap_or(&0);
        if remaining_decimals >= 4 {
            write!(f, "{d:04}")?;
            remaining_decimals -= 4;
        } else {
            let truncated = d / 10_i16.pow(4 - remaining_decimals as u32);
            write!(
                f,
                "{:0width$}",
                truncated,
                width = remaining_decimals as usize
            )?;

            remaining_decimals = 0;
            break;
        }
    }

    if remaining_decimals > 0 {
        write!(f, "{:0width$}", 0, width = remaining_decimals as usize)?;
    }

    Ok(())
}

type ParseError = StdBox<dyn Error + Sync + Send>;

/// Components of a parsed numeric string
struct ParsedComponents<'a> {
    sign: NumericSign,
    integer_part: &'a [u8],
    decimal_part: &'a [u8],
    exponent: i32,
}

impl<'a> ParsedComponents<'a> {
    /// Calculate the scale for a numeric value, handling scientific notation
    fn calculate_scale(&self, effective_scale: u16) -> u16 {
        // Check if the original number is zero
        let is_zero = self.integer_part.iter().all(|&b| b == b'0')
            && (self.decimal_part.is_empty() || self.decimal_part.iter().all(|&b| b == b'0'));
        if is_zero {
            if self.exponent < 0 {
                // For "0e-10", preserve the scale implied by the exponent
                (-self.exponent as u16).max(self.decimal_part.len() as u16)
            } else {
                // For "0.0e5", preserve the original decimal scale
                self.decimal_part.len() as u16
            }
        } else if self.exponent < 0 {
            // For negative exponents, calculate implied decimal places
            let implied = self.decimal_part.len() as i32 + (-self.exponent);
            implied.max(0).min(u16::MAX as i32) as u16
        } else {
            // For positive exponents, use the effective scale
            effective_scale
        }
    }
}

fn parse_special_values(s: &str) -> Result<Option<Numeric>, ParseError> {
    if s.eq_ignore_ascii_case("NaN") {
        return Ok(Some(Numeric::nan()));
    }
    if s.eq_ignore_ascii_case("Infinity") || s.eq_ignore_ascii_case("Inf") {
        return Ok(Some(Numeric::infinity()));
    }
    if s.eq_ignore_ascii_case("-Infinity") || s.eq_ignore_ascii_case("-Inf") {
        return Ok(Some(Numeric::negative_infinity()));
    }
    Ok(None)
}

fn parse_scientific_component(s: &[u8]) -> Result<(&[u8], Option<i32>), ParseError> {
    let (s, e) = split_e(s);

    let exp = if let Some(mut e) = e {
        if e.is_empty() {
            return Err("empty scientific notation string".into());
        }

        let mut positive = true;
        let mut exp: i32 = 0;

        if let Some(&b'-') = e.first() {
            positive = false;
            e = &e[1..];
        } else if let Some(&b'+') = e.first() {
            e = &e[1..];
        }

        if e.is_empty() {
            return Err("empty scientific notation exponent".into());
        }

        for &b in e {
            if !b.is_ascii_digit() {
                return Err("scientific notation string contain non-digit character".into());
            }
            // Prevent integer overflow in exponent
            exp = exp.saturating_mul(10).saturating_add((b - b'0') as i32);
            if exp > 1000000 {
                // Reasonable limit for exponents
                return Err("scientific notation exponent too large".into());
            }
        }

        Some(if positive { exp } else { -exp })
    } else {
        None
    };

    Ok((s, exp))
}

fn parse_number_components(s: &str) -> Result<ParsedComponents<'_>, ParseError> {
    let mut sign = NumericSign::Positive;

    let mut s = s.as_bytes();
    if let Some(&b'-') = s.first() {
        sign = NumericSign::Negative;
        s = &s[1..];
    };

    // Check for completely empty input after removing sign
    if s.is_empty() {
        return Err("empty numeric string".into());
    }

    let (s, exp) = parse_scientific_component(s)?;
    let (integer_part, decimal_part) = split_decimal(s);
    let decimal_part = decimal_part.unwrap_or_default();

    if integer_part.is_empty() && decimal_part.is_empty() {
        return Err("invalid numeric string".into());
    }

    Ok(ParsedComponents {
        sign,
        integer_part,
        decimal_part,
        exponent: exp.unwrap_or(0),
    })
}

/// Represents the layout of digits after applying scientific notation transformation,
/// calculating positions and zero-padding.
#[derive(Debug)]
struct ScientificLayout<'a> {
    integer_part: &'a [u8],
    decimal_part: &'a [u8],
    decimal_point_pos: i32,
    leading_zeros: usize,
    trailing_zeros: usize,
}

impl<'a> ScientificLayout<'a> {
    fn new(integer: &'a [u8], decimal: &'a [u8], exponent: i32) -> Self {
        let decimal_point_pos = integer.len() as i32 + exponent;

        let total_original_digits = integer.len() + decimal.len();

        let (leading_zeros, trailing_zeros) = if exponent > 0 {
            let total_needed = decimal_point_pos as usize;
            let trailing_zeros = total_needed.saturating_sub(total_original_digits);
            (0, trailing_zeros)
        } else if decimal_point_pos <= 0 {
            let leading_zeros = (-decimal_point_pos) as usize;
            (leading_zeros, 0)
        } else {
            (0, 0)
        };

        Self {
            integer_part: integer,
            decimal_part: decimal,
            decimal_point_pos,
            leading_zeros,
            trailing_zeros,
        }
    }

    /// Returns the effective integer digits
    fn integer_digits(&self) -> Vec<u8> {
        if self.decimal_point_pos <= 0 {
            return vec![];
        }

        let split_pos = (self.decimal_point_pos as usize)
            .min(self.integer_part.len() + self.decimal_part.len());

        let total_len = split_pos + self.trailing_zeros;
        let mut result = Vec::with_capacity(total_len);

        result.extend_from_slice(self.integer_part);

        // Add from decimal part if needed
        if split_pos > self.integer_part.len() {
            let decimal_count = split_pos - self.integer_part.len();
            result.extend(self.decimal_part.iter().take(decimal_count));
        }

        // Add trailing zeros
        result.resize(total_len, b'0');

        result
    }

    /// Returns the effective decimal digits
    fn decimal_digits(&self) -> Vec<u8> {
        if self.decimal_point_pos <= 0 {
            // All digits become decimal with leading zeros
            let total_len = self.leading_zeros + self.integer_part.len() + self.decimal_part.len();
            let mut result = Vec::with_capacity(total_len);
            result.resize(self.leading_zeros, b'0');
            result.extend_from_slice(self.integer_part);
            result.extend_from_slice(self.decimal_part);
            result
        } else {
            let split_pos = (self.decimal_point_pos as usize)
                .min(self.integer_part.len() + self.decimal_part.len());

            if split_pos >= self.integer_part.len() + self.decimal_part.len() {
                // No decimal digits
                Vec::new()
            } else if split_pos >= self.integer_part.len() {
                // Split within decimal part
                let skip_decimal = split_pos - self.integer_part.len();
                self.decimal_part[skip_decimal..].to_vec()
            } else {
                // Split within integer part
                let mut result = Vec::new();
                result.extend_from_slice(&self.integer_part[split_pos..]);
                result.extend_from_slice(self.decimal_part);
                result
            }
        }
    }
}

/// Process decimal-only numbers, setting weight based on leading zeros
fn process_decimal_digits(
    decimal_bytes: &[u8],
    digits: &mut Vec<i16>,
    weight: &mut i16,
) -> Result<(), ParseError> {
    let leading_zeros = decimal_bytes.iter().take_while(|&&b| b == b'0').count();

    if leading_zeros < decimal_bytes.len() {
        // Calculate weight based on position of first significant digit group
        *weight = -(((leading_zeros / 4) + 1) as i16);

        // Process from the start of the significant digit group
        let group_start = (leading_zeros / 4) * 4;
        let decimal_digits = build_decimal_digits_from_bytes(&decimal_bytes[group_start..])?;
        digits.extend(decimal_digits);
    } else {
        // All zeros
        *weight = 0;
    }
    Ok(())
}

/// Builds base-10000 digits from integer digit bytes.
/// Returns digits in most-significant-first order.
fn build_integer_digits_from_bytes(digits: &[u8]) -> Result<Vec<i16>, ParseError> {
    // Skip leading zeros
    let trimmed_start = digits
        .iter()
        .position(|&b| b != b'0')
        .unwrap_or(digits.len());

    if trimmed_start == digits.len() {
        return Ok(Vec::new());
    }

    let trimmed = &digits[trimmed_start..];

    // Process in chunks of 4 from the right, converting directly to base-10000
    let mut result = Vec::with_capacity(trimmed.len().div_ceil(4));

    for chunk in trimmed.rchunks(4) {
        let mut digit = 0i16;
        for &b in chunk {
            if !b.is_ascii_digit() {
                return Err("invalid digit character".into());
            }
            digit = digit * 10 + (b - b'0') as i16;
        }
        result.push(digit);
    }

    result.reverse(); // Reverse to get most-significant-first order
    Ok(result)
}

/// Builds base-10000 digits from decimal part bytes.
fn build_decimal_digits_from_bytes(digits: &[u8]) -> Result<Vec<i16>, ParseError> {
    if digits.is_empty() {
        return Ok(Vec::new());
    }

    // Process in 4-digit chunks, padding the last chunk
    let mut result = Vec::with_capacity(digits.len().div_ceil(4));

    for chunk in digits.chunks(4) {
        let mut digit = 0i16;
        for &b in chunk {
            if !b.is_ascii_digit() {
                return Err("invalid digit character".into());
            }
            digit = digit * 10 + (b - b'0') as i16;
        }

        // Pad to 4 digits for proper base-10000 representation
        if chunk.len() < 4 {
            digit *= 10i16.pow((4 - chunk.len()) as u32);
        }

        result.push(digit);
    }

    Ok(result)
}

/// Normalize leading zeros
fn normalize_leading_zeros(digits: &mut Vec<i16>, weight: &mut i16) {
    let leading_zero_count = digits.iter().take_while(|&&d| d == 0).count();

    if leading_zero_count > 0 {
        if leading_zero_count == digits.len() {
            // All digits are zero
            *weight = 0;
            digits.clear();
        } else {
            // Remove leading zeros
            digits.drain(0..leading_zero_count);
            *weight -= leading_zero_count as i16;
        }
    }
}

/// Remove trailing zeros
fn normalize_trailing_zeros(digits: &mut Vec<i16>) {
    // Find last non-zero position
    if let Some(last_nonzero) = digits.iter().rposition(|&d| d != 0) {
        digits.truncate(last_nonzero + 1);
    } else {
        digits.clear();
    }
}

fn split_e(s: &[u8]) -> (&[u8], Option<&[u8]>) {
    let mut s = s.splitn(2, |&b| b == b'e' || b == b'E');
    let first = s.next().unwrap();
    let second = s.next();
    (first, second)
}

fn split_decimal(s: &[u8]) -> (&[u8], Option<&[u8]>) {
    let mut s = s.splitn(2, |&b| b == b'.');
    let first = s.next().unwrap();
    let second = s.next();
    (first, second)
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_string_deserialization_and_serialization() {
        let cases = &[
            (
                "0",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 0,
                    digits: vec![],
                },
            ),
            (
                "1",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 0,
                    digits: vec![1],
                },
            ),
            (
                "-1",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 0,
                    digits: vec![1],
                },
            ),
            (
                "10",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 0,
                    digits: vec![10],
                },
            ),
            (
                "-10",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 0,
                    digits: vec![10],
                },
            ),
            (
                "20000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![2],
                },
            ),
            (
                "-20000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 1,
                    digits: vec![2],
                },
            ),
            (
                "20001",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![2, 1],
                },
            ),
            (
                "-20001",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 1,
                    digits: vec![2, 1],
                },
            ),
            (
                "200000000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 2,
                    digits: vec![2],
                },
            ),
            (
                "2.0",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 1,
                    weight: 0,
                    digits: vec![2],
                },
            ),
            (
                "2.1",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 1,
                    weight: 0,
                    digits: vec![2, 1000],
                },
            ),
            (
                "2.10",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 2,
                    weight: 0,
                    digits: vec![2, 1000],
                },
            ),
            (
                "200000000.0001",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 4,
                    weight: 2,
                    digits: vec![2, 0, 0, 1],
                },
            ),
            (
                "-200000000.0001",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 4,
                    weight: 2,
                    digits: vec![2, 0, 0, 1],
                },
            ),
            (
                "0.1",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 1,
                    weight: -1,
                    digits: vec![1000],
                },
            ),
            (
                "-0.1",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 1,
                    weight: -1,
                    digits: vec![1000],
                },
            ),
            (
                "123.456",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 3,
                    weight: 0,
                    digits: vec![123, 4560],
                },
            ),
            (
                "-123.456",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 3,
                    weight: 0,
                    digits: vec![123, 4560],
                },
            ),
            (
                "-123.0456",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 4,
                    weight: 0,
                    digits: vec![123, 456],
                },
            ),
            (
                "0.1000000000000000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 16,
                    weight: -1,
                    digits: vec![1000],
                },
            ),
            (
                "-0.1000000000000000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 16,
                    weight: -1,
                    digits: vec![1000],
                },
            ),
            (
                "0.003159370000000000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 18,
                    weight: -1,
                    digits: vec![31, 5937],
                },
            ),
            (
                "-0.003159370000000000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 18,
                    weight: -1,
                    digits: vec![31, 5937],
                },
            ),
            (
                "0.0000000000000002",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 16,
                    weight: -4,
                    digits: vec![2],
                },
            ),
            (
                "-0.0000000000000002",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 16,
                    weight: -4,
                    digits: vec![2],
                },
            ),
            (
                "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 35,
                    digits: vec![
                        1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                        0, 0, 0, 0, 0, 0, 0, 1,
                    ],
                },
            ),
            (
                "-100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 35,
                    digits: vec![
                        1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                        0, 0, 0, 0, 0, 0, 0, 1,
                    ],
                },
            ),
        ];

        for (str, n) in cases {
            assert_eq!(*str, n.to_string(), "numeric to string");
            let num = str.parse::<Numeric>().expect("parse numeric");
            assert_eq!(num, *n, "numeric from string");
        }
    }

    #[test]
    fn test_from_scientific_notation() {
        let cases = &[
            (
                "2e4",
                "20000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![2],
                },
            ),
            (
                "2e+4",
                "20000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![2],
                },
            ),
            (
                "-2e4",
                "-20000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 1,
                    digits: vec![2],
                },
            ),
            (
                "-2e-4",
                "-0.0002",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 4,
                    weight: -1,
                    digits: vec![2],
                },
            ),
            (
                "1.234e4",
                "12340",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![1, 2340],
                },
            ),
            (
                "-1.234e4",
                "-12340",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 1,
                    digits: vec![1, 2340],
                },
            ),
            (
                "1.234e5",
                "123400",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![12, 3400],
                },
            ),
            (
                "-1.234e5",
                "-123400",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 1,
                    digits: vec![12, 3400],
                },
            ),
            (
                "1.234e8",
                "123400000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 2,
                    digits: vec![1, 2340],
                },
            ),
            (
                "-1.234e8",
                "-123400000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 2,
                    digits: vec![1, 2340],
                },
            ),
            (
                "0.0001e4",
                "1",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 0,
                    digits: vec![1],
                },
            ),
            (
                "-0.0001e4",
                "-1",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 0,
                    digits: vec![1],
                },
            ),
            (
                "0.0001e5",
                "10",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 0,
                    digits: vec![10],
                },
            ),
            (
                "-0.0001e5",
                "-10",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 0,
                    digits: vec![10],
                },
            ),
            (
                "2e16",
                "20000000000000000",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 4,
                    digits: vec![2],
                },
            ),
            (
                "-2e16",
                "-20000000000000000",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 0,
                    weight: 4,
                    digits: vec![2],
                },
            ),
            (
                "2e-16",
                "0.0000000000000002",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 16,
                    weight: -4,
                    digits: vec![2],
                },
            ),
            (
                "-2e-16",
                "-0.0000000000000002",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 16,
                    weight: -4,
                    digits: vec![2],
                },
            ),
            (
                "2e-17",
                "0.00000000000000002",
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 17,
                    weight: -5,
                    digits: vec![2000],
                },
            ),
            (
                "-2e-17",
                "-0.00000000000000002",
                Numeric {
                    sign: NumericSign::Negative,
                    scale: 17,
                    weight: -5,
                    digits: vec![2000],
                },
            ),
        ];

        for (e, str, n) in cases {
            let num = e.parse::<Numeric>().expect("parse numeric");
            assert_eq!(num, *n, "{e} to numeric");
            assert_eq!(num.to_string(), *str, "{e} back to string");
        }
    }

    #[test]
    fn test_error_conditions() {
        // Test inputs that should definitely be errors
        let definitely_invalid = &[
            "",        // Empty string
            ".",       // Just a decimal sign
            "-",       // Just minus sign
            "+",       // Just plus sign
            "abc",     // Non-numeric characters
            "1.2.3",   // Multiple decimal points
            "1.2.3e4", // Multiple decimals with scientific
            "1e2e3",   // Multiple scientific notations
            "∞",       // Unicode infinity (not ASCII)
            "1ee2",    // Double e
            "1e",      // Scientific notation without exponent
            "1e+",     // Scientific notation with just +
            "1e-",     // Scientific notation with just -
        ];

        for &invalid in definitely_invalid {
            assert!(
                invalid.parse::<Numeric>().is_err(),
                "Should fail to parse: '{}'",
                invalid
            );
        }

        // Test that some edge cases are actually valid
        let valid_edge_cases = &[
            ("-.1", "-0.1"), // Negative with no integer part
            ("1.", "1"),     // Trailing decimal point
            ("0.", "0"),     // Zero with trailing decimal
            (".1", "0.1"),   // Decimal point prefix
            (".0", "0.0"),   // Zero with decimal point prefix
        ];

        for (input, expected) in valid_edge_cases {
            let num = input
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Should parse: '{}'", input));
            assert_eq!(
                num.to_string(),
                *expected,
                "Edge case: {} -> {}",
                input,
                expected
            );
        }
    }

    #[test]
    fn test_scientific_notation_edge_cases() {
        let cases = &[
            // Zero with various scientific notations - scale is preserved
            ("0e0", "0"),
            ("0e10", "0"),
            ("0e-10", "0.0000000000"), // Scale preserved from scientific notation
            ("0.0e5", "0.0"),
            ("0.000e100", "0.000"),
            // Very large positive exponents
            (
                "1e50",
                "100000000000000000000000000000000000000000000000000",
            ),
            ("2e20", "200000000000000000000"),
            // Very large negative exponents
            (
                "1e-50",
                "0.00000000000000000000000000000000000000000000000001",
            ),
            ("5e-25", "0.0000000000000000000000005"),
            // Leading zeros in mantissa with scientific notation
            ("000123e2", "12300"),
            ("0.000123e3", "0.123"),
            ("0.000123e6", "123"),
            // Trailing zeros
            ("1230e-2", "12.30"),
            ("1000e3", "1000000"),
        ];

        for (input, expected) in cases {
            let num = input
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Failed to parse: {}", input));
            let result = num.to_string();
            assert_eq!(
                result, *expected,
                "Scientific notation: {} -> {}",
                input, expected
            );
        }
    }

    #[test]
    fn test_normalization_edge_cases() {
        let cases = &[
            // Various patterns of leading zeros
            ("000", "0"),
            ("000.000", "0.000"),
            ("0001", "1"),
            ("0001.0010", "1.0010"), // Trailing zeros in scale are preserved
            // Trailing zeros in different positions
            ("1.000", "1.000"),
            ("1.100", "1.100"),
            ("1.010", "1.010"),
            ("10.000", "10.000"),
            // All zeros in digit groups
            ("10000", "10000"),         // This becomes weight=1, digits=[1]
            ("100000000", "100000000"), // This becomes weight=2, digits=[1]
        ];

        for (input, expected) in cases {
            let num = input
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Failed to parse: {}", input));
            let result = num.to_string();
            assert_eq!(
                result, *expected,
                "Normalization: {} -> {}",
                input, expected
            );
        }
    }

    #[test]
    fn test_weight_boundary_conditions() {
        let cases = &[
            // Test numbers that fall on base-10000 digit boundaries
            ("1", 0),         // Single digit -> weight 0
            ("9999", 0),      // Max single group -> weight 0
            ("10000", 1),     // Min double group -> weight 1
            ("99999999", 1),  // Max double group -> weight 1
            ("100000000", 2), // Min triple group -> weight 2
            ("0.1", -1),
            ("0.0001", -1),
            ("0.00001", -2),
            ("0.000000001", -3),
        ];

        for (input, expected_weight) in cases {
            let num = input
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Failed to parse: {}", input));
            assert_eq!(
                num.weight(),
                *expected_weight,
                "Weight for {}: expected {}, got {}",
                input,
                expected_weight,
                num.weight()
            );
        }
    }

    #[test]
    fn test_precision_limits() {
        // Test very high precision numbers
        let high_precision_cases = &[
            "0.123456789012345678901234567890123456789012345678901234567890",
            "123456789012345678901234567890.123456789012345678901234567890",
            "0.000000000000000000000000000000000000000000000000000000000123456789",
        ];

        for input in high_precision_cases {
            let num = input
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Failed to parse: {}", input));
            let result = num.to_string();
            // Should be able to round-trip, though might lose trailing zeros
            let reparsed = result
                .parse::<Numeric>()
                .unwrap_or_else(|_| panic!("Failed to reparse: {}", result));
            assert_eq!(num, reparsed, "Round-trip failed for: {}", input);
        }
    }

    #[test]
    fn test_display_formatting_edge_cases() {
        let cases = &[
            // Numbers that test decimal formatting boundaries
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 4,
                    weight: -1,
                    digits: vec![1],
                },
                "0.0001",
            ),
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 4,
                    weight: -2,
                    digits: vec![1],
                },
                "0.0000",
            ), // Only 4 decimal places due to scale
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 8,
                    weight: -2,
                    digits: vec![1],
                },
                "0.00000001",
            ), // 8 decimal places
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 1,
                    weight: -1,
                    digits: vec![5000],
                },
                "0.5",
            ),
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 8,
                    weight: -2,
                    digits: vec![1234],
                },
                "0.00001234",
            ),
            // Numbers that test weight group boundaries
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![1],
                },
                "10000",
            ),
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 2,
                    digits: vec![1],
                },
                "100000000",
            ),
            (
                Numeric {
                    sign: NumericSign::Positive,
                    scale: 0,
                    weight: 1,
                    digits: vec![1, 2345],
                },
                "12345",
            ),
        ];

        for (numeric, expected) in cases {
            let result = numeric.to_string();
            assert_eq!(
                result, *expected,
                "Display formatting failed for {:?}",
                numeric
            );
        }
    }

    #[test]
    fn test_various_corner_cases() {
        // Scientific notation bounds - exponent limits
        let exponent_bounds = &[
            "1e1000000",  // At exponent limit (should work)
            "1e1000001",  // Above exponent limit (should error)
            "1e-1000000", // At negative limit (should work)
            "1e-1000001", // Above negative limit (should error)
        ];

        // First and third should work, second and fourth should error
        assert!(
            exponent_bounds[0].parse::<Numeric>().is_ok(),
            "Should parse: {}",
            exponent_bounds[0]
        );
        assert!(
            exponent_bounds[1].parse::<Numeric>().is_err(),
            "Should error: {}",
            exponent_bounds[1]
        );
        assert!(
            exponent_bounds[2].parse::<Numeric>().is_ok(),
            "Should parse: {}",
            exponent_bounds[2]
        );
        assert!(
            exponent_bounds[3].parse::<Numeric>().is_err(),
            "Should error: {}",
            exponent_bounds[3]
        );

        // Scale calculation u16 boundary
        let scale_boundary = &[
            ("1e-65535", 65535_u16),     // Maximum u16 scale
            ("1.234e-65533", 65535_u16), // Should cap at u16::MAX
        ];
        for (input, expected_scale) in scale_boundary {
            let parsed = input.parse::<Numeric>().unwrap();
            assert_eq!(parsed.scale(), *expected_scale, "Scale boundary: {}", input);
        }

        // Weight boundary conditions - very deep decimals
        let weight_boundaries = &[
            ("0.0001", -1_i16),      // Basic negative weight
            ("0.00001", -2_i16),     // Weight -2
            ("0.000000001", -3_i16), // Weight -3 (9 zeros, 3rd group)
        ];
        for (input, expected_weight) in weight_boundaries {
            let parsed = input.parse::<Numeric>().unwrap();
            assert_eq!(
                parsed.weight(),
                *expected_weight,
                "Weight boundary: {}",
                input
            );
        }

        // Digit processing - 4-digit boundary cases
        let digit_boundaries = &[
            ("0.0000123", -2_i16),      // 4 leading zeros -> 2nd group
            ("0.00000001", -2_i16),     // 7 leading zeros -> 2nd group
            ("0.000000000001", -3_i16), // 11 leading zeros -> 3rd group
        ];
        for (input, expected_weight) in digit_boundaries {
            let parsed = input.parse::<Numeric>().unwrap();
            assert_eq!(
                parsed.weight(),
                *expected_weight,
                "Digit boundary: {}",
                input
            );
        }

        // ScientificLayout split logic edge cases
        let split_cases = &[
            ("12.34e-2", "0.1234"),     // Split within decimal
            ("123.45e-5", "0.0012345"), // Split creates leading zeros
            ("123e-1", "12.3"),         // Split exactly at boundary
            ("1e0", "1"),               // No-op scientific notation
        ];
        for (input, expected) in split_cases {
            let parsed = input.parse::<Numeric>().unwrap();
            assert_eq!(format!("{}", parsed), *expected, "Split case: {}", input);
        }

        // Error conditions with detailed messages
        let detailed_errors = &[
            "1e1000001", // scientific notation exponent too large
            "1e+",       // empty scientific notation exponent
            "1e-",       // empty scientific notation exponent
            "1e",        // empty scientific notation string
            "",          // empty numeric string
        ];
        for input in detailed_errors {
            assert!(input.parse::<Numeric>().is_err(), "Should error: {}", input);
        }

        // Zero with various scales to test calculate_scale zero handling
        let zero_scale_cases = &[
            ("0e-10", 10_u16),  // Zero with implied scale
            ("0.0e5", 1_u16),   // Zero preserving original scale
            ("0.000e3", 3_u16), // Zero with decimal scale
        ];
        for (input, expected_scale) in zero_scale_cases {
            let parsed = input.parse::<Numeric>().unwrap();
            assert_eq!(
                parsed.scale(),
                *expected_scale,
                "Zero scale case: {}",
                input
            );
        }
    }

    use proptest::prelude::*;
    proptest! {
        #[test]
        fn test_arbitrary_f64_from_string_and_back(value in any::<f64>()) {
            let prop_val = value.to_string();
            let numeric = Numeric::from_str(&prop_val).expect("parse numeric");
            let str_val = numeric.to_string();
            assert_eq!(prop_val, str_val, "proprty test value {value}");
        }
        #[test]
        fn test_arbitrary_i64_from_string_and_back(value in any::<i64>()) {
            let prop_val = value.to_string();
            let numeric = Numeric::from_str(&prop_val).expect("parse numeric");
            let str_val = numeric.to_string();
            assert_eq!(prop_val, str_val, "proprty test value {value}");
        }
    }
}
