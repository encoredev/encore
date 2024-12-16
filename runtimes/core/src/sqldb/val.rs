use anyhow::Context;
use bytes::{BufMut, BytesMut};
use serde::Serialize;
use std::error::Error;
use tokio_postgres::types::{accepts, to_sql_checked, FromSql, IsNull, Kind, ToSql, Type};
use uuid::Uuid;

use crate::api::{DateTime, PValue};

#[derive(Debug)]
pub enum RowValue {
    PVal(PValue),
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
                Type::TEXT | Type::VARCHAR => val.to_string().to_sql(ty, out),
                _ => Err(format!("uuid not supported for column of type {}", ty).into()),
            },

            Self::PVal(val) => match *ty {
                Type::JSON | Type::JSONB => val.to_sql(ty, out),

                _ => match val {
                    PValue::Null => Ok(IsNull::Yes),
                    PValue::Bool(bool) => match *ty {
                        Type::BOOL => bool.to_sql(ty, out),
                        Type::TEXT | Type::VARCHAR => bool.to_string().to_sql(ty, out),
                        _ => Err(format!("bool not supported for column of type {}", ty).into()),
                    },

                    PValue::String(str) => match *ty {
                        Type::TEXT | Type::VARCHAR => str.to_sql(ty, out),
                        Type::BYTEA => {
                            let val = str.as_bytes();
                            val.to_sql(ty, out)
                        }
                        Type::TIMESTAMP => {
                            let val = str.parse::<chrono::NaiveDateTime>().map_err(Box::new)?;
                            val.to_sql(ty, out)
                        }
                        Type::TIMESTAMPTZ => {
                            let val =
                                chrono::DateTime::parse_from_rfc3339(str).map_err(Box::new)?;
                            val.with_timezone(&chrono::Utc).to_sql(ty, out)
                        }
                        Type::DATE => {
                            let val = chrono::NaiveDate::parse_from_str(str, "%Y-%m-%d")
                                .map_err(Box::new)?;
                            val.to_sql(ty, out)
                        }
                        Type::TIME => {
                            let val = chrono::NaiveTime::parse_from_str(str, "%H:%M:%S")
                                .map_err(Box::new)?;
                            val.to_sql(ty, out)
                        }
                        Type::UUID => {
                            let val = Uuid::parse_str(str).context("unable to parse uuid")?;
                            val.to_sql(ty, out)
                        }
                        _ => Err(format!("string not supported for column of type {}", ty).into()),
                    },

                    PValue::Number(num) => match *ty {
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
                            val.map_err(Box::new)?.to_sql(ty, out)
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
                            val.map_err(Box::new)?.to_sql(ty, out)
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
                            val.map_err(Box::new)?.to_sql(ty, out)
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
                            val.map_err(Box::new)?.to_sql(ty, out)
                        }
                        Type::TEXT | Type::VARCHAR => val.to_string().to_sql(ty, out),
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
                    PValue::DateTime(dt) => dt.to_sql(ty, out),
                    PValue::Array(_) => {
                        Err(format!("array not supported for column of type {}", ty).into())
                    }
                    PValue::Object(_) => {
                        Err(format!("object not supported for column of type {}", ty).into())
                    }
                },
            },
        }
    }

    fn accepts(ty: &Type) -> bool
    where
        Self: Sized,
    {
        matches!(
            *ty,
            Type::BOOL
                | Type::BYTEA
                | Type::CHAR
                | Type::NAME
                | Type::TEXT
                | Type::VARCHAR
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
                | Type::TIME
        )
    }

    tokio_postgres::types::to_sql_checked!();
}

impl<'a> FromSql<'a> for RowValue {
    fn from_sql(ty: &Type, raw: &'a [u8]) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(match *ty {
            Type::BOOL => {
                let val: bool = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Bool(val))
            }
            Type::BYTEA => {
                let val: Vec<u8> = FromSql::from_sql(ty, raw)?;
                Self::Bytes(val)
            }
            Type::TEXT | Type::VARCHAR => {
                let val: String = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::String(val))
            }

            Type::INT2 => {
                let val: i16 = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Number(serde_json::Number::from(val)))
            }
            Type::INT4 => {
                let val: i32 = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Number(serde_json::Number::from(val)))
            }
            Type::INT8 => {
                let val: i64 = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Number(serde_json::Number::from(val)))
            }
            Type::OID => {
                let val: u32 = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Number(serde_json::Number::from(val)))
            }
            Type::JSON | Type::JSONB => {
                let val: PValue = FromSql::from_sql(ty, raw)?;
                Self::PVal(val)
            }
            Type::JSON_ARRAY | Type::JSONB_ARRAY => {
                let val: Vec<PValue> = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::Array(val))
            }
            Type::UUID => {
                let val: uuid::Uuid = FromSql::from_sql(ty, raw)?;
                Self::Uuid(val)
            }

            Type::FLOAT4 => {
                let val: f32 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val as f64) {
                    Some(num) => Self::PVal(PValue::Number(num)),
                    None => Self::PVal(PValue::Null),
                }
            }

            Type::FLOAT8 => {
                let val: f64 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val) {
                    Some(num) => Self::PVal(PValue::Number(num)),
                    None => Self::PVal(PValue::Null),
                }
            }

            Type::TIMESTAMP => {
                let val: DateTime = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::DateTime(val))
            }
            Type::TIMESTAMPTZ => {
                let val: DateTime = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::DateTime(val))
            }
            Type::DATE => {
                let val: chrono::NaiveDate = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::String(val.to_string()))
            }
            Type::TIME => {
                let val: chrono::NaiveTime = FromSql::from_sql(ty, raw)?;
                Self::PVal(PValue::String(val.to_string()))
            }

            _ => {
                if let Kind::Enum(_) = ty.kind() {
                    let val = std::str::from_utf8(raw)?;
                    Self::PVal(PValue::String(val.to_string()))
                } else {
                    return Err(format!("unsupported type: {:?}", ty).into());
                }
            }
        })
    }

    fn from_sql_null(_: &Type) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(Self::PVal(PValue::Null))
    }

    fn accepts(ty: &Type) -> bool {
        match *ty {
            Type::BOOL
            | Type::BYTEA
            | Type::TEXT
            | Type::VARCHAR
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
            ref other => matches!(other.kind(), Kind::Enum(_)),
        }
    }
}

impl<'a> FromSql<'a> for PValue {
    fn from_sql(ty: &Type, raw: &'a [u8]) -> Result<PValue, Box<dyn Error + Sync + Send>> {
        let val: serde_json::Value = FromSql::from_sql(ty, raw)?;
        Ok(val.into())
    }

    accepts!(JSON, JSONB);
}

impl ToSql for PValue {
    fn to_sql(
        &self,
        ty: &Type,
        out: &mut BytesMut,
    ) -> Result<IsNull, Box<dyn Error + Sync + Send>> {
        if *ty == Type::JSONB {
            out.put_u8(1);
        }

        let mut ser = serde_json::ser::Serializer::new(out.writer());
        self.serialize(&mut ser)?;
        Ok(IsNull::No)
    }

    accepts!(JSON, JSONB);
    to_sql_checked!();
}
