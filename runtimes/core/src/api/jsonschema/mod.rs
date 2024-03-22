use std::fmt;
use std::sync::Arc;

use serde::de::{DeserializeSeed, Deserializer};

pub use de::{Basic, BasicOrValue, Field, Struct, Value};

use crate::api::jsonschema::de::DecodeValue;

mod de;
mod meta;
mod parse;
mod ser;

use crate::api::jsonschema::parse::ParseWithSchema;
use crate::api::APIResult;
pub use meta::Builder;

#[derive(Clone)]
pub struct JSONSchema {
    registry: Arc<Registry>,
    root: usize,
}

pub struct Registry {
    /// Vector of allocated values.
    values: Vec<Value>,
}

impl Registry {
    pub fn get(&self, mut idx: usize) -> &Value {
        loop {
            match &self.values[idx] {
                Value::Ref(i) => idx = *i,
                other => return other,
            }
        }
    }
}

impl fmt::Debug for Registry {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Don't render the list of values since it's too large.
        f.debug_struct("Registry").finish()
    }
}

impl JSONSchema {
    pub fn root_value(&self) -> &Value {
        &self.registry.values[self.root]
    }

    #[inline]
    pub fn root(&self) -> &Struct {
        let Value::Struct(str) = &self.registry.values[self.root] else {
            panic!("root is not a struct");
        };
        str
    }

    pub fn parse<P, O>(&self, payload: P) -> APIResult<O>
    where
        P: ParseWithSchema<O>,
        O: Sized,
    {
        payload.parse_with_schema(&self)
    }
}

impl fmt::Debug for JSONSchema {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.registry.values.get(self.root) {
            Some(ref v) => v.write_debug(&self.registry, f),
            None => write!(f, "Ref({})", self.root),
        }
    }
}

impl Value {
    fn write_debug(&self, reg: &Registry, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Value::Basic(b) => write!(f, "{:?}", b),
            Value::Struct(Struct { fields }) => {
                f.debug_struct("Struct").field("fields", &fields).finish()
            }
            Value::Option(v) => f.debug_struct("Option").field("value", &v).finish(),
            Value::Array(v) => f.debug_struct("Array").field("value", &v).finish(),
            Value::Map(v) => f.debug_struct("Map").field("value", &v).finish(),
            Value::Ref(idx) => match reg.values.get(*idx) {
                Some(ref v) => v.write_debug(reg, f),
                None => write!(f, "Ref({})", idx),
            },
        }
    }
}

impl<'de> DeserializeSeed<'de> for &JSONSchema {
    type Value = serde_json::Map<String, serde_json::Value>;

    fn deserialize<D>(self, deserializer: D) -> Result<Self::Value, D::Error>
    where
        D: Deserializer<'de>,
    {
        let visitor = DecodeValue {
            reg: &self.registry,
            value: &self.registry.values[self.root],
        };
        let value = deserializer.deserialize_any(visitor)?;
        match value {
            serde_json::Value::Object(map) => Ok(map),
            _ => Err(serde::de::Error::custom("expected object")),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::jsonschema::de::*;
    use std::collections::HashMap;

    #[test]
    fn test() {
        let reg = Arc::new(Registry {
            values: vec![
                Value::Struct(Struct {
                    fields: {
                        let mut fields = HashMap::new();
                        fields.insert(
                            "bar".to_string(),
                            Field {
                                value: BasicOrValue::Value(1),
                                optional: false,
                                name_override: None,
                            },
                        );
                        fields
                    },
                }),
                Value::Option(BasicOrValue::Value(2)),
                Value::Basic(Basic::Any),
            ],
        });

        let schema = JSONSchema {
            registry: reg.clone(),
            root: 0,
        };

        let str = r#"{"foo": "bar", "blah": "baz"}"#;
        let mut jsonde = serde_json::Deserializer::from_str(str);
        let res = serde::de::DeserializeSeed::deserialize(&schema, &mut jsonde);
        println!("{:?}", res);
    }
}
