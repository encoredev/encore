use std::{
    collections::BTreeMap,
    fmt::{Debug, Display},
};

use serde::{Serialize, Serializer};

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
        write!(f, "{}", c)
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
        }
    }
}

impl Display for PValue {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PValue::Null => write!(f, "null"),
            PValue::Bool(b) => write!(f, "{}", b),
            PValue::Number(n) => write!(f, "{}", n),
            PValue::String(s) => write!(f, "{}", s),
            PValue::DateTime(dt) => write!(f, "{}", dt.to_rfc3339()),
            PValue::Array(a) => {
                write!(f, "[")?;
                for (i, v) in a.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}", v)?;
                }
                write!(f, "]")
            }
            PValue::Object(o) => {
                write!(f, "{{")?;
                for (i, (k, v)) in o.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{}: {}", k, v)?;
                }
                write!(f, "}}")
            }
            PValue::Cookie(c) => write!(f, "{}", c),
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
