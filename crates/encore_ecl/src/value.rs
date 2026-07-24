use std::fmt;

/// Identifies the type of a [`Value`].
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum ValueKind {
    Number,
    Bool,
    String,
    Size,
    Duration,
}

impl ValueKind {
    /// Reports whether values of this kind support ordering comparisons.
    pub(crate) fn is_ordered(self) -> bool {
        matches!(
            self,
            ValueKind::Number | ValueKind::Size | ValueKind::Duration
        )
    }
}

impl fmt::Display for ValueKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            ValueKind::Number => "number",
            ValueKind::Bool => "bool",
            ValueKind::String => "string",
            ValueKind::Size => "size",
            ValueKind::Duration => "duration",
        };
        f.write_str(s)
    }
}

/// An ECL value: a number, bool, string, size, or duration. Sizes are stored
/// canonically in bytes, durations in milliseconds.
#[derive(Clone, Debug)]
pub struct Value {
    pub kind: ValueKind,
    /// `Number`: the number; `Size`: bytes; `Duration`: milliseconds
    pub num: f64,
    /// `String`
    pub str: String,
    /// `Bool`
    pub bool: bool,

    /// display unit for sizes/durations ("" picks one automatically)
    pub(crate) unit: String,
}

impl Default for Value {
    fn default() -> Value {
        Value {
            kind: ValueKind::Number,
            num: 0.0,
            str: String::new(),
            bool: false,
            unit: String::new(),
        }
    }
}

/// An error parsing or constructing a [`Value`].
#[derive(Debug, thiserror::Error)]
pub enum ValueError {
    #[error("unknown size unit {} (valid units: {valid})", go_quote(.unit))]
    UnknownSizeUnit { unit: String, valid: String },
    #[error("unknown duration unit {} (valid units: {valid})", go_quote(.unit))]
    UnknownDurationUnit { unit: String, valid: String },
    #[error("invalid quantity {}", go_quote(.0))]
    InvalidQuantity(String),
    #[error("unknown unit {} in quantity {}", go_quote(.unit), go_quote(.quantity))]
    UnknownUnitInQuantity { unit: String, quantity: String },
    #[error("value {} normalizes to an empty resource name", go_quote(.0))]
    EmptyNormalizedName(String),
}

/// Returns a numeric [`Value`].
pub fn number(v: f64) -> Value {
    Value {
        kind: ValueKind::Number,
        num: v,
        ..Value::default()
    }
}

/// Returns a boolean [`Value`].
pub fn boolean(b: bool) -> Value {
    Value {
        kind: ValueKind::Bool,
        bool: b,
        ..Value::default()
    }
}

/// Returns a string [`Value`].
pub fn string(s: impl Into<String>) -> Value {
    Value {
        kind: ValueKind::String,
        str: s.into(),
        ..Value::default()
    }
}

/// Returns a size [`Value`] of `v` units, e.g. `size(512.0, "Mi")`.
pub fn size(v: f64, unit: &str) -> Result<Value, ValueError> {
    match size_factor(unit) {
        Some(factor) => Ok(Value {
            kind: ValueKind::Size,
            num: v * factor,
            unit: unit.to_string(),
            ..Value::default()
        }),
        None => Err(ValueError::UnknownSizeUnit {
            unit: unit.to_string(),
            valid: unit_list(SIZE_UNITS),
        }),
    }
}

/// Returns a duration [`Value`] of `v` units, e.g. `duration(30.0, "d")`.
pub fn duration(v: f64, unit: &str) -> Result<Value, ValueError> {
    match duration_factor(unit) {
        Some(factor) => Ok(Value {
            kind: ValueKind::Duration,
            num: v * factor,
            unit: unit.to_string(),
            ..Value::default()
        }),
        None => Err(ValueError::UnknownDurationUnit {
            unit: unit.to_string(),
            valid: unit_list(DURATION_UNITS),
        }),
    }
}

/// Parses a number with an optional unit suffix, such as "512Mi", "30d", or
/// "2.5". Panics on invalid input; intended for tests and static
/// initialization.
pub fn must_parse_quantity(s: &str) -> Value {
    parse_quantity(s).expect("must_parse_quantity")
}

/// Parses a number with an optional unit suffix, such as "512Mi", "30d", or
/// "2.5".
pub fn parse_quantity(s: &str) -> Result<Value, ValueError> {
    let bytes = s.as_bytes();
    let mut i = 0;
    while i < bytes.len() && (bytes[i].is_ascii_digit() || bytes[i] == b'.' || bytes[i] == b'-') {
        i += 1;
    }
    let num: f64 = s[..i]
        .parse()
        .map_err(|_| ValueError::InvalidQuantity(s.to_string()))?;
    let unit = &s[i..];
    if unit.is_empty() {
        Ok(number(num))
    } else if size_factor(unit).is_some() {
        size(num, unit)
    } else if duration_factor(unit).is_some() {
        duration(num, unit)
    } else {
        Err(ValueError::UnknownUnitInQuantity {
            unit: unit.to_string(),
            quantity: s.to_string(),
        })
    }
}

const SIZE_UNITS: &[(&str, f64)] = &[
    ("B", 1.0),
    ("KB", 1e3),
    ("MB", 1e6),
    ("GB", 1e9),
    ("TB", 1e12),
    ("Ki", 1024.0),
    ("Mi", 1_048_576.0),
    ("Gi", 1_073_741_824.0),
    ("Ti", 1_099_511_627_776.0),
];

const DURATION_UNITS: &[(&str, f64)] = &[
    ("ms", 1.0),
    ("s", 1000.0),
    ("m", 60.0 * 1000.0),
    ("h", 60.0 * 60.0 * 1000.0),
    ("d", 24.0 * 60.0 * 60.0 * 1000.0),
];

pub(crate) fn size_factor(unit: &str) -> Option<f64> {
    SIZE_UNITS.iter().find(|(u, _)| *u == unit).map(|(_, f)| *f)
}

pub(crate) fn duration_factor(unit: &str) -> Option<f64> {
    DURATION_UNITS
        .iter()
        .find(|(u, _)| *u == unit)
        .map(|(_, f)| *f)
}

pub(crate) fn size_unit_list() -> String {
    unit_list(SIZE_UNITS)
}

pub(crate) fn duration_unit_list() -> String {
    unit_list(DURATION_UNITS)
}

/// All known unit names (size units followed by duration units), for typo
/// suggestions.
pub(crate) fn all_unit_names() -> Vec<String> {
    SIZE_UNITS
        .iter()
        .chain(DURATION_UNITS.iter())
        .map(|(u, _)| u.to_string())
        .collect()
}

fn unit_list(units: &[(&str, f64)]) -> String {
    let mut names: Vec<(&str, f64)> = units.to_vec();
    names.sort_by(|a, b| {
        a.1.partial_cmp(&b.1)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then(a.0.cmp(b.0))
    });
    names.iter().map(|(n, _)| *n).collect::<Vec<_>>().join(", ")
}

impl fmt::Display for Value {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.kind {
            ValueKind::Number => f.write_str(&format_float(self.num)),
            ValueKind::Bool => f.write_str(if self.bool { "true" } else { "false" }),
            ValueKind::String => f.write_str(&go_quote(&self.str)),
            ValueKind::Size => f.write_str(&format_quantity(self.num, &self.unit, SIZE_UNITS, "B")),
            ValueKind::Duration => {
                f.write_str(&format_quantity(self.num, &self.unit, DURATION_UNITS, "ms"))
            }
        }
    }
}

/// Formats a float as Go's `strconv.FormatFloat(f, 'g', -1, 64)` would for the
/// value ranges ECL uses. Rust's default `f64` `Display` is also shortest
/// round-trip and matches Go for quantities; extreme exponents are not
/// exercised by ECL.
pub(crate) fn format_float(f: f64) -> String {
    if f.is_nan() {
        return "NaN".to_string();
    }
    if f.is_infinite() {
        return if f < 0.0 { "-Inf" } else { "+Inf" }.to_string();
    }
    format!("{f}")
}

fn format_quantity(canonical: f64, unit: &str, units: &[(&str, f64)], base: &str) -> String {
    let mut unit = unit.to_string();
    if unit.is_empty() {
        // Pick the largest unit that divides the value evenly.
        let mut best = base;
        let mut best_factor = units
            .iter()
            .find(|(u, _)| *u == base)
            .map(|(_, f)| *f)
            .unwrap();
        for (u, factor) in units {
            let scaled = canonical / factor;
            if scaled >= 1.0 && scaled == (scaled as i64) as f64 && *factor > best_factor {
                best = u;
                best_factor = *factor;
            }
        }
        unit = best.to_string();
    }
    let factor = units
        .iter()
        .find(|(u, _)| *u == unit)
        .map(|(_, f)| *f)
        .unwrap();
    format!("{}{unit}", format_float(canonical / factor))
}

/// Reports whether two values are equal. Values of different kinds are never
/// equal.
pub(crate) fn values_equal(a: &Value, b: &Value) -> bool {
    if a.kind != b.kind {
        return false;
    }
    match a.kind {
        ValueKind::Number | ValueKind::Size | ValueKind::Duration => a.num == b.num,
        ValueKind::Bool => a.bool == b.bool,
        ValueKind::String => a.str == b.str,
    }
}

/// Normalizes a dynamic block/reference value into a valid resource name: it
/// trims surrounding whitespace, lowercases, replaces each run of invalid
/// characters (anything other than a-z, 0-9 or '-') with a single '-', trims
/// leading and trailing '-', and rejects an empty result.
pub(crate) fn normalize_dynamic_name(s: &str) -> Result<String, ValueError> {
    let mut b = String::new();
    let mut prev_dash = false;
    for r in s.trim().chars() {
        match r {
            'A'..='Z' => {
                b.push(r.to_ascii_lowercase());
                prev_dash = false;
            }
            'a'..='z' | '0'..='9' => {
                b.push(r);
                prev_dash = false;
            }
            _ => {
                if !prev_dash {
                    b.push('-');
                    prev_dash = true;
                }
            }
        }
    }
    let name = b.trim_matches('-').to_string();
    if name.is_empty() {
        return Err(ValueError::EmptyNormalizedName(s.to_string()));
    }
    Ok(name)
}

/// Renders a string the way Go's `strconv.Quote` does for the inputs ECL uses:
/// wrapped in double quotes with the standard escapes.
pub(crate) fn go_quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for r in s.chars() {
        match r {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\t' => out.push_str("\\t"),
            '\r' => out.push_str("\\r"),
            '\u{7}' => out.push_str("\\a"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '\u{b}' => out.push_str("\\v"),
            c if c == ' ' || (c.is_ascii_graphic()) || (!c.is_control() && c as u32 >= 0x80) => {
                out.push(c)
            }
            c if (c as u32) < 0x80 => out.push_str(&format!("\\x{:02x}", c as u32)),
            c => out.push_str(&format!("\\u{:04x}", c as u32)),
        }
    }
    out.push('"');
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_quantity_works() {
        let v = must_parse_quantity("512Mi");
        assert_eq!(v.kind, ValueKind::Size);
        assert_eq!(v.num, (512 * 1024 * 1024) as f64);
        assert_eq!(v.to_string(), "512Mi");

        let v = must_parse_quantity("30d");
        assert_eq!(v.kind, ValueKind::Duration);
        assert_eq!(v.num, (30i64 * 24 * 60 * 60 * 1000) as f64);
        assert_eq!(v.to_string(), "30d");

        let v = must_parse_quantity("2.5");
        assert_eq!(v.kind, ValueKind::Number);
        assert_eq!(v.num, 2.5);
        assert_eq!(v.to_string(), "2.5");

        let e = parse_quantity("10grams").unwrap_err();
        assert!(e.to_string().contains("unknown unit \"grams\""), "{e}");
        let e = parse_quantity("abc").unwrap_err();
        assert_eq!(e.to_string(), "invalid quantity \"abc\"");
    }

    #[test]
    fn value_equality() {
        // Different unit spellings of the same quantity are equal.
        assert!(values_equal(
            &must_parse_quantity("1Gi"),
            &must_parse_quantity("1024Mi")
        ));
        assert!(values_equal(
            &must_parse_quantity("1m"),
            &must_parse_quantity("60s")
        ));
        assert!(!values_equal(
            &must_parse_quantity("1GB"),
            &must_parse_quantity("1Gi")
        ));

        // Different kinds are never equal, even with the same numeric value.
        assert!(!values_equal(
            &number(1024.0),
            &must_parse_quantity("1024B")
        ));
        assert!(!values_equal(&boolean(true), &string("true")));
    }

    #[test]
    fn value_string() {
        assert_eq!(number(0.5).to_string(), "0.5");
        assert_eq!(number(8.0).to_string(), "8");
        assert_eq!(boolean(false).to_string(), "false");
        assert_eq!(string("europe-west1").to_string(), "\"europe-west1\"");

        // Values without a display unit pick a sensible one automatically.
        assert_eq!(
            Value {
                kind: ValueKind::Size,
                num: 2048.0,
                ..Value::default()
            }
            .to_string(),
            "2Ki"
        );
        assert_eq!(
            Value {
                kind: ValueKind::Duration,
                num: 90_000.0,
                ..Value::default()
            }
            .to_string(),
            "90s"
        );
    }
}
