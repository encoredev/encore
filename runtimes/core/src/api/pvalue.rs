use std::{
    collections::BTreeMap,
    fmt::{Debug, Display},
    ops::{Add, Div, Mul, Sub},
    str::FromStr,
};

use bytes::BytesMut;
use malachite::rational::{conversion::primitive_int_from_rational, Rational};
use malachite::{
    base::num::conversion::{
        string::options::ToSciOptions,
        traits::{FromSciString, ToSci},
    },
    rational::conversion::primitive_float_from_rational,
};
use serde::{Serialize, Serializer};

use crate::sqldb;

/// Represents any valid value in a request/response payload.
///
/// It is a more type-safe version of JSON, where we support additional
/// semantic types like timestamps.
#[derive(Clone, Eq, PartialEq, Debug)]
pub enum PValue {
    /// Represents a JSON null value.
    Null,

    /// Represents a JSON boolean.
    Bool(bool),

    /// Represents a JSON number, whether integer or floating point.
    Number(serde_json::Number),

    /// Represents a Decimal type with arbitrary precision.
    Decimal(Decimal),

    /// Represents a JSON string.
    String(String),

    /// Represents a JSON array.
    Array(Vec<PValue>),

    /// Represents a JSON object.
    Object(PValues),

    // Represents a datetime value.
    DateTime(DateTime),

    // Represents a cookie.
    Cookie(Cookie),
}

impl PValue {
    pub fn is_null(&self) -> bool {
        matches!(self, PValue::Null)
    }

    pub fn is_array(&self) -> bool {
        matches!(self, PValue::Array(..))
    }

    /// If the `PValue` is a String, returns the associated str.
    /// Returns None otherwise.
    pub fn as_str(&self) -> Option<&str> {
        match self {
            PValue::String(s) => Some(s),
            _ => None,
        }
    }

    pub fn type_name(&self) -> &'static str {
        match self {
            PValue::Null => "null",
            PValue::Bool(_) => "boolean",
            PValue::Number(_) => "number",
            PValue::String(_) => "string",
            PValue::Array(_) => "array",
            PValue::Object(_) => "object",
            PValue::DateTime(_) => "datetime",
            PValue::Cookie(_) => "cookie",
            PValue::Decimal(_) => "decimal",
        }
    }
}

pub type PValues = BTreeMap<String, PValue>;

pub type DateTime = chrono::DateTime<chrono::FixedOffset>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Cookie {
    pub name: String,
    pub value: Box<PValue>,
    pub path: Option<String>,
    pub domain: Option<String>,
    pub secure: Option<bool>,
    pub http_only: Option<bool>,
    pub expires: Option<DateTime>,
    pub max_age: Option<u64>,
    pub same_site: Option<SameSite>,
    pub partitioned: Option<bool>,
}

impl<'a> From<&'a Cookie> for cookie::Cookie<'a> {
    fn from(value: &'a Cookie) -> Self {
        let mut builder = cookie::CookieBuilder::new(&value.name, value.value.to_string());
        if let Some(path) = &value.path {
            builder = builder.path(path);
        }
        if let Some(domain) = &value.domain {
            builder = builder.domain(domain);
        }
        if let Some(secure) = &value.secure {
            builder = builder.secure(*secure);
        }
        if let Some(http_only) = &value.http_only {
            builder = builder.http_only(*http_only);
        }
        if let Some(expires) = &value.expires {
            let system_time: std::time::SystemTime = (*expires).into();
            let expire = cookie::time::OffsetDateTime::from(system_time);
            builder = builder.expires(expire);
        }
        if let Some(max_age) = &value.max_age {
            builder = builder.max_age(cookie::time::Duration::seconds(*max_age as i64));
        }
        if let Some(same_site) = &value.same_site {
            let same_site = match same_site {
                SameSite::Strict => cookie::SameSite::Strict,
                SameSite::Lax => cookie::SameSite::Lax,
                SameSite::None => cookie::SameSite::None,
            };
            builder = builder.same_site(same_site);
        }
        if let Some(partitioned) = &value.partitioned {
            builder = builder.partitioned(*partitioned);
        }

        builder.build()
    }
}

impl Display for Cookie {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let c: cookie::Cookie<'_> = self.into();
        write!(f, "{c}")
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SameSite {
    Strict,
    Lax,
    None,
}

impl Display for SameSite {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SameSite::Strict => write!(f, "Strict"),
            SameSite::Lax => write!(f, "Lax"),
            SameSite::None => write!(f, "None"),
        }
    }
}

#[derive(Clone, Hash, Eq, PartialEq, Debug)]
pub struct Decimal(Rational);

impl Add for &Decimal {
    type Output = Decimal;

    fn add(self, rhs: Self) -> Self::Output {
        Decimal((&self.0).add(&rhs.0))
    }
}
impl Sub for &Decimal {
    type Output = Decimal;

    fn sub(self, rhs: Self) -> Self::Output {
        Decimal((&self.0).sub(&rhs.0))
    }
}
impl Mul for &Decimal {
    type Output = Decimal;

    fn mul(self, rhs: Self) -> Self::Output {
        Decimal((&self.0).mul(&rhs.0))
    }
}
impl Div for &Decimal {
    type Output = Decimal;

    fn div(self, rhs: Self) -> Self::Output {
        Decimal((&self.0).div(&rhs.0))
    }
}

impl TryFrom<&Decimal> for i64 {
    type Error = primitive_int_from_rational::SignedFromRationalError;

    fn try_from(value: &Decimal) -> Result<Self, Self::Error> {
        i64::try_from(&value.0)
    }
}

impl TryFrom<&Decimal> for i32 {
    type Error = primitive_int_from_rational::SignedFromRationalError;

    fn try_from(value: &Decimal) -> Result<Self, Self::Error> {
        i32::try_from(&value.0)
    }
}

impl TryFrom<&Decimal> for i16 {
    type Error = primitive_int_from_rational::SignedFromRationalError;

    fn try_from(value: &Decimal) -> Result<Self, Self::Error> {
        i16::try_from(&value.0)
    }
}

impl TryFrom<&Decimal> for f64 {
    type Error = primitive_float_from_rational::FloatConversionError;

    fn try_from(value: &Decimal) -> Result<Self, Self::Error> {
        f64::try_from(&value.0)
    }
}

impl TryFrom<&Decimal> for f32 {
    type Error = primitive_float_from_rational::FloatConversionError;

    fn try_from(value: &Decimal) -> Result<Self, Self::Error> {
        f32::try_from(&value.0)
    }
}

impl PartialEq<f64> for &Decimal {
    fn eq(&self, other: &f64) -> bool {
        self.0 == *other
    }
}

impl PartialOrd<f64> for &Decimal {
    fn partial_cmp(&self, other: &f64) -> Option<std::cmp::Ordering> {
        self.0.partial_cmp(other)
    }
}

impl FromStr for Decimal {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let r = Rational::from_sci_string(s)
            .ok_or_else(|| anyhow::anyhow!("Failed to parse decimal from string: {s}"))?;
        Ok(Decimal(r))
    }
}

impl Display for Decimal {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let mut opts = ToSciOptions::default();
        opts.set_size_complete();
        opts.set_include_trailing_zeros(true);
        if !self.0.fmt_sci_valid(opts) {
            // e.g the number is 1/3
            opts.set_scale(10);
        }
        self.0.fmt_sci(f, opts)
    }
}

impl From<Rational> for Decimal {
    fn from(r: Rational) -> Self {
        Decimal(r)
    }
}

impl tokio_postgres::types::ToSql for Decimal {
    fn to_sql(
        &self,
        _ty: &tokio_postgres::types::Type,
        out: &mut BytesMut,
    ) -> Result<tokio_postgres::types::IsNull, Box<dyn std::error::Error + Sync + Send>> {
        let n = sqldb::numeric::Numeric::from_str(&self.to_string())?;
        sqldb::numeric::numeric_to_sql(n, out);
        Ok(tokio_postgres::types::IsNull::No)
    }

    tokio_postgres::types::accepts!(NUMERIC);
    tokio_postgres::types::to_sql_checked!();
}

impl<'a> tokio_postgres::types::FromSql<'a> for Decimal {
    fn from_sql(
        _ty: &tokio_postgres::types::Type,
        raw: &[u8],
    ) -> Result<Self, Box<dyn std::error::Error + Sync + Send>> {
        let n = sqldb::numeric::numeric_from_sql(raw)?;
        let d = Decimal::from_str(&n.to_string())?;
        Ok(d)
    }

    tokio_postgres::types::accepts!(NUMERIC);
}

impl Serialize for PValue {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        match self {
            PValue::Null => serializer.serialize_unit(),
            PValue::Bool(b) => serializer.serialize_bool(*b),
            PValue::Number(n) => n.serialize(serializer),
            PValue::String(s) => serializer.serialize_str(s),
            PValue::Array(a) => a.serialize(serializer),
            PValue::Object(o) => o.serialize(serializer),
            PValue::DateTime(dt) => dt.serialize(serializer),
            PValue::Cookie(c) => serializer.serialize_str(&c.to_string()),
            PValue::Decimal(d) => serializer.serialize_str(&d.to_string()),
        }
    }
}

impl Display for PValue {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PValue::Null => write!(f, "null"),
            PValue::Bool(b) => write!(f, "{b}"),
            PValue::Number(n) => write!(f, "{n}"),
            PValue::String(s) => write!(f, "{s}"),
            PValue::DateTime(dt) => write!(f, "{}", dt.to_rfc3339()),
            PValue::Array(a) => {
                write!(f, "[")?;
                for (i, v) in a.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{v}")?;
                }
                write!(f, "]")
            }
            PValue::Object(o) => {
                write!(f, "{{")?;
                for (i, (k, v)) in o.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{k}: {v}")?;
                }
                write!(f, "}}")
            }
            PValue::Cookie(c) => write!(f, "{c}"),
            PValue::Decimal(d) => write!(f, "{d}",),
        }
    }
}

impl From<serde_json::Value> for PValue {
    fn from(value: serde_json::Value) -> Self {
        match value {
            serde_json::Value::Null => PValue::Null,
            serde_json::Value::Bool(b) => PValue::Bool(b),
            serde_json::Value::Number(n) => PValue::Number(n),
            serde_json::Value::String(s) => PValue::String(s),
            serde_json::Value::Array(a) => PValue::Array(a.into_iter().map(PValue::from).collect()),
            serde_json::Value::Object(o) => {
                PValue::Object(o.into_iter().map(|(k, v)| (k, PValue::from(v))).collect())
            }
        }
    }
}
