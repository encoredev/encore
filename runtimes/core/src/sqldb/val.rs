use bytes::BytesMut;
use std::error::Error;
use tokio_postgres::types::{FromSql, IsNull, ToSql, Type};

#[derive(Debug)]
pub enum RowValue {
    Json(serde_json::Value),
    Bytes(Vec<u8>),
    Uuid(uuid::Uuid),
}

impl ToSql for RowValue {
    fn to_sql(&self, ty: &Type, out: &mut BytesMut) -> Result<IsNull, Box<dyn Error + Sync + Send>>
    where
        Self: Sized,
    {
        match self {
            Self::Bytes(val) => val.to_sql(ty, out),
            Self::Uuid(val) => match *ty {
                Type::UUID => val.to_sql(ty, out),
                Type::TEXT => val.to_string().to_sql(ty, out),
                _ => {
                    return Err(format!("uuid not supported for column of type {}", ty).into());
                }
            },

            Self::Json(val) => match *ty {
                Type::JSON | Type::JSONB => val.to_sql(ty, out),

                _ => match val {
                    serde_json::Value::Null => Ok(IsNull::Yes),
                    serde_json::Value::Bool(bool) => match *ty {
                        Type::BOOL => bool.to_sql(ty, out),
                        Type::TEXT => bool.to_string().to_sql(ty, out),
                        _ => {
                            return Err(
                                format!("bool not supported for column of type {}", ty).into()
                            );
                        }
                    },

                    serde_json::Value::String(str) => match *ty {
                        Type::TEXT => str.to_sql(ty, out),
                        Type::BYTEA => {
                            let val = str.as_bytes();
                            val.to_sql(ty, out)
                        }
                        Type::TIMESTAMP => {
                            let val =
                                chrono::NaiveDateTime::parse_from_str(str, "%Y-%m-%d %H:%M:%S")
                                    .map_err(|e| Box::new(e))?;
                            val.to_sql(ty, out)
                        }
                        Type::TIMESTAMPTZ => {
                            let val = chrono::DateTime::parse_from_rfc3339(str)
                                .map_err(|e| Box::new(e))?;
                            val.with_timezone(&chrono::Utc).to_sql(ty, out)
                        }
                        Type::DATE => {
                            let val = chrono::NaiveDate::parse_from_str(str, "%Y-%m-%d")
                                .map_err(|e| Box::new(e))?;
                            val.to_sql(ty, out)
                        }
                        Type::TIME => {
                            let val = chrono::NaiveTime::parse_from_str(str, "%H:%M:%S")
                                .map_err(|e| Box::new(e))?;
                            val.to_sql(ty, out)
                        }
                        _ => {
                            return Err(
                                format!("string not supported for column of type {}", ty).into()
                            );
                        }
                    },

                    serde_json::Value::Number(num) => match *ty {
                        Type::INT2 => {
                            let val: Result<i16, _> = if num.is_i64() {
                                num.as_i64().unwrap().try_into()
                            } else if num.is_u64() {
                                num.as_u64().unwrap().try_into()
                            } else if num.is_f64() {
                                let float = num.as_f64().unwrap();
                                let res = float as i16;
                                if res as f64 == float {
                                    Ok(res)
                                } else {
                                    return Err(format!("number {} is not an i16", float).into());
                                }
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.map_err(|e| Box::new(e))?.to_sql(ty, out)
                        }
                        Type::INT4 => {
                            let val: Result<i32, _> = if num.is_i64() {
                                num.as_i64().unwrap().try_into()
                            } else if num.is_u64() {
                                num.as_u64().unwrap().try_into()
                            } else if num.is_f64() {
                                let float = num.as_f64().unwrap();
                                let res = float as i32;
                                if res as f64 == float {
                                    Ok(res)
                                } else {
                                    return Err(format!("number {} is not an i32", float).into());
                                }
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.map_err(|e| Box::new(e))?.to_sql(ty, out)
                        }
                        Type::INT8 => {
                            let val: Result<i64, _> = if num.is_i64() {
                                Ok(num.as_i64().unwrap())
                            } else if num.is_u64() {
                                num.as_u64().unwrap().try_into()
                            } else if num.is_f64() {
                                let float = num.as_f64().unwrap();
                                let res = float as i64;
                                if res as f64 == float {
                                    Ok(res)
                                } else {
                                    return Err(format!("number {} is not an i64", float).into());
                                }
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.map_err(|e| Box::new(e))?.to_sql(ty, out)
                        }
                        Type::FLOAT4 => {
                            let val: f32 = if num.is_i64() {
                                num.as_i64().unwrap() as f32
                            } else if num.is_u64() {
                                num.as_u64().unwrap() as f32
                            } else if num.is_f64() {
                                num.as_f64().unwrap() as f32
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.to_sql(ty, out)
                        }
                        Type::FLOAT8 => {
                            let val: f64 = if num.is_i64() {
                                num.as_i64().unwrap() as f64
                            } else if num.is_u64() {
                                num.as_u64().unwrap() as f64
                            } else if num.is_f64() {
                                num.as_f64().unwrap()
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.to_sql(ty, out)
                        }
                        Type::OID => {
                            let val: Result<u32, _> = if num.is_i64() {
                                num.as_i64().unwrap().try_into()
                            } else if num.is_u64() {
                                num.as_u64().unwrap().try_into()
                            } else if num.is_f64() {
                                let float = num.as_f64().unwrap();
                                let res = float as u32;
                                if res as f64 == float {
                                    Ok(res)
                                } else {
                                    return Err(format!("number {} is not an u32", float).into());
                                }
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            };
                            val.map_err(|e| Box::new(e))?.to_sql(ty, out)
                        }
                        Type::TEXT => val.to_string().to_sql(ty, out),
                        _ => {
                            if num.is_i64() {
                                num.as_i64().unwrap().to_sql(ty, out)
                            } else if num.is_u64() {
                                (num.as_u64().unwrap() as i64).to_sql(ty, out)
                            } else if num.is_f64() {
                                num.as_f64().unwrap().to_sql(ty, out)
                            } else {
                                return Err(format!("unsupported number: {:?}", num).into());
                            }
                        }
                    },

                    serde_json::Value::Array(_) => {
                        return Err(format!("array not supported for column of type {}", ty).into());
                    }
                    serde_json::Value::Object(_) => {
                        return Err(
                            format!("object not supported for column of type {}", ty).into()
                        );
                    }
                },
            },
        }
    }

    fn accepts(ty: &Type) -> bool
    where
        Self: Sized,
    {
        match *ty {
            Type::BOOL
            | Type::BYTEA
            | Type::CHAR
            | Type::NAME
            | Type::TEXT
            | Type::INT2
            | Type::INT4
            | Type::INT8
            | Type::OID
            | Type::JSONB
            | Type::JSON
            | Type::POINT
            | Type::BOX
            | Type::PATH
            | Type::UUID
            | Type::FLOAT4
            | Type::FLOAT8
            | Type::TIMESTAMP
            | Type::TIMESTAMPTZ
            | Type::DATE
            | Type::TIME => true,
            _ => false,
        }
    }

    tokio_postgres::types::to_sql_checked!();
}

impl<'a> FromSql<'a> for RowValue {
    fn from_sql(ty: &Type, raw: &'a [u8]) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(match *ty {
            Type::BOOL => {
                let val: bool = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Bool(val))
            }
            Type::BYTEA => {
                let val: Vec<u8> = FromSql::from_sql(ty, raw)?;
                Self::Bytes(val)
            }
            Type::TEXT => {
                let val: String = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::String(val))
            }
            Type::INT2 => {
                let val: i16 = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Number(serde_json::Number::from(val)))
            }
            Type::INT4 => {
                let val: i32 = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Number(serde_json::Number::from(val)))
            }
            Type::INT8 => {
                let val: i64 = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Number(serde_json::Number::from(val)))
            }
            Type::OID => {
                let val: u32 = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Number(serde_json::Number::from(val)))
            }
            Type::JSON | Type::JSONB => {
                let val: serde_json::Value = FromSql::from_sql(ty, raw)?;
                Self::Json(val)
            }
            Type::JSON_ARRAY | Type::JSONB_ARRAY => {
                let val: Vec<serde_json::Value> = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::Array(val))
            }
            Type::UUID => {
                let val: uuid::Uuid = FromSql::from_sql(ty, raw)?;
                Self::Uuid(val)
            }

            Type::FLOAT4 => {
                let val: f32 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val as f64) {
                    Some(num) => Self::Json(serde_json::Value::Number(num)),
                    None => Self::Json(serde_json::Value::Null),
                }
            }

            Type::FLOAT8 => {
                let val: f64 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val) {
                    Some(num) => Self::Json(serde_json::Value::Number(num)),
                    None => Self::Json(serde_json::Value::Null),
                }
            }

            Type::TIMESTAMP => {
                let val: chrono::NaiveDateTime = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::String(val.to_string()))
            }
            Type::TIMESTAMPTZ => {
                let val: chrono::DateTime<chrono::Utc> = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::String(val.to_string()))
            }
            Type::DATE => {
                let val: chrono::NaiveDate = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::String(val.to_string()))
            }
            Type::TIME => {
                let val: chrono::NaiveTime = FromSql::from_sql(ty, raw)?;
                Self::Json(serde_json::Value::String(val.to_string()))
            }

            _ => {
                return Err(format!("unsupported type: {:?}", ty).into());
            }
        })
    }

    fn from_sql_null(_: &Type) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(Self::Json(serde_json::Value::Null))
    }

    fn accepts(ty: &Type) -> bool {
        match *ty {
            Type::BOOL
            | Type::BYTEA
            | Type::TEXT
            | Type::INT2
            | Type::INT4
            | Type::INT8
            | Type::OID
            | Type::JSONB
            | Type::JSON
            | Type::UUID
            | Type::FLOAT4
            | Type::FLOAT8
            | Type::TIMESTAMP
            | Type::TIMESTAMPTZ
            | Type::DATE
            | Type::TIME => true,
            _ => false,
        }
    }
}
