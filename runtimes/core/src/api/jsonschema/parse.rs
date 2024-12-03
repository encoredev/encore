use crate::api::jsonschema::{Basic, BasicOrValue, JSONSchema, Registry, Struct, Value};
use crate::api::{self, PValue, PValues};
use crate::api::{schema, APIResult};
use schema::ToHeaderStr;

use std::str::FromStr;

use crate::api::jsonschema::de::Literal;

pub trait ParseWithSchema<Output> {
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<Output>;
}

macro_rules! header_to_str {
    ($header_value:expr) => {
        $header_value.to_str().map_err(|err| api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "invalid header value".to_string(),
            internal_message: Some(format!("invalid header value: {}", err)),
            stack: None,
            details: None,
        })
    };
}

impl<H> ParseWithSchema<PValues> for H
where
    H: schema::HTTPHeaders,
{
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<PValues> {
        let mut result = PValues::new();
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
                        details: None,
                    });
                }
            };

            result.insert(
                field_key.clone(),
                match &field.value {
                    BasicOrValue::Basic(basic) => {
                        let basic = Value::Basic(*basic);

                        parse_header_value(header_to_str!(header_value)?, reg, &basic)?
                    }
                    BasicOrValue::Value(idx) => {
                        // Determine the type of the value(s).
                        let basic_val: Value; // for borrowing below
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
                            let values = std::iter::once(header_value).chain(values);
                            let mut arr = Vec::new();
                            for header_value in values {
                                let value = parse_header_value(
                                    header_to_str!(header_value)?,
                                    reg,
                                    value_type,
                                )?;
                                arr.push(value);
                            }
                            PValue::Array(arr)
                        } else {
                            parse_header_value(header_to_str!(header_value)?, reg, value_type)?
                        }
                    }
                },
            );
        }

        Ok(result)
    }
}

fn parse_header_value(header: &str, reg: &Registry, schema: &Value) -> APIResult<PValue> {
    match schema {
        // Recurse
        Value::Ref(idx) => parse_header_value(header, reg, &reg.values[*idx]),

        Value::Validation(v) => {
            let inner = match &v.bov {
                BasicOrValue::Basic(basic) => parse_basic_str(basic, header),
                BasicOrValue::Value(idx) => parse_header_value(header, reg, &reg.values[*idx]),
            }?;
            match v.validate_pval(&inner) {
                Ok(()) => Ok(inner),
                Err(err) => Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "invalid header value".to_string(),
                    internal_message: Some(format!("invalid header value: {}", err)),
                    stack: None,
                    details: None,
                }),
            }
        }

        // If we have an empty header for an option, that's fine.
        Value::Option(_) if header.is_empty() => Ok(PValue::Null),

        // Otherwise recurse.
        Value::Option(opt) => match opt {
            BasicOrValue::Basic(basic) => parse_basic_str(basic, header),
            BasicOrValue::Value(idx) => parse_header_value(header, reg, &reg.values[*idx]),
        },

        Value::Basic(basic) => parse_basic_str(basic, header),

        Value::Struct { .. } | Value::Map(_) | Value::Array(_) => unsupported(reg, schema),

        Value::Literal(lit) => match lit {
            Literal::Str(want) if header == want => Ok(PValue::String(want.to_string())),
            Literal::Bool(true) if header == "true" => Ok(PValue::Bool(true)),
            Literal::Bool(false) if header == "false" => Ok(PValue::Bool(false)),
            Literal::Int(want) if header.parse() == Ok(*want) => {
                Ok(PValue::Number(serde_json::Number::from(*want)))
            }
            Literal::Float(want) if header.parse() == Ok(*want) => {
                if let Some(num) = serde_json::Number::from_f64(*want) {
                    Ok(PValue::Number(num))
                } else {
                    Err(api::Error {
                        code: api::ErrCode::InvalidArgument,
                        message: "invalid header value".to_string(),
                        internal_message: Some(format!("invalid float value: {}", header)),
                        stack: None,
                        details: None,
                    })
                }
            }

            want => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "invalid header value".to_string(),
                internal_message: Some(format!("expected {}, got {}", want.expecting(), header)),
                stack: None,
                details: None,
            }),
        },

        Value::Union(union) => {
            // Find the first value that matches.
            for value in union {
                let result = match value {
                    BasicOrValue::Basic(basic) => parse_basic_str(basic, header),
                    BasicOrValue::Value(idx) => {
                        let value = reg.get(*idx);
                        parse_header_value(header, reg, value)
                    }
                };
                match result {
                    Ok(value) => return Ok(value),
                    Err(_) => continue,
                }
            }
            Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "invalid header value".to_string(),
                internal_message: Some(format!("no union value matched: {}", header)),
                stack: None,
                details: None,
            })
        }
    }
}

impl ParseWithSchema<PValue> for PValue {
    fn parse_with_schema(self, schema: &JSONSchema) -> APIResult<PValue> {
        let reg = schema.registry.as_ref();
        let fields = &schema.root().fields;
        match self {
            PValue::Object(obj) => {
                let mut result = PValues::new();
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
                Ok(PValue::Object(result))
            }

            _ => unexpected_json(reg, schema.root_value(), &self),
        }
    }
}

#[cfg_attr(
    feature = "rttrace",
    tracing::instrument(skip(reg), ret, level = "trace")
)]
fn parse_json_value(this: PValue, reg: &Registry, schema: &Value) -> APIResult<PValue> {
    match schema {
        // Recurse
        Value::Ref(idx) => parse_json_value(this, reg, &reg.values[*idx]),

        Value::Validation(v) => {
            let inner = match &v.bov {
                BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, this),
                BasicOrValue::Value(idx) => parse_json_value(this, reg, &reg.values[*idx]),
            }?;
            match v.validate_pval(&inner) {
                Ok(()) => Ok(inner),
                Err(err) => Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: err.to_string(),
                    internal_message: None,
                    stack: None,
                    details: None,
                }),
            }
        }

        // If we have a null value for an option, that's fine.
        Value::Option(_) if this.is_null() => Ok(PValue::Null),

        // Otherwise recurse.
        Value::Option(opt) => match opt {
            BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, this),
            BasicOrValue::Value(idx) => parse_json_value(this, reg, &reg.values[*idx]),
        },

        Value::Basic(basic) => parse_basic_json(reg, basic, this),

        Value::Literal(lit) => {
            let invalid = |got| {
                Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "invalid value".to_string(),
                    internal_message: Some(format!("expected {}, got {:#?}", lit.expecting(), got)),
                    stack: None,
                    details: None,
                })
            };

            match (this, lit) {
                (PValue::String(got), Literal::Str(want)) if &got == want => {
                    Ok(PValue::String(got))
                }
                (PValue::Bool(got), Literal::Bool(want)) if &got == want => Ok(PValue::Bool(got)),
                (PValue::Number(got), Literal::Int(want)) => {
                    if got.as_i64() == Some(*want) {
                        Ok(PValue::Number(got))
                    } else {
                        invalid(PValue::Number(got))
                    }
                }
                (PValue::Number(got), Literal::Float(want)) => {
                    if got.as_f64() == Some(*want) {
                        Ok(PValue::Number(got))
                    } else {
                        invalid(PValue::Number(got))
                    }
                }
                (got, _) => invalid(got),
            }
        }

        Value::Struct(Struct { fields }) => match this {
            PValue::Object(obj) => {
                let mut result = PValues::new();
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
                Ok(PValue::Object(result))
            }

            _ => unexpected_json(reg, schema, &this),
        },

        Value::Map(value_type) => match this {
            PValue::Object(obj) => {
                let mut result = PValues::new();
                for (key, value) in obj {
                    let value = match value_type {
                        BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, value)?,
                        BasicOrValue::Value(idx) => {
                            parse_json_value(value, reg, &reg.values[*idx])?
                        }
                    };
                    result.insert(key, value);
                }
                Ok(PValue::Object(result))
            }

            _ => unexpected_json(reg, schema, &this),
        },

        Value::Array(value_type) => match this {
            PValue::Array(arr) => {
                let mut result = Vec::with_capacity(arr.len());
                for val in arr {
                    let value = match value_type {
                        BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, val)?,
                        BasicOrValue::Value(idx) => parse_json_value(val, reg, &reg.values[*idx])?,
                    };
                    result.push(value);
                }
                Ok(PValue::Array(result))
            }

            _ => unexpected_json(reg, schema, &this),
        },

        Value::Union(types) => {
            // Find the first type that matches.
            for candidate in types {
                let result = match candidate {
                    BasicOrValue::Basic(basic) => parse_basic_json(reg, basic, this.clone()),
                    BasicOrValue::Value(idx) => {
                        parse_json_value(this.clone(), reg, &reg.values[*idx])
                    }
                };
                if let Ok(value) = result {
                    return Ok(value);
                }
            }

            // Couldn't find a match.
            Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "invalid value".to_string(),
                internal_message: Some(format!("no union type matched: {}", describe_json(&this),)),
                stack: None,
                details: None,
            })
        }
    }
}

fn unexpected_json(reg: &Registry, schema: &Value, value: &PValue) -> APIResult<PValue> {
    Err(api::Error {
        code: api::ErrCode::InvalidArgument,
        message: "invalid value".to_string(),
        internal_message: Some(format!(
            "expected {}, got {}",
            schema.expecting(reg),
            describe_json(value),
        )),
        stack: None,
        details: None,
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
        details: None,
    })
}

fn describe_json(value: &PValue) -> &'static str {
    match value {
        PValue::Null => "null",
        PValue::Bool(_) => "a boolean",
        PValue::Number(_) => "a number",
        PValue::String(_) => "a string",
        PValue::DateTime(_) => "a datetime",
        PValue::Array(_) => "an array",
        PValue::Object(_) => "an object",
    }
}

fn parse_basic_json(reg: &Registry, basic: &Basic, value: PValue) -> APIResult<PValue> {
    match (basic, &value) {
        (Basic::Any, _) => Ok(value),

        (Basic::Null, PValue::Null) => Ok(value),
        (Basic::Bool, PValue::Bool(_)) => Ok(value),
        (Basic::Number, PValue::Number(_)) => Ok(value),
        (Basic::String, PValue::String(_)) => Ok(value),

        (Basic::String, PValue::Number(num)) => Ok(PValue::String(num.to_string())),
        (Basic::String, PValue::Bool(bool)) => Ok(PValue::String(bool.to_string())),

        (_, PValue::String(str)) => match basic {
            Basic::Bool => match str.as_str() {
                "true" => Ok(PValue::Bool(true)),
                "false" => Ok(PValue::Bool(false)),
                _ => Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("invalid boolean value: {}", str),
                    internal_message: None,
                    stack: None,
                    details: None,
                }),
            },
            Basic::Number => serde_json::Number::from_str(str)
                .map(PValue::Number)
                .map_err(|_err| api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("invalid number value: {}", str),
                    internal_message: None,
                    stack: None,
                    details: None,
                }),
            Basic::Null if str == "null" => Ok(PValue::Null),

            _ => unexpected_json(reg, &Value::Basic(*basic), &value),
        },

        _ => unexpected_json(reg, &Value::Basic(*basic), &value),
    }
}

fn parse_basic_str(basic: &Basic, str: &str) -> APIResult<PValue> {
    match basic {
        Basic::Any | Basic::String => Ok(PValue::String(str.to_string())),

        Basic::Null if str.is_empty() || str == "null" => Ok(PValue::Null),

        Basic::Bool => match str {
            "true" => Ok(PValue::Bool(true)),
            "false" => Ok(PValue::Bool(false)),
            _ => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!("invalid boolean value: {}", str),
                internal_message: None,
                stack: None,
                details: None,
            }),
        },

        Basic::Number => serde_json::Number::from_str(str)
            .map(PValue::Number)
            .map_err(|_err| api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!("invalid number value: {}", str),
                internal_message: None,
                stack: None,
                details: None,
            }),

        Basic::DateTime => api::DateTime::parse_from_rfc3339(str)
            .map(PValue::DateTime)
            .map_err(|_err| api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "invalid datetime".to_string(),
                internal_message: Some(format!("invalid datetime string {:?}", str)),
                stack: None,
                details: None,
            }),

        _ => Err(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "invalid value".to_string(),
            internal_message: Some(format!("expected {}, got {:#?}", basic.expecting(), str)),
            stack: None,
            details: None,
        }),
    }
}
