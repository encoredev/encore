use std::collections::{HashMap, HashSet};
use std::fmt;
use std::fmt::Display;
use std::marker::PhantomData;

use serde::de::{DeserializeSeed, MapAccess, SeqAccess, Unexpected, Visitor};
use serde::{Deserialize, Deserializer};

use crate::api::jsonschema::Registry;

#[derive(Debug, Clone)]
pub enum Value {
    // A JSON primitive (e.g. string, number, boolean, null).
    Basic(Basic),

    /// Consume a value if present.
    Option(BasicOrValue),

    /// Consume a map of key-value pairs (the keys are always strings in JSON).
    Map(BasicOrValue),

    /// A struct with a set of known fields.
    Struct(Struct),

    /// Consume an array of values.
    Array(BasicOrValue),

    /// Consume an array of values.
    Union(Vec<BasicOrValue>),

    // Reference to another value.
    Ref(usize),
}

#[derive(Debug, Clone)]
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
            .find(|(field_name, field)| {
                field.name_override.as_deref().unwrap_or(field_name) == name
            })
            .is_some()
    }
}

#[derive(Debug, Clone)]
pub struct Field {
    pub value: BasicOrValue,
    pub optional: bool,
    pub name_override: Option<String>,
}

impl Default for Struct {
    fn default() -> Self {
        Struct {
            fields: HashMap::new(),
        }
    }
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

    pub fn expecting(&self, reg: &Registry) -> &'static str {
        match self {
            Value::Array(_) => "a JSON array",
            Value::Basic(basic) => basic.expecting(),
            Value::Map(_) => "a JSON object",
            Value::Union(_) => "one of various JSON values",
            Value::Option(bov) => bov.expecting(reg),
            Value::Ref(idx) => reg.get(*idx).expecting(reg),
            Value::Struct { .. } => "a JSON object",
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
}

impl Basic {
    pub fn expecting(&self) -> &'static str {
        match self {
            Basic::Any => "any valid JSON value",
            Basic::Null => "null",
            Basic::Bool => "a boolean",
            Basic::Number => "a number",
            Basic::String => "a string",
        }
    }
}

impl<'de> DeserializeSeed<'de> for Basic {
    type Value = serde_json::Value;

    fn deserialize<D>(self, deserializer: D) -> Result<Self::Value, D::Error>
    where
        D: Deserializer<'de>,
    {
        let reg = Registry { values: vec![] };
        let visitor = DecodeValue {
            reg: &reg,
            value: &Value::Basic(self),
        };
        visitor.deserialize(deserializer)
    }
}

#[derive(Debug, Copy, Clone)]
pub enum BasicOrValue {
    Basic(Basic),
    Value(usize),
}

impl BasicOrValue {
    pub fn expecting(&self, reg: &Registry) -> &'static str {
        match self {
            BasicOrValue::Basic(basic) => basic.expecting(),
            BasicOrValue::Value(idx) => reg.get(*idx).expecting(reg),
        }
    }
}

#[derive(Copy, Clone)]
pub(super) struct DecodeValue<'a> {
    pub(super) reg: &'a Registry,
    pub(super) value: &'a Value,
}

impl<'a> DecodeValue<'a> {
    fn resolve(&'a self, idx: usize) -> DecodeValue<'a> {
        DecodeValue {
            reg: self.reg,
            value: &self.reg.values[idx],
        }
    }
}

impl<'de: 'a, 'a> DeserializeSeed<'de> for DecodeValue<'a> {
    type Value = serde_json::Value;

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
            reg: &$self.reg,
            value: &$self.reg.values[*$idx],
        };
        visitor.$method($value)
    }};
}

macro_rules! recurse_ref0 {
    ($self:ident, $idx:expr, $method:ident) => {{
        let visitor = DecodeValue {
            reg: &$self.reg,
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
                    reg: &$self.reg,
                    value: &basic_val,
                };
                visitor.$method($value)
            }
            BasicOrValue::Value(idx) => {
                let visitor = DecodeValue {
                    reg: &$self.reg,
                    value: &$self.reg.values[*idx],
                };
                visitor.$method($value)
            }
        }
    }};
}

macro_rules! recurse0 {
    ($self:ident, $bov:expr, $method:ident) => {{
        match $bov {
            BasicOrValue::Basic(basic) => {
                let basic_val = Value::Basic(*basic);
                let visitor = DecodeValue {
                    reg: &$self.reg,
                    value: &basic_val,
                };
                visitor.$method()
            }
            BasicOrValue::Value(idx) => {
                let visitor = DecodeValue {
                    reg: &$self.reg,
                    value: &$self.reg.values[*idx],
                };
                visitor.$method()
            }
        }
    }};
}

impl<'de, 'a> Visitor<'de> for DecodeValue<'a> {
    type Value = serde_json::Value;

    fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
        formatter.write_str(match self.value {
            Value::Basic(b) => match b {
                Basic::Any => "any valid JSON value",
                Basic::Null => "null",
                Basic::Bool => "a boolean",
                Basic::Number => "a number",
                Basic::String => "a string",
            },
            Value::Map(_) => "a JSON object",
            Value::Array(_) => "a JSON array",
            Value::Union(_) => "one of various JSON values",
            Value::Option(_) => "any valid JSON value or null",
            Value::Struct { .. } => "a JSON object",
            Value::Ref(_) => "a JSON value",
        })
    }

    #[inline]
    fn visit_bool<E>(self, value: bool) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match &self.value {
            Value::Basic(Basic::Any | Basic::Bool) => Ok(serde_json::Value::Bool(value)),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_bool, value),
            Value::Option(val) => {
                recurse!(self, val, visit_bool, value)
            }
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_bool, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Bool(value), &self))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Bool(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_i64<E>(self, value: i64) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => Ok(serde_json::Value::Number(value.into())),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_i64, value),
            Value::Option(val) => {
                recurse!(self, val, visit_i64, value)
            }
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_i64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Signed(value), &self))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Signed(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_u64<E>(self, value: u64) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => Ok(serde_json::Value::Number(value.into())),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_u64, value),
            Value::Option(val) => {
                recurse!(self, val, visit_u64, value)
            }
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_u64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Unsigned(value), &self))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Unsigned(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_f64<E>(self, value: f64) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Number) => Ok(serde_json::Number::from_f64(value)
                .map_or(serde_json::Value::Null, serde_json::Value::Number)),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_f64, value),
            Value::Option(bov) => {
                recurse!(self, bov, visit_f64, value)
            }
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_f64, value);
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Float(value), &self))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Float(value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_str<E>(self, value: &str) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        self.visit_string(String::from(value))
    }

    #[inline]
    fn visit_string<E>(self, value: String) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::String) => Ok(serde_json::Value::String(value)),
            Value::Ref(idx) => recurse_ref!(self, idx, visit_string, value),
            Value::Option(bov) => {
                recurse!(self, bov, visit_string, value)
            }
            Value::Union(types) => {
                for typ in types {
                    let res: Result<_, E> = recurse!(self, typ, visit_string, value.clone());
                    if let Ok(val) = res {
                        return Ok(val);
                    }
                }
                Err(serde::de::Error::invalid_type(Unexpected::Str(&value), &self))
            }
            _ => Err(serde::de::Error::invalid_type(
                Unexpected::Str(&value),
                &self,
            )),
        }
    }

    #[inline]
    fn visit_none<E>(self) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        match self.value {
            Value::Basic(Basic::Any | Basic::Null) | Value::Option(_) => {
                Ok(serde_json::Value::Null)
            }
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
    fn visit_some<D>(self, deserializer: D) -> Result<serde_json::Value, D::Error>
    where
        D: Deserializer<'de>,
    {
        Deserialize::deserialize(deserializer)
    }

    #[inline]
    fn visit_unit<E>(self) -> Result<serde_json::Value, E>
    where
        E: serde::de::Error,
    {
        self.visit_none()
    }

    #[inline]
    fn visit_seq<A>(self, mut seq: A) -> Result<serde_json::Value, A::Error>
    where
        A: SeqAccess<'de>,
    {
        match &self.value {
            Value::Basic(Basic::Any) => visit_seq(self, seq),
            Value::Array(bov) => match bov {
                BasicOrValue::Basic(basic) => {
                    let basic_val = Value::Basic(*basic);
                    let visitor = DecodeValue {
                        reg: &self.reg,
                        value: &basic_val,
                    };
                    visit_seq(visitor, seq)
                }
                BasicOrValue::Value(idx) => {
                    let visitor = DecodeValue {
                        reg: &self.reg,
                        value: &self.reg.values[*idx],
                    };
                    visit_seq(visitor, seq)
                }
            },
            Value::Ref(idx) => recurse_ref!(self, idx, visit_seq, seq),
            Value::Option(bov) => recurse!(self, bov, visit_seq, seq),
            Value::Union(_) => {
                let mut vec: Vec<serde_json::Value> = Vec::new();
                while let Some(elem) = seq.next_element()? {
                    vec.push(elem);
                }
                let arr = serde_json::Value::Array(vec);
                self.validate(&arr)?;
                Ok(arr)
            }
            _ => return Err(serde::de::Error::invalid_type(Unexpected::Seq, &self)),
        }
    }

    fn visit_map<A>(self, mut map: A) -> Result<serde_json::Value, A::Error>
    where
        A: MapAccess<'de>,
    {
        match &self.value {
            Value::Basic(Basic::Any) => visit_map(self, map),
            Value::Map(bov) => match bov {
                BasicOrValue::Basic(basic) => {
                    let basic_val = Value::Basic(*basic);
                    let visitor = DecodeValue {
                        reg: &self.reg,
                        value: &basic_val,
                    };
                    visit_map(visitor, map)
                }
                BasicOrValue::Value(idx) => {
                    let visitor = DecodeValue {
                        reg: &self.reg,
                        value: &self.reg.values[*idx],
                    };
                    visit_map(visitor, map)
                }
            },

            Value::Struct(Struct { fields }) => {
                let mut values = serde_json::Map::new();
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

                Ok(serde_json::Value::Object(values))
            }

            Value::Ref(idx) => recurse_ref!(self, idx, visit_map, map),
            Value::Option(bov) => recurse!(self, bov, visit_map, map),
            Value::Union(_) => {
                let mut values = serde_json::Map::new();
                while let Some((key, value)) = map.next_entry()? {
                    values.insert(key, value);
                }
                let map = serde_json::Value::Object(values);
                self.validate(&map)?;
                Ok(map)
            }
            _ => return Err(serde::de::Error::invalid_type(Unexpected::Map, &self)),
        }
    }
}

fn visit_seq<'de, A>(elem: DecodeValue, mut seq: A) -> Result<serde_json::Value, A::Error>
where
    A: SeqAccess<'de>,
{
    let mut vec = Vec::new();
    // TODO optimize to stop using JSONValueVisitor and use serde_json's visitor directly?
    while let Some(elem) = seq.next_element_seed(elem)? {
        vec.push(elem);
    }
    Ok(serde_json::Value::Array(vec))
}

fn visit_map<'de, A>(elem: DecodeValue, mut map: A) -> Result<serde_json::Value, A::Error>
where
    A: MapAccess<'de>,
{
    let mut values = serde_json::Map::new();
    while let Some((key, value)) = map.next_entry_seed(PhantomData, elem)? {
        values.insert(key, value);
    }
    Ok(serde_json::Value::Object(values))
}

struct FieldList<'a> {
    names: &'a [&'a str],
}

impl<'a> DecodeValue<'a> {
    fn validate<E>(&self, value: &serde_json::Value) -> Result<(), E>
        where E: serde::de::Error
    {
        match value {
            serde_json::Value::Null => match self.value {
                Value::Basic(Basic::Any | Basic::Null) => Ok(()),
                Value::Option(_) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
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
            serde_json::Value::Bool(bool) => match self.value {
                Value::Basic(Basic::Any | Basic::Bool) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Bool(*bool), self))
                }
                _ => Err(serde::de::Error::invalid_type(Unexpected::Bool(*bool), self)),
            },
            serde_json::Value::Number(num) => match self.value {
                Value::Basic(Basic::Any | Basic::Number) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Other("number"), self))
                }
                _ => Err(serde::de::Error::invalid_type(Unexpected::Other("number"), self)),
            },

            serde_json::Value::String(string) => match self.value {
                Value::Basic(Basic::Any | Basic::String) => Ok(()),
                Value::Ref(idx) => recurse_ref!(self, idx, validate, value),
                Value::Option(val) => {
                    recurse!(self, val, validate, value)
                }
                Value::Union(types) => {
                    for typ in types {
                        let res: Result<_, E> = recurse!(self, typ, validate, value);
                        if res.is_ok() {
                            return res;
                        }
                    }
                    Err(serde::de::Error::invalid_type(Unexpected::Str(string), self))
                }
                _ => Err(serde::de::Error::invalid_type(Unexpected::Str(string), self)),
            },

            serde_json::Value::Array(array) => match self.value {
                Value::Basic(Basic::Any) => Ok(()),
                Value::Array(bov) => match bov {
                    BasicOrValue::Basic(basic) => {
                        let basic_val = Value::Basic(*basic);
                        let visitor = DecodeValue {
                            reg: &self.reg,
                            value: &basic_val,
                        };
                        for elem in array {
                            visitor.validate(elem)?;
                        }
                        Ok(())
                    }
                    BasicOrValue::Value(idx) => {
                        let visitor = DecodeValue {
                            reg: &self.reg,
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

            serde_json::Value::Object(map) => match self.value {
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
                            reg: &self.reg,
                            value: &basic_val,
                        };
                        for (_key, value) in map {
                            visitor.validate(value)?;
                        }
                        Ok(())
                    }
                    BasicOrValue::Value(idx) => {
                        let visitor = DecodeValue {
                            reg: &self.reg,
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

                _ => return Err(serde::de::Error::invalid_type(Unexpected::Map, self)),
            }
        }
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
