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
}

impl PValue {
    pub fn is_null(&self) -> bool {
        matches!(self, PValue::Null)
    }

    /// If the `PValue` is a String, returns the associated str.
    /// Returns None otherwise.
    pub fn as_str(&self) -> Option<&str> {
        match self {
            PValue::String(s) => Some(s),
            _ => None,
        }
    }
}

pub type PValues = BTreeMap<String, PValue>;

pub type DateTime = chrono::DateTime<chrono::FixedOffset>;

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
