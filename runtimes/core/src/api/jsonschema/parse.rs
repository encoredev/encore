use crate::api;
use crate::api::jsonschema::{Basic, BasicOrValue, JSONSchema, Registry, Struct, Value};
use crate::api::{schema, APIResult};
use schema::{AsStr, ToHeaderStr};
use std::collections::HashMap;
use std::str::FromStr;

use serde_json::Value as JSON;

pub trait ParseWithSchema<Output> {
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<Output>;
}

impl<H> ParseWithSchema<serde_json::Map<String, JSON>> for H
where
    H: schema::HTTPHeaders,
{
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<serde_json::Map<String, JSON>> {
        let mut result = serde_json::Map::new();
        let reg = schema.registry.as_ref();

        for (field_key, field) in schema.root().fields.iter() {
            let header_name = field.name_override.as_deref().unwrap_or(field_key.as_str());
            let mut values = self.get_all(header_name);
            let Some(header_value) = values.next() else {
                if field.optional {
                    continue;
                } else {
                    return Err(api::Error {
                        code: api::ErrCode::InvalidArgument,
                        message: format!("missing required header: {}", header_name),
                        internal_message: None,
                        stack: None,
                    });
                }
            };

            result.insert(
                field_key.clone(),
                match &field.value {
                    BasicOrValue::Basic(basic) => {
                        let basic = Value::Basic(*basic);
                        let value = parse_header_value(header_value, reg, &basic)?;
                        value
                    }
                    BasicOrValue::Value(idx) => {
                        // Determine the type of the value(s).
                        let mut basic_val = Value::Basic(Basic::Null); // for borrowing below
                        let (value_type, is_array) = match reg.get(*idx) {
                            Value::Array(bov) => (
                                match bov {
                                    BasicOrValue::Value(idx) => reg.get(*idx),
                                    BasicOrValue::Basic(basic) => {
                                        basic_val = Value::Basic(*basic);
                                        &basic_val
                                    }
                                },
                                true,
                            ),
                            val => (val, false),
                        };

                        if is_array {
                            let mut values = std::iter::once(header_value).chain(values);
                            let mut arr = Vec::new();
                            for header_value in values {
                                let value = parse_header_value(header_value, reg, value_type)?;
                                arr.push(value);
                            }
                            serde_json::Value::Array(arr)
                        } else {
                            let value = parse_header_value(header_value, reg, value_type)?;
                            value
                        }
                    }
                },
            );
        }

        Ok(result)
    }

    // fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<serde_json::Map<String, JSON>> {
    //     // The number of times we've seen a field that's ambiguous whether
    //     // it's an array or not. Used to ensure we don't nest arrays incorrectly.
    //     let mut maybe_array_counters = HashMap::new();
    //
    //     let reg = schema.registry.as_ref();
    //     let fields = &schema.root().fields;
    //     let mut result = serde_json::Map::new();
    //
    //     for (key, header_value) in self.headers() {
    //         let key = key.as_str();
    //         enum Kind {
    //             NotArray,
    //             IsArray,
    //             MaybeArray,
    //         }
    //
    //         let (decoded, kind) = match fields.get(key) {
    //             None => {
    //                 // Not known to schema; pass it unmodified.
    //                 // TODO ignore?
    //                 let str = header_value.to_str().map_err(|err| api::Error {
    //                     code: api::ErrCode::InvalidArgument,
    //                     message: "invalid header value".to_string(),
    //                     internal_message: Some(format!("invalid header value: {}", err)),
    //                     stack: None,
    //                 })?;
    //                 (JSON::String(str.to_string()), Kind::MaybeArray)
    //             }
    //             Some(field) => {
    //                 match &field.value {
    //                     BasicOrValue::Basic(basic) => {
    //                         let basic = Value::Basic(*basic);
    //                         let value = parse_header_value(header_value, reg, &basic)?;
    //                         (value, Kind::NotArray)
    //                     }
    //                     BasicOrValue::Value(idx) => {
    //                         let value_type = reg.get(*idx);
    //                         let value = parse_header_value(header_value, reg, value_type)?;
    //
    //                         // Is this field an array?
    //                         let kind = match value_type {
    //                             Value::Array(_) => Kind::IsArray,
    //                             Value::Option(BasicOrValue::Value(idx)) => {
    //                                 if reg.get(*idx).is_array() {
    //                                     Kind::IsArray
    //                                 } else {
    //                                     Kind::NotArray
    //                                 }
    //                             }
    //                             _ => Kind::NotArray,
    //                         };
    //                         (value, kind)
    //                     }
    //                 }
    //             }
    //         };
    //
    //         let key = key.to_string();
    //         match kind {
    //             Kind::IsArray => {
    //                 // The schema is an array field. Add the value to the array.
    //                 let arr = result
    //                     .entry(key)
    //                     .or_insert_with(|| serde_json::Value::Array(Vec::new()));
    //                 if let JSON::Array(arr) = arr {
    //                     arr.push(decoded);
    //                 }
    //             }
    //             Kind::NotArray => {
    //                 // Not an array; insert only if it doesn't exist.
    //                 result.entry(key).or_insert(decoded);
    //             }
    //
    //             Kind::MaybeArray => {
    //                 // Update the counter for this field.
    //                 let counter = maybe_array_counters
    //                     .entry(key.clone())
    //                     .and_modify(|count| *count += 1)
    //                     .or_insert(1);
    //                 let counter = *counter;
    //
    //                 if counter == 1 {
    //                     // First time seeing this field. Insert it.
    //                     result.insert(key, decoded);
    //                 } else {
    //                     // Second time seeing this field. Promote it to an array.
    //                     let existing = result.remove(&key).unwrap();
    //                     let updated = match existing {
    //                         JSON::Array(mut arr) => {
    //                             arr.push(decoded);
    //                             JSON::Array(arr)
    //                         }
    //                         _ => JSON::Array(vec![existing, decoded]),
    //                     };
    //                     result.insert(key, updated);
    //                 }
    //             }
    //         }
    //     }
    //
    //     Ok(result)
    // }
}

fn parse_header_value<V>(header: V, reg: &Registry, schema: &Value) -> APIResult<JSON>
where
    V: ToHeaderStr,
{
    let str = header.to_str().map_err(|err| api::Error {
        code: api::ErrCode::InvalidArgument,
        message: "invalid header value".to_string(),
        internal_message: Some(format!("invalid header value: {}", err)),
        stack: None,
    })?;

    match schema {
        // Recurse
        Value::Ref(idx) => parse_header_value(header, reg, &reg.values[*idx]),

        // If we have an empty header for an option, that's fine.
        Value::Option(_) if header.is_empty() => Ok(JSON::Null),

        // Otherwise recurse.
        Value::Option(opt) => match opt {
            BasicOrValue::Basic(basic) => parse_basic_str(basic, str),
            BasicOrValue::Value(idx) => parse_header_value(header, reg, &reg.values[*idx]),
        },

        Value::Basic(basic) => parse_basic_str(basic, str),

        Value::Struct { .. } | Value::Map(_) | Value::Array(_) => unsupported(reg, schema),
    }
}

impl ParseWithSchema<serde_json::Value> for serde_json::Value {
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<serde_json::Value> {
        let reg = schema.registry.as_ref();
        let fields = &schema.root().fields;
        match self {
            JSON::Object(obj) => {
                let mut result = serde_json::Map::new();
                for (key, value) in obj {
                    let value = match fields.get(&key) {
                        // Not known to schema; pass it unmodified.
                        None => value,

                        Some(field) => match &field.value {
                            BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, value)?,

                            BasicOrValue::Value(idx) => {
                                parse_json_value(value, reg, &reg.values[*idx])?
                            }
                        },
                    };
                    result.insert(key, value);
                }
                Ok(result.into())
            }

            _ => unexpected_json(reg, schema.root_value(), &self),
        }
    }
}

fn parse_json_value(
    this: serde_json::Value,
    reg: &Registry,
    schema: &Value,
) -> APIResult<serde_json::Value> {
    match schema {
        // Recurse
        Value::Ref(idx) => parse_json_value(this, reg, &reg.values[*idx]),

        // If we have a null value for an option, that's fine.
        Value::Option(_) if this.is_null() => Ok(JSON::Null),

        // Otherwise recurse.
        Value::Option(opt) => match opt {
            BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, this),
            BasicOrValue::Value(idx) => parse_json_value(this, reg, &reg.values[*idx]),
        },

        Value::Basic(basic) => parse_basic_json(reg, basic, this),

        Value::Struct(Struct { fields }) => match this {
            JSON::Object(obj) => {
                let mut result = serde_json::Map::new();
                for (key, value) in obj {
                    let value = match fields.get(&key) {
                        // Not known to schema; pass it unmodified.
                        None => value,

                        Some(field) => match &field.value {
                            BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, value)?,

                            BasicOrValue::Value(idx) => {
                                parse_json_value(value, reg, &reg.values[*idx])?
                            }
                        },
                    };
                    result.insert(key, value);
                }
                Ok(result.into())
            }

            _ => unexpected_json(reg, schema, &this),
        },

        Value::Map(value_type) => match this {
            JSON::Object(obj) => {
                let mut result = serde_json::Map::new();
                for (key, value) in obj {
                    let value = match value_type {
                        BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, value)?,
                        BasicOrValue::Value(idx) => {
                            parse_json_value(value, reg, &reg.values[*idx])?
                        }
                    };
                    result.insert(key, value);
                }
                Ok(result.into())
            }

            _ => unexpected_json(reg, schema, &this),
        },

        Value::Array(value_type) => match this {
            JSON::Array(arr) => {
                let mut result = Vec::with_capacity(arr.len());
                for val in arr {
                    let value = match value_type {
                        BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, val)?,
                        BasicOrValue::Value(idx) => parse_json_value(val, reg, &reg.values[*idx])?,
                    };
                    result.push(value);
                }
                Ok(result.into())
            }

            _ => unexpected_json(reg, schema, &this),
        },
    }
}

fn unexpected_json(reg: &Registry, schema: &Value, value: &JSON) -> APIResult<JSON> {
    Err(api::Error {
        code: api::ErrCode::InvalidArgument,
        message: "invalid value".to_string(),
        internal_message: Some(format!(
            "expected {}, got {}",
            schema.expecting(reg),
            describe_json(value),
        )),
        stack: None,
    })
}

fn unexpected_str(reg: &Registry, schema: &Value, value: &str) -> APIResult<JSON> {
    Err(api::Error {
        code: api::ErrCode::InvalidArgument,
        message: "invalid value".to_string(),
        internal_message: Some(format!("expected {}, got {}", schema.expecting(reg), value,)),
        stack: None,
    })
}

fn unsupported<T>(reg: &Registry, schema: &Value) -> APIResult<T> {
    Err(api::Error {
        code: api::ErrCode::InvalidArgument,
        message: "unsupported schema type".to_string(),
        internal_message: Some(format!(
            "got an unsupported schema type: {}",
            schema.expecting(reg),
        )),
        stack: None,
    })
}

fn describe_json(value: &serde_json::Value) -> &'static str {
    match value {
        JSON::Null => "null",
        JSON::Bool(_) => "a boolean",
        JSON::Number(_) => "a number",
        JSON::String(_) => "a string",
        JSON::Array(_) => "an array",
        JSON::Object(_) => "an object",
    }
}

fn parse_basic_json(
    reg: &Registry,
    basic: &Basic,
    value: serde_json::Value,
) -> APIResult<serde_json::Value> {
    match (basic, &value) {
        (Basic::Any, _) => Ok(value),

        (Basic::Null, JSON::Null) => Ok(value),
        (Basic::Bool, JSON::Bool(_)) => Ok(value),
        (Basic::Number, JSON::Number(_)) => Ok(value),
        (Basic::String, JSON::String(_)) => Ok(value),

        (Basic::String, JSON::Number(num)) => Ok(JSON::String(num.to_string())),
        (Basic::String, JSON::Bool(bool)) => Ok(JSON::String(bool.to_string())),

        (_, JSON::String(str)) => match basic {
            Basic::Bool => match str.as_str() {
                "true" => Ok(JSON::Bool(true)),
                "false" => Ok(JSON::Bool(false)),
                _ => Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("invalid boolean value: {}", str),
                    internal_message: None,
                    stack: None,
                }),
            },
            Basic::Number => serde_json::Number::from_str(str)
                .map(|num| serde_json::Value::Number(num))
                .map_err(|_err| api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("invalid number value: {}", str),
                    internal_message: None,
                    stack: None,
                }),
            Basic::Null if str == "null" => Ok(JSON::Null),

            _ => unexpected_json(reg, &Value::Basic(*basic), &value),
        },

        _ => unexpected_json(reg, &Value::Basic(*basic), &value),
    }
}

fn parse_basic_str(basic: &Basic, str: &str) -> APIResult<serde_json::Value> {
    match basic {
        Basic::Any | Basic::String => Ok(JSON::String(str.to_string())),

        Basic::Null if str == "" || str == "null" => Ok(JSON::Null),

        Basic::Bool => match str {
            "true" => Ok(JSON::Bool(true)),
            "false" => Ok(JSON::Bool(false)),
            _ => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!("invalid boolean value: {}", str),
                internal_message: None,
                stack: None,
            }),
        },

        Basic::Number => serde_json::Number::from_str(str)
            .map(|num| JSON::Number(num))
            .map_err(|_err| api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!("invalid number value: {}", str),
                internal_message: None,
                stack: None,
            }),

        _ => Err(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "invalid value".to_string(),
            internal_message: Some(format!("expected {}, got {:#?}", basic.expecting(), str)),
            stack: None,
        }),
    }
}

// impl<T, O> ParseWithSchema<O> for &T
// where
//     T: ParseWithSchema<O> + Clone,
// {
//     fn parse_with_schema(self, reg: &Registry, schema: &Value) -> APIResult<O> {
//         self.clone().parse_with_schema(reg, schema)
//     }
// }
