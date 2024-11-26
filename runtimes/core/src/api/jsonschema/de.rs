use std::borrow::Cow;
use std::collections::{BTreeMap, HashMap, HashSet};
use std::fmt;
use std::fmt::Display;
use std::marker::PhantomData;

use serde::de::{DeserializeSeed, MapAccess, SeqAccess, Unexpected, Visitor};
use serde::Deserializer;

use crate::api::jsonschema::{validation::Validation, Registry};
use crate::api::{self, PValue, PValues};

use serde_json::Number as JSONNumber;
use serde_json::Value as JVal;

#[derive(Debug, Clone)]
pub enum Value {
    // A JSON primitive (e.g. string, number, boolean, null).
    Basic(Basic),

    // A literal value.
    Literal(Literal),

    /// Consume a value if present.
    Option(BasicOrValue),

    /// Consume a map of key-value pairs (the keys are always strings in JSON).
    Map(BasicOrValue),

    /// A struct with a set of known fields.
    Struct(Struct),

    /// Consume an array of values.
    Array(BasicOrValue),

    /// Consume a single value, one of a union of possible types.
    Union(Vec<BasicOrValue>),

    /// Reference to another value.
    Ref(usize),

    /// A value with additional value-based validation.
    Validation(Validation),
}

#[derive(Debug, Clone, Default)]
pub struct Struct {
    pub fields: HashMap<String, Field>,
}

impl Struct {
    pub fn is_empty(&self) -> bool {
        self.fields.is_empty()
    }

    pub fn contains_name(&self, name: &str) -> bool {
        self.fields
            .iter()
            .any(|(field_name, field)| field.name_override.as_deref().unwrap_or(field_name) == name)
    }
}

#[derive(Debug, Clone)]
pub struct Field {
    pub value: BasicOrValue,
    pub optional: bool,
    pub name_override: Option<String>,
}

impl Value {
    pub fn is_basic(&self) -> bool {
        matches!(self, Value::Basic(_))
    }

    pub fn is_option(&self) -> bool {
        matches!(self, Value::Option(_))
    }

    pub fn is_map(&self) -> bool {
        matches!(self, Value::Map(_))
    }

    pub fn is_struct(&self) -> bool {
        matches!(self, Value::Struct { .. })
    }

    pub fn is_array(&self) -> bool {
        matches!(self, Value::Array(_))
    }

    pub fn is_ref(&self) -> bool {
        matches!(self, Value::Ref(_))
    }

    pub fn expecting<'a>(&'a self, reg: &'a Registry) -> Cow<'a, str> {
        match self {
            Value::Array(_) => Cow::Borrowed("a JSON array"),
            Value::Basic(basic) => Cow::Borrowed(basic.expecting()),
            Value::Map(_) => Cow::Borrowed("a JSON map"),
            Value::Literal(lit) => Cow::Owned(lit.expecting()),
            Value::Option(bov) => bov.expecting(reg),
            Value::Ref(idx) => reg.get(*idx).expecting(reg),
            Value::Struct { .. } => Cow::Borrowed("a JSON object"),
            Value::Validation(v) => v.bov.expecting(reg),
            Value::Union(types) => {
                let mut s = String::new();
                let num = types.len();
                for (i, typ) in types.iter().enumerate() {
                    if i > 0 {
                        if i == (num - 1) && num > 2 {
                            s.push_str(", or ");
                        } else if i == (num - 1) {
                            s.push_str(" or ");
                        } else {
                            s.push_str(", ");
                        }
                    }
                    s.push_str(&typ.expecting(reg));
                }
                Cow::Owned(s)
            }
        }
    }
}

#[derive(Debug, Copy, Clone)]
pub enum Basic {
    Any, // Any valid JSON value.
    Null,
    Bool,
    Number,
    String,
    DateTime,
}

impl Basic {
    pub fn expecting(&self) -> &'static str {
        match self {
            Basic::Any => "any valid JSON value",
            Basic::Null => "null",
            Basic::Bool => "a boolean",
            Basic::Number => "a number",
            Basic::String => "a string",
            Basic::DateTime => "a datetime string",
        }
    }
}

#[derive(Debug, Clone)]
pub enum Literal {
    Str(String), // A literal string
    Bool(bool),
    Int(i64),
    Float(f64),
}

impl Literal {
    pub fn expecting(&self) -> String {
        match self {
            Literal::Str(lit) => format!("{:#?}", lit),
            Literal::Bool(lit) => format!("{:#?}", lit),
            Literal::Int(lit) => format!("{:#?}", lit),
            Literal::Float(lit) => format!("{:#?}", lit),
        }
    }

    pub fn expecting_type(&self) -> &'static str {
        match self {
            Literal::Str(_) => "string",
            Literal::Bool(_) => "boolean",
            Literal::Int(_) => "integer",
            Literal::Float(_) => "number",
        }
    }
}

#[derive(Debug, Copy, Clone)]
pub enum BasicOrValue {
    Basic(Basic),
    Value(usize),
}

impl BasicOrValue {
    pub fn expecting<'a>(&'a self, reg: &'a Registry) -> Cow<'a, str> {
        match self {
            BasicOrValue::Basic(basic) => Cow::Borrowed(basic.expecting()),
            BasicOrValue::Value(idx) => reg.get(*idx).expecting(reg),
        }
    }
}

#[derive(Debug, Default, Clone)]
pub struct DecodeConfig {
    // If true, attempts to parse strings as other primitive types
    // when there's a type mismatch.
    pub coerce_strings: bool,
}

#[derive(Copy, Clone, Debug)]
pub(super) struct DecodeValue<'a> {
    pub(super) cfg: &'a DecodeConfig,
    pub(super) reg: &'a Registry,
    pub(super) value: &'a Value,
}

impl<'a> DecodeValue<'a> {
    fn resolve(&'a self, idx: usize) -> DecodeValue<'a> {
        DecodeValue {
            cfg: self.cfg,
            reg: self.reg,
            value: &self.reg.values[idx],
        }
    }
}

impl<'de: 'a, 'a> DeserializeSeed<'de> for DecodeValue<'a> {
    type Value = PValue;

    fn deserialize<D>(self, deserializer: D) -> Result<Self::Value, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(self)
    }
}

macro_rules! recurse_ref {
    ($self:ident, $idx:expr, $method:ident, $value:expr) => {{
        let visitor = DecodeValue {
            cfg: $self.cfg,
            reg: $self.reg,
            value: &$self.reg.values[*$idx],
        };
        visitor.$method($value)
    }};
}

macro_rules! recurse_ref0 {
    ($self:ident, $idx:expr, $method:ident) => {{
        let visitor = DecodeValue {
            cfg: $self.cfg,
            reg: $self.reg,
            value: &$self.reg.values[*$idx],
        };
        visitor.$method()
    }};
}

macro_rules! recurse {
    ($self:ident, $bov:expr, $method:ident, $value:expr) => {{
        match $bov {
            BasicOrValue::Basic(basic) => {
                let basic_val = Value::Basic(*basic);
                let visitor = DecodeValue {
                    cfg: $self.cfg,
                    reg: $self.reg,
                    value: &basic_val,
                };
                visitor.$method($value)
            }
            BasicOrValue::Value(idx) => {
                let visitor = DecodeValue {
                    cfg: $self.cfg,
                    reg: $self.reg,
                    value: &$self.reg.values[*idx],
                };
                visitor.$method($value)
            }
        }
    }};
}

macro_rules! validate_pval {
    ($self:ident, $v:ident, $method:ident, $value:expr) => {{
        let inner = recurse!($self, &$v.bov, $method, $value)?;
        match $v.validate_pval(&inner) {
            Ok(()) => Ok(inner),
            Err(err) => Err(serde::de::Error::custom(err)),
        }
    }};
}

macro_rules! validate_jval {
    ($self:ident, $v:ident, $method:ident, $value:expr) => {{
        recurse!($self, &$v.bov, $method, $value)?;
        match $v.validate_jval($value) {
            Ok(()) => Ok(()),
            Err(err) => Err(serde::de::Error::custom(err)),
        }
    }};
}

macro_rules! recurse0 {
    ($self:ident, $bov:expr, $method:ident) => {{
        match $bov {
            BasicOrValue::Basic(basic) => {
                let basic_val = Value::Basic(*basic);
                let visitor = DecodeValue {
                    cfg: $self.cfg,
                    reg: $self.reg,
                    value: &basic_val,
                };
                visitor.$method()
            }
            BasicOrValue::Value(idx) => {
                let visitor = DecodeValue {
                    cfg: $self.cfg,
                    reg: &$self.reg,
                    value: &$self.reg.values[*idx],
                };
                visitor.$method()
            }
        }
    }};
}

impl<'de> Visitor<'de> for DecodeValue<'_> {
    type Value = PValue;

    fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self.value {
            Value::Literal(lit) => {
                let s = lit.expecting();
                formatter.write_str(&s)
            }
            Value::Basic(b) => formatter.write_str(match b {
                Basic::Any => "any valid JSON value",
                Basic::Null => "null",
                Basic::Bool => "a boolean",
                Basic::Number => "a number",
                Basic::String => "a string",
                Basic::DateTime => "a datetime string",
            }),
            Value::Map(_) => formatter.write_str("a JSON object"),
            Value::Array(_) => formatter.write_str("a JSON array"),
            Value::Union(union) => {
                let num = union.len();
                let mut s = String::new();
                for (i, typ) in union.iter().enumerate() {
                    if i > 0 {
                        if num > 2 && i == (num - 1) {
                            s.push_str(", or ");
                        } else if i == (num - 1) {
                            s.push_str(" or ");
                        } else {
                            s.push_str(", ");
                        }
                    }
                    let expecting = typ.expecting(self.reg);
                    s.push_str(&expecting);
                }
                formatter.write_str(&s)
            }
            Value::Option(_) => formatter.write_str("any valid JSON value or null"),
            Value::Struct { .. } => formatter.write_str("a JSON object"),
            Value::Ref(_) => formatter.write_str("a JSON value"),
            Value::Validation(v) => formatter.write_str(v.bov.expecting(self.reg).as_ref()),
        }
    }

    #[inline]
    fn visit_bool<E>(self, value: bool) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match &self.value {
            Value::Basic(Basic::Any | Basic::Bool) => Ok(PValue::Bool(value)),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_bool, value),
            Value::Option(val) => {
                recurse!(self, val, visit_bool, value)
            }
            Value::Literal(Literal::Bool(bool)) if *bool == value => Ok(PValue::Bool(value)),
            Value::Validation(v) => validate_pval!(self, v, visit_bool, value),
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_bool, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(
                    Unexpected::Bool(value),
                    &self,
                ))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Bool(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_i64<E>(self, value: i64) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => Ok(PValue::Number(value.into())),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_i64, value),
            Value::Option(val) => {
                recurse!(self, val, visit_i64, value)
            }
            Value::Literal(Literal::Int(val)) if *val == value => Ok(PValue::Number(value.into())),
            Value::Validation(v) => validate_pval!(self, v, visit_i64, value),
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_i64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(
                    Unexpected::Signed(value),
                    &self,
                ))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Signed(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_u64<E>(self, value: u64) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => Ok(PValue::Number(value.into())),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_u64, value),
            Value::Option(val) => {
                recurse!(self, val, visit_u64, value)
            }
            Value::Literal(Literal::Int(val)) if *val == value as i64 => {
                Ok(PValue::Number(value.into()))
            }
            Value::Validation(v) => validate_pval!(self, v, visit_u64, value),
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_u64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(
                    Unexpected::Unsigned(value),
                    &self,
                ))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Unsigned(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_f64<E>(self, value: f64) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => {
                Ok(JSONNumber::from_f64(value).map_or(PValue::Null, PValue::Number))
            }
            Value::Ref(idx) => recurse_ref!(self, idx, visit_f64, value),
            Value::Option(bov) => {
                recurse!(self, bov, visit_f64, value)
            }
            Value::Literal(Literal::Float(val)) if *val == value => {
                if let Some(num) = JSONNumber::from_f64(value) {
                    Ok(PValue::Number(num))
                } else {
                    Err(serde::de::Error::custom(format_args!(
                        "expected {}, got {}",
                        val, value
                    )))
                }
            }
            Value::Validation(v) => validate_pval!(self, v, visit_f64, value),
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_f64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(
                    Unexpected::Float(value),
                    &self,
                ))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Float(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_str<E>(self, value: &str) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        self.visit_string(String::from(value))
    }

    #[inline]
    #[cfg_attr(
        feature = "rttrace",
        tracing::instrument(skip(self), ret, level = "trace")
    )]
    fn visit_string<E>(self, value: String) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(b) => match b {
                Basic::Any | Basic::String => Ok(PValue::String(value)),
                Basic::DateTime => api::DateTime::parse_from_rfc3339(&value)
                    .map(PValue::DateTime)
                    .map_err(|e| {
                        serde::de::Error::custom(format_args!("invalid datetime: {}", e,))
                    }),
                Basic::Bool if self.cfg.coerce_strings => {
                    if value == "true" {
                        Ok(PValue::Bool(true))
                    } else if value == "false" {
                        Ok(PValue::Bool(false))
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected a boolean, got {}",
                            value
                        )))
                    }
                }
                Basic::Number if self.cfg.coerce_strings => {
                    if let Ok(num) = value.parse::<i64>() {
                        Ok(PValue::Number(num.into()))
                    } else if let Ok(num) = value.parse::<f64>() {
                        Ok(JSONNumber::from_f64(num).map_or(PValue::Null, PValue::Number))
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected a number, got {}",
                            value
                        )))
                    }
                }
                Basic::Null if self.cfg.coerce_strings => {
                    if value == "null" {
                        Ok(PValue::Null)
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected null, got {}",
                            value
                        )))
                    }
                }
                _ => Err(serde::de::Error::invalid_type(
                    Unexpected::Str(&value),
                    &self,
                )),
            },
            Value::Ref(idx) => recurse_ref!(self, idx, visit_string, value),
            Value::Option(bov) => {
                recurse!(self, bov, visit_string, value)
            }

            Value::Literal(Literal::Str(val)) if val.as_str() == value => Ok(PValue::String(value)),
            Value::Literal(lit) => match lit {
                Literal::Str(val) if val.as_str() == value => Ok(PValue::String(value)),
                Literal::Bool(_) if self.cfg.coerce_strings => {
                    if let Ok(got) = value.parse::<bool>() {
                        self.visit_bool(got)
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected {}, got {}",
                            lit.expecting(),
                            value,
                        )))
                    }
                }
                Literal::Int(_) if self.cfg.coerce_strings => {
                    if let Ok(got) = value.parse::<i64>() {
                        self.visit_i64(got)
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected {}, got {}",
                            lit.expecting(),
                            value,
                        )))
                    }
                }
                Literal::Float(_) if self.cfg.coerce_strings => {
                    if let Ok(got) = value.parse::<f64>() {
                        self.visit_f64(got)
                    } else {
                        Err(serde::de::Error::custom(format_args!(
                            "expected {}, got {}",
                            lit.expecting(),
                            value,
                        )))
                    }
                }
                _ => Err(serde::de::Error::custom(format_args!(
                    "expected {}, got {}",
                    lit.expecting(),
                    value,
                ))),
            },
            Value::Validation(v) => validate_pval!(self, v, visit_string, value),

            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_string, value.clone());
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(
                    Unexpected::Str(&value),
                    &self,
                ))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Str(&value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_none<E>(self) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Null) | Value::Option(_) => Ok(PValue::Null),
            Value::Ref(idx) => recurse_ref0!(self, idx, visit_none),
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse0!(self, typ, visit_none);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Option, &self))
            }
            _ => Err(serde::de::Error::invalid_type(Unexpected::Option, &self)),
        }
    }

    #[inline]
    fn visit_some<D>(self, deserializer: D) -> Result<PValue, D::Error>
    where
        D: Deserializer<'de>,
    {
        DeserializeSeed::deserialize(self, deserializer)
    }

    #[inline]
    fn visit_unit<E>(self) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        self.visit_none()
    }

    #[inline]
    #[cfg_attr(
        feature = "rttrace",
        tracing::instrument(skip(self, seq), ret, level = "trace")
    )]
    fn visit_seq<A>(self, mut seq: A) -> Result<PValue, A::Error>
    where
        A: SeqAccess<'de>,
    {
        match &self.value {
            Value::Basic(Basic::Any) => visit_seq(self, seq),
            Value::Array(bov) => match bov {
                BasicOrValue::Basic(basic) => {
                    let basic_val = Value::Basic(*basic);
                    let visitor = DecodeValue {
                        cfg: self.cfg,
                        reg: self.reg,
                        value: &basic_val,
                    };
                    visit_seq(visitor, seq)
                }
                BasicOrValue::Value(idx) => {
                    let visitor = DecodeValue {
                        cfg: self.cfg,
                        reg: self.reg,
                        value: &self.reg.values[*idx],
                    };
                    visit_seq(visitor, seq)
                }
            },
            Value::Ref(idx) => recurse_ref!(self, idx, visit_seq, seq),
            Value::Option(bov) => recurse!(self, bov, visit_seq, seq),
            Value::Validation(v) => validate_pval!(self, v, visit_seq, seq),
            Value::Union(candidates) => {
                let mut vec: Vec<serde_json::Value> = Vec::new();
                while let Some(val) = seq.next_element()? {
                    vec.push(val);
                }
                let arr = JVal::Array(vec);

                for c in candidates {
                    match c {
                        BasicOrValue::Basic(basic) => {
                            let basic_val = Value::Basic(*basic);
                            let visitor = DecodeValue {
                                cfg: self.cfg,
                                reg: self.reg,
                                value: &basic_val,
                            };
                            if visitor.validate::<A::Error>(&arr).is_ok() {
                                return visitor.transform(arr);
                            }
                        }
                        BasicOrValue::Value(idx) => {
                            let visitor = DecodeValue {
                                cfg: self.cfg,
                                reg: self.reg,
                                value: &self.reg.values[*idx],
                            };
                            if visitor.validate::<A::Error>(&arr).is_ok() {
                                return visitor.transform(arr);
                            }
                        }
                    }
                }

                Err(serde::de::Error::invalid_type(Unexpected::Seq, &self))
            }

            _ => Err(serde::de::Error::invalid_type(Unexpected::Seq, &self)),
        }
    }

    #[cfg_attr(
        feature = "rttrace",
        tracing::instrument(skip(self, map), ret, level = "trace")
    )]
    fn visit_map<A>(self, mut map: A) -> Result<PValue, A::Error>
    where
        A: MapAccess<'de>,
    {
        match &self.value {
            Value::Basic(Basic::Any) => visit_map(self, map),
            Value::Map(bov) => match bov {
                BasicOrValue::Basic(basic) => {
                    let basic_val = Value::Basic(*basic);
                    let visitor = DecodeValue {
                        cfg: self.cfg,
                        reg: self.reg,
                        value: &basic_val,
                    };
                    visit_map(visitor, map)
                }
                BasicOrValue::Value(idx) => {
                    let visitor = DecodeValue {
                        cfg: self.cfg,
                        reg: self.reg,
                        value: &self.reg.values[*idx],
                    };
                    visit_map(visitor, map)
                }
            },

            Value::Struct(Struct { fields }) => {
                let mut values = PValues::new();
                let mut seen = HashSet::new();
                while let Some(key) = map.next_key::<String>()? {
                    // Get the corresponding value from the schema.
                    match fields.get(&key) {
                        Some(entry) => {
                            // Check for duplicate keys.
                            if !seen.insert(key.clone()) {
                                return Err(serde::de::Error::custom(format_args!(
                                    "duplicate field {}",
                                    key
                                )));
                            }

                            // Resolve the field value.
                            let value = match &entry.value {
                                BasicOrValue::Value(field_idx) => {
                                    let field = self.resolve(*field_idx);
                                    map.next_value_seed(field)?
                                }
                                BasicOrValue::Basic(basic) => {
                                    let field = DecodeValue {
                                        cfg: self.cfg,
                                        reg: self.reg,
                                        value: &Value::Basic(*basic),
                                    };
                                    map.next_value_seed(field)?
                                }
                            };

                            // Insert it into our map.
                            values.insert(key, value);
                        }
                        None => {
                            // Unknown field; ignore it.
                            map.next_value::<serde::de::IgnoredAny>()?;
                        }
                    }
                }

                // Report any missing fields.
                if seen.len() != fields.len() {
                    let missing = fields
                        .iter()
                        .filter_map(|(key, field)| {
                            if seen.contains(key) {
                                return None;
                            }

                            // If the field is optional, don't consider it missing.
                            if field.optional {
                                return None;
                            } else if let BasicOrValue::Value(idx) = &field.value {
                                if matches!(self.resolve(*idx).value, Value::Option(_)) {
                                    return None;
                                }
                            }

                            Some(key.as_str())
                        })
                        .collect::<Vec<_>>();

                    match missing.len() {
                        0 => {} // do nothing
                        1 => {
                            return Err(serde::de::Error::custom(format_args!(
                                "missing field {}",
                                missing[0]
                            )))
                        }
                        _ => {
                            return Err(serde::de::Error::custom(format_args!(
                                "missing fields {}",
                                FieldList { names: &missing }
                            )))
                        }
                    }
                }

                Ok(PValue::Object(values))
            }

            Value::Ref(idx) => recurse_ref!(self, idx, visit_map, map),
            Value::Option(bov) => recurse!(self, bov, visit_map, map),
            Value::Validation(v) => validate_pval!(self, v, visit_map, map),
            Value::Union(candidates) => {
                let mut values = serde_json::Map::new();
                while let Some((key, value)) = map.next_entry()? {
                    values.insert(key, value);
                }
                let map = JVal::Object(values);
                for c in candidates {
                    match c {
                        BasicOrValue::Basic(basic) => {
                            let basic_val = Value::Basic(*basic);
                            let visitor = DecodeValue {
                                cfg: self.cfg,
                                reg: self.reg,
                                value: &basic_val,
                            };
                            if visitor.validate::<A::Error>(&map).is_ok() {
                                return visitor.transform(map);
                            }
                        }
                        BasicOrValue::Value(idx) => {
                            let visitor = DecodeValue {
                                cfg: self.cfg,
                                reg: self.reg,
                                value: &self.reg.values[*idx],
                            };
                            if visitor.validate::<A::Error>(&map).is_ok() {
                                return visitor.transform(map);
                            }
                        }
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Map, &self))
            }
            _ => Err(serde::de::Error::invalid_type(Unexpected::Map, &self)),
        }
    }
}

fn visit_seq<'de, A>(elem: DecodeValue, mut seq: A) -> Result<PValue, A::Error>
where
    A: SeqAccess<'de>,
{
    let mut vec = Vec::new();
    // TODO optimize to stop using JSONValueVisitor and use serde_json's visitor directly?
    while let Some(elem) = seq.next_element_seed(elem)? {
        vec.push(elem);
    }
    Ok(PValue::Array(vec))
}

fn visit_map<'de, A>(elem: DecodeValue, mut map: A) -> Result<PValue, A::Error>
where
    A: MapAccess<'de>,
{
    let mut values = PValues::new();
    while let Some((key, value)) = map.next_entry_seed(PhantomData, elem)? {
        values.insert(key, value);
    }
    Ok(PValue::Object(values))
}

struct FieldList<'a> {
    names: &'a [&'a str],
}

impl DecodeValue<'_> {
    #[cfg_attr(
        feature = "rttrace",
        tracing::instrument(skip(self), ret, level = "trace")
    )]
    pub fn validate<E>(&self, value: &JVal) -> Result<(), E>
    where
        E: serde::de::Error,
    {
        match value {
            JVal::Null => match self.value {
                Value::Basic(Basic::Any | Basic::Null) => Ok(()),
                Value::Option(_) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Validation(v) => validate_jval!(self, v, validate, value),
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Option, self))
                }
                _ => Err(serde::de::Error::invalid_type(Unexpected::Option, self)),
            },
            JVal::Bool(bool) => match self.value {
                Value::Basic(Basic::Any | Basic::Bool) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Literal(lit) => match lit {
                    Literal::Bool(val) if *bool == *val => Ok(()),
                    _ => Err(serde::de::Error::custom(format_args!(
                        "expected {}, got {}",
                        lit.expecting(),
                        bool,
                    ))),
                },
                Value::Validation(v) => validate_jval!(self, v, validate, value),
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(
                        Unexpected::Bool(*bool),
                        self,
                    ))
                }
                _ => Err(serde::de::Error::invalid_type(
                    Unexpected::Bool(*bool),
                    self,
                )),
            },
            JVal::Number(num) => match self.value {
                Value::Basic(Basic::Any | Basic::Number) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Literal(lit) => match lit {
                    Literal::Int(val) if num.as_i64() == Some(*val) => Ok(()),
                    Literal::Float(val) if num.as_f64() == Some(*val) => Ok(()),
                    _ => Err(serde::de::Error::custom(format_args!(
                        "expected {}, got {}",
                        lit.expecting(),
                        num,
                    ))),
                },
                Value::Validation(v) => validate_jval!(self, v, validate, value),
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(
                        Unexpected::Other("number"),
                        self,
                    ))
                }
                _ => Err(serde::de::Error::invalid_type(
                    Unexpected::Other("number"),
                    self,
                )),
            },

            JVal::String(string) => match self.value {
                Value::Basic(Basic::Any | Basic::String) => Ok(()),
                Value::Basic(Basic::DateTime) => api::DateTime::parse_from_rfc3339(string)
                    .map(|_| ())
                    .map_err(|e| {
                        serde::de::Error::custom(format_args!("invalid datetime: {}", e,))
                    }),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Literal(lit) => match lit {
                    Literal::Str(val) if string.as_str() == *val => Ok(()),
                    _ => Err(serde::de::Error::custom(format_args!(
                        "expected {}, got {}",
                        lit.expecting(),
                        string,
                    ))),
                },
                Value::Validation(v) => validate_jval!(self, v, validate, value),
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(
                        Unexpected::Str(string),
                        self,
                    ))
                }
                _ => Err(serde::de::Error::invalid_type(
                    Unexpected::Str(string),
                    self,
                )),
            },

            JVal::Array(array) => match self.value {
                Value::Basic(Basic::Any) => Ok(()),
                Value::Array(bov) => match bov {
                    BasicOrValue::Basic(basic) => {
                        let basic_val = Value::Basic(*basic);
                        let visitor = DecodeValue {
                            cfg: self.cfg,
                            reg: self.reg,
                            value: &basic_val,
                        };
                        for elem in array {
                            visitor.validate(elem)?;
                        }
                        Ok(())
                    }
                    BasicOrValue::Value(idx) => {
                        let visitor = DecodeValue {
                            cfg: self.cfg,
                            reg: self.reg,
                            value: &self.reg.values[*idx],
                        };
                        for elem in array {
                            visitor.validate(elem)?;
                        }
                        Ok(())
                    }
                },
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(bov) => {
                    for elem in array {
                        recurse!(self, bov, validate, elem)?;
                    }
                    Ok(())
                }
                Value::Validation(v) => validate_jval!(self, v, validate, value),
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Seq, self))
                }
                _ => Err(serde::de::Error::invalid_type(Unexpected::Seq, self)),
            },

            JVal::Object(map) => match self.value {
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(bov) => recurse!(self, bov, validate, value),
                Value::Basic(Basic::Any) => Ok(()),

                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Map, self))
                }
                Value::Map(bov) => match bov {
                    BasicOrValue::Basic(basic) => {
                        let basic_val = Value::Basic(*basic);
                        let visitor = DecodeValue {
                            cfg: self.cfg,
                            reg: self.reg,
                            value: &basic_val,
                        };
                        for (_key, value) in map {
                            visitor.validate(value)?;
                        }
                        Ok(())
                    }
                    BasicOrValue::Value(idx) => {
                        let visitor = DecodeValue {
                            cfg: self.cfg,
                            reg: self.reg,
                            value: &self.reg.values[*idx],
                        };
                        for (_key, value) in map {
                            visitor.validate(value)?;
                        }
                        Ok(())
                    }
                },
                Value::Struct(Struct { fields }) => {
                    let mut seen = HashSet::new();
                    for (key, value) in map {
                        match fields.get(key.as_str()) {
                            Some(entry) => {
                                seen.insert(key.clone());

                                match &entry.value {
                                    BasicOrValue::Value(field_idx) => {
                                        let field = self.resolve(*field_idx);
                                        field.validate(value)?
                                    }
                                    BasicOrValue::Basic(basic) => {
                                        let field = DecodeValue {
                                            cfg: self.cfg,
                                            reg: self.reg,
                                            value: &Value::Basic(*basic),
                                        };
                                        field.validate(value)?
                                    }
                                }
                            }
                            None => {
                                // Unknown field; ignore it.
                            }
                        }
                    }

                    // Report any missing fields.
                    if seen.len() != fields.len() {
                        let missing = fields
                            .iter()
                            .filter_map(|(key, field)| {
                                if seen.contains(key) {
                                    return None;
                                }

                                if field.optional {
                                    return None;
                                } else if let BasicOrValue::Value(idx) = &field.value {
                                    if matches!(self.resolve(*idx).value, Value::Option(_)) {
                                        return None;
                                    }
                                }

                                Some(key.as_str())
                            })
                            .collect::<Vec<_>>();

                        match missing.len() {
                            0 => {} // do nothing
                            1 => {
                                return Err(serde::de::Error::custom(format_args!(
                                    "missing field {}",
                                    missing[0]
                                )))
                            }
                            _ => {
                                return Err(serde::de::Error::custom(format_args!(
                                    "missing fields {}",
                                    FieldList { names: &missing }
                                )))
                            }
                        }
                    }

                    Ok(())
                }
                Value::Validation(v) => validate_jval!(self, v, validate, value),

                _ => Err(serde::de::Error::invalid_type(Unexpected::Map, self)),
            },
        }
    }

    #[cfg_attr(
        feature = "rttrace",
        tracing::instrument(skip(self), ret, level = "trace")
    )]
    fn transform<E>(&self, value: JVal) -> Result<PValue, E>
    where
        E: serde::de::Error,
    {
        Ok(match value {
            JVal::Null => PValue::Null,
            JVal::Bool(val) => PValue::Bool(val),
            JVal::Number(num) => PValue::Number(num),
            JVal::Array(vals) => match self.value {
                Value::Ref(idx) => return recurse_ref!(self, idx, transform, JVal::Array(vals)),
                Value::Option(bov) => return recurse!(self, bov, transform, JVal::Array(vals)),
                Value::Validation(v) => {
                    return recurse!(self, &v.bov, transform, JVal::Array(vals))
                }
                Value::Basic(Basic::Any) => {
                    let mut new_vals = Vec::with_capacity(vals.len());
                    for val in vals {
                        new_vals.push(self.transform(val)?);
                    }
                    PValue::Array(new_vals)
                }
                Value::Array(bov) => {
                    let mut new_vals = Vec::with_capacity(vals.len());
                    for val in vals {
                        let val = recurse!(self, bov, transform, val)?;
                        new_vals.push(val);
                    }
                    PValue::Array(new_vals)
                }
                Value::Union(candidates) => {
                    let val = JVal::Array(vals);
                    for c in candidates {
                        match c {
                            BasicOrValue::Basic(basic) => {
                                let basic_val = Value::Basic(*basic);
                                let visitor = DecodeValue {
                                    cfg: self.cfg,
                                    reg: self.reg,
                                    value: &basic_val,
                                };
                                if visitor.validate::<E>(&val).is_ok() {
                                    return visitor.transform(val);
                                }
                            }
                            BasicOrValue::Value(idx) => {
                                let visitor = DecodeValue {
                                    cfg: self.cfg,
                                    reg: self.reg,
                                    value: &self.reg.values[*idx],
                                };
                                if visitor.validate::<E>(&val).is_ok() {
                                    return visitor.transform(val);
                                }
                            }
                        }
                    }
                    return Err(serde::de::Error::invalid_type(Unexpected::Seq, self));
                }
                Value::Basic(basic) => {
                    return Err(serde::de::Error::invalid_type(
                        Unexpected::Other(basic.expecting()),
                        self,
                    ))
                }
                Value::Literal(lit) => {
                    return Err(serde::de::Error::invalid_type(
                        Unexpected::Other(lit.expecting_type()),
                        self,
                    ))
                }
                Value::Map(_) | Value::Struct(_) => {
                    return Err(serde::de::Error::invalid_type(Unexpected::Map, self))
                }
            },
            JVal::Object(obj) => match self.value {
                Value::Ref(idx) => return recurse_ref!(self, idx, transform, JVal::Object(obj)),
                Value::Option(bov) => return recurse!(self, bov, transform, JVal::Object(obj)),
                Value::Validation(v) => {
                    return recurse!(self, &v.bov, transform, JVal::Object(obj))
                }
                Value::Basic(Basic::Any) => {
                    let mut new_obj = BTreeMap::new();
                    for (key, val) in obj {
                        new_obj.insert(key, self.transform(val)?);
                    }
                    PValue::Object(new_obj)
                }
                Value::Map(bov) => {
                    let mut new_obj = BTreeMap::new();
                    for (key, val) in obj {
                        let val = recurse!(self, bov, transform, val)?;
                        new_obj.insert(key, val);
                    }
                    PValue::Object(new_obj)
                }
                Value::Struct(Struct { fields }) => {
                    let mut new_obj = BTreeMap::new();
                    for (key, value) in obj {
                        match fields.get(key.as_str()) {
                            Some(entry) => {
                                let val = recurse!(self, &entry.value, transform, value)?;
                                new_obj.insert(key, val);
                            }
                            None => {
                                // Unknown field; ignore it.
                            }
                        }
                    }
                    PValue::Object(new_obj)
                }
                Value::Union(candidates) => {
                    let val = JVal::Object(obj);
                    for c in candidates {
                        match c {
                            BasicOrValue::Basic(basic) => {
                                let basic_val = Value::Basic(*basic);
                                let visitor = DecodeValue {
                                    cfg: self.cfg,
                                    reg: self.reg,
                                    value: &basic_val,
                                };
                                if visitor.validate::<E>(&val).is_ok() {
                                    return visitor.transform(val);
                                }
                            }
                            BasicOrValue::Value(idx) => {
                                let visitor = DecodeValue {
                                    cfg: self.cfg,
                                    reg: self.reg,
                                    value: &self.reg.values[*idx],
                                };
                                if visitor.validate::<E>(&val).is_ok() {
                                    return visitor.transform(val);
                                }
                            }
                        }
                    }

                    return Err(serde::de::Error::invalid_type(Unexpected::Map, self));
                }
                Value::Basic(basic) => {
                    return Err(serde::de::Error::invalid_type(
                        Unexpected::Other(basic.expecting()),
                        self,
                    ))
                }
                Value::Literal(lit) => {
                    return Err(serde::de::Error::invalid_type(
                        Unexpected::Other(lit.expecting_type()),
                        self,
                    ))
                }
                Value::Array(_) => {
                    return Err(serde::de::Error::invalid_type(Unexpected::Seq, self))
                }
            },
            JVal::String(str) => match self.value {
                Value::Ref(idx) => return recurse_ref!(self, idx, transform, JVal::String(str)),
                Value::Option(bov) => return recurse!(self, bov, transform, JVal::String(str)),
                Value::Validation(v) => {
                    return recurse!(self, &v.bov, transform, JVal::String(str))
                }
                Value::Basic(Basic::DateTime) => api::DateTime::parse_from_rfc3339(&str)
                    .map(PValue::DateTime)
                    .map_err(|e| {
                        serde::de::Error::custom(format_args!("invalid datetime: {}", e,))
                    })?,

                // Any non-datetime basic value gets transformed into a string.
                Value::Basic(_) => PValue::String(str),

                Value::Literal(_) => PValue::String(str),
                Value::Union(types) => {
                    let val = JVal::String(str);
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, transform, val.clone());
                        if res.is_ok() {
                            return res;
                        }
                    }
                    return Err(serde::de::Error::invalid_type(
                        Unexpected::Other("string"),
                        self,
                    ));
                }
                Value::Map(_) | Value::Struct(_) | Value::Array(_) => {
                    return Err(serde::de::Error::invalid_type(Unexpected::Str(&str), self))
                }
            },
        })
    }
}

impl Display for FieldList<'_> {
    fn fmt(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        match self.names.len() {
            0 => panic!(), // special case elsewhere
            1 => write!(formatter, "`{}`", self.names[0]),
            2 => write!(formatter, "`{}` and `{}`", self.names[0], self.names[1]),
            _ => {
                for (i, alt) in self.names.iter().enumerate() {
                    if i > 0 {
                        write!(formatter, ", ")?;
                    }
                    if i == self.names.len() - 1 {
                        write!(formatter, "and ")?;
                    }
                    write!(formatter, "`{}`", alt)?;
                }
                Ok(())
            }
        }
    }
}
