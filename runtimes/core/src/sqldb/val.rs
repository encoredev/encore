use anyhow::Context;
use bytes::{BufMut, BytesMut};
use serde::Serialize;
use std::error::Error;
use tokio_postgres::types::{to_sql_checked, FromSql, IsNull, Kind, ToSql, Type};
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

            Self::PVal(val) => val.to_sql(ty, out),
        }
    }

    fn accepts(ty: &Type) -> bool {
        matches!(*ty, Type::BYTEA | Type::UUID | Type::TEXT | Type::VARCHAR)
            || matches!(ty.kind(), Kind::Array(ty) if <RowValue as ToSql>::accepts(ty))
            || <PValue as ToSql>::accepts(ty)
    }

    to_sql_checked!();
}

impl ToSql for PValue {
    fn to_sql(
        &self,
        ty: &Type,
        out: &mut BytesMut,
    ) -> Result<IsNull, Box<dyn Error + Sync + Send>> {
        match *ty {
            Type::JSON | Type::JSONB => {
                if *ty == Type::JSONB {
                    out.put_u8(1);
                }

                let mut ser = serde_json::ser::Serializer::new(out.writer());
                self.serialize(&mut ser)?;
                Ok(IsNull::No)
            }

            _ => match self {
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
                        let val = chrono::DateTime::parse_from_rfc3339(str).map_err(Box::new)?;
                        val.with_timezone(&chrono::Utc).to_sql(ty, out)
                    }
                    Type::DATE => {
                        let val =
                            chrono::NaiveDate::parse_from_str(str, "%Y-%m-%d").map_err(Box::new)?;
                        val.to_sql(ty, out)
                    }
                    Type::TIME => {
                        let val =
                            chrono::NaiveTime::parse_from_str(str, "%H:%M:%S").map_err(Box::new)?;
                        val.to_sql(ty, out)
                    }
                    Type::UUID => {
                        let val = Uuid::parse_str(str).context("unable to parse uuid")?;
                        val.to_sql(ty, out)
                    }
                    _ => {
                        if let Kind::Enum(_) = ty.kind() {
                            str.to_sql(ty, out)
                        } else {
                            Err(format!("string not supported for column of type {}", ty).into())
                        }
                    }
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
                    Type::TEXT | Type::VARCHAR => self.to_string().to_sql(ty, out),
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
                PValue::DateTime(dt) => match *ty {
                    Type::DATE => dt.naive_utc().date().to_sql(ty, out),
                    Type::TIMESTAMP => dt.naive_utc().to_sql(ty, out),
                    Type::TIMESTAMPTZ => dt.to_sql(ty, out),
                    Type::TEXT | Type::VARCHAR => dt.to_rfc3339().to_sql(ty, out),
                    _ => Err(format!("unsupported type for DateTime: {}", ty).into()),
                },
                PValue::Array(arr) => arr.to_sql(ty, out),
                PValue::Object(_) => {
                    Err(format!("object not supported for column of type {}", ty).into())
                }
            },
        }
    }

    fn accepts(ty: &Type) -> bool {
        matches!(
            *ty,
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
                | Type::TIME
        ) || matches!(ty.kind(), Kind::Enum(_))
            || matches!(ty.kind(), Kind::Array(ty) if <PValue as ToSql>::accepts(ty))
    }
    to_sql_checked!();
}

impl<'a> FromSql<'a> for RowValue {
    fn from_sql(ty: &Type, raw: &'a [u8]) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(match *ty {
            Type::BYTEA => {
                let val: Vec<u8> = FromSql::from_sql(ty, raw)?;
                Self::Bytes(val)
            }
            Type::UUID => {
                let val: uuid::Uuid = FromSql::from_sql(ty, raw)?;
                Self::Uuid(val)
            }
            _ => {
                if <PValue as FromSql>::accepts(ty) {
                    Self::PVal(FromSql::from_sql(ty, raw)?)
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
        matches!(*ty, Type::BYTEA | Type::UUID)
            || matches!(ty.kind(), Kind::Array(ty) if <RowValue as FromSql>::accepts(ty))
            || <PValue as FromSql>::accepts(ty)
    }
}

impl<'a> FromSql<'a> for PValue {
    fn from_sql(ty: &Type, raw: &'a [u8]) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(match *ty {
            Type::BOOL => {
                let val: bool = FromSql::from_sql(ty, raw)?;
                PValue::Bool(val)
            }
            Type::TEXT | Type::VARCHAR => {
                let val: String = FromSql::from_sql(ty, raw)?;
                PValue::String(val)
            }
            Type::INT2 => {
                let val: i16 = FromSql::from_sql(ty, raw)?;
                PValue::Number(serde_json::Number::from(val))
            }
            Type::INT4 => {
                let val: i32 = FromSql::from_sql(ty, raw)?;
                PValue::Number(serde_json::Number::from(val))
            }
            Type::INT8 => {
                let val: i64 = FromSql::from_sql(ty, raw)?;
                PValue::Number(serde_json::Number::from(val))
            }
            Type::OID => {
                let val: u32 = FromSql::from_sql(ty, raw)?;
                PValue::Number(serde_json::Number::from(val))
            }
            Type::JSON | Type::JSONB => {
                let val: serde_json::Value = FromSql::from_sql(ty, raw)?;
                val.into()
            }
            Type::FLOAT4 => {
                let val: f32 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val as f64) {
                    Some(num) => PValue::Number(num),
                    None => PValue::Null,
                }
            }
            Type::FLOAT8 => {
                let val: f64 = FromSql::from_sql(ty, raw)?;
                match serde_json::Number::from_f64(val) {
                    Some(num) => PValue::Number(num),
                    None => PValue::Null,
                }
            }
            Type::TIMESTAMP => {
                let val: DateTime = FromSql::from_sql(ty, raw)?;
                PValue::DateTime(val)
            }
            Type::TIMESTAMPTZ => {
                let val: DateTime = FromSql::from_sql(ty, raw)?;
                PValue::DateTime(val)
            }
            Type::DATE => {
                let val: chrono::NaiveDate = FromSql::from_sql(ty, raw)?;
                PValue::String(val.to_string())
            }
            Type::TIME => {
                let val: chrono::NaiveTime = FromSql::from_sql(ty, raw)?;
                PValue::String(val.to_string())
            }

            _ => {
                if let Kind::Array(_) = ty.kind() {
                    let val: Vec<_> = FromSql::from_sql(ty, raw)?;
                    PValue::Array(val)
                } else if let Kind::Enum(_) = ty.kind() {
                    let val = std::str::from_utf8(raw)?;
                    PValue::String(val.to_string())
                } else {
                    return Err(format!("unsupported type: {:?}", ty).into());
                }
            }
        })
    }

    fn from_sql_null(_: &Type) -> Result<Self, Box<dyn Error + Sync + Send>> {
        Ok(PValue::Null)
    }

    fn accepts(ty: &Type) -> bool {
        matches!(
            *ty,
            Type::BOOL
                | Type::TEXT
                | Type::VARCHAR
                | Type::INT2
                | Type::INT4
                | Type::INT8
                | Type::OID
                | Type::JSONB
                | Type::JSON
                | Type::FLOAT4
                | Type::FLOAT8
                | Type::TIMESTAMP
                | Type::TIMESTAMPTZ
                | Type::DATE
                | Type::TIME
        ) || matches!(ty.kind(), Kind::Enum(_))
            || matches!(ty.kind(), Kind::Array(ty) if <PValue as FromSql>::accepts(ty))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use bytes::BytesMut;
    use serde_json::json;
    use tokio_postgres::types::Type;

    #[test]
    fn test_rowvalue_to_sql_bytes() {
        let value = RowValue::Bytes(vec![1, 2, 3]);
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::BYTEA, &mut buf);
        assert!(result.is_ok());
        assert_eq!(buf, vec![1, 2, 3]);
    }

    #[test]
    fn test_rowvalue_to_sql_uuid() {
        let uuid = Uuid::nil();
        let value = RowValue::Uuid(uuid);
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::UUID, &mut buf);
        assert!(result.is_ok());
    }

    #[test]
    fn test_rowvalue_to_sql_pval() {
        let value = RowValue::PVal(PValue::String("test".to_string()));
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::TEXT, &mut buf);
        assert!(result.is_ok());
    }

    #[test]
    fn test_pvalue_to_sql_json() {
        let value: PValue = json!({"key": "value"}).into();
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::JSONB, &mut buf);
        assert!(result.is_ok());
        assert_eq!(buf[0], 1); // JSONB prefix
    }

    #[test]
    fn test_pvalue_to_sql_number() {
        let value = PValue::Number(serde_json::Number::from(42));
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::INT4, &mut buf);
        assert!(result.is_ok());
    }

    #[test]
    fn test_rowvalue_from_sql_bytes() {
        let raw = &[1, 2, 3];
        let result = RowValue::from_sql(&Type::BYTEA, raw);
        assert!(result.is_ok());
        if let RowValue::Bytes(val) = result.unwrap() {
            assert_eq!(val, vec![1, 2, 3]);
        } else {
            panic!("Expected RowValue::Bytes");
        }
    }

    #[test]
    fn test_rowvalue_from_sql_uuid() {
        let uuid = Uuid::nil();
        let raw = uuid.as_bytes();
        let result = RowValue::from_sql(&Type::UUID, raw);
        assert!(result.is_ok());
        if let RowValue::Uuid(val) = result.unwrap() {
            assert_eq!(val, uuid);
        } else {
            panic!("Expected RowValue::Uuid");
        }
    }

    #[test]
    fn test_pvalue_from_sql_bool() {
        let raw = &[1]; // true
        let result = PValue::from_sql(&Type::BOOL, raw);
        assert!(result.is_ok());
        if let PValue::Bool(val) = result.unwrap() {
            assert!(val);
        } else {
            panic!("Expected PValue::Bool");
        }
    }

    #[test]
    fn test_pvalue_from_sql_string() {
        let raw = b"test";
        let result = PValue::from_sql(&Type::TEXT, raw);
        assert!(result.is_ok());
        if let PValue::String(val) = result.unwrap() {
            assert_eq!(val, "test");
        } else {
            panic!("Expected PValue::String");
        }
    }

    #[test]
    fn test_pvalue_from_sql_number() {
        let raw = &[0, 0, 0, 42]; // INT4 representation of 42
        let result = PValue::from_sql(&Type::INT4, raw);
        assert!(result.is_ok());
        if let PValue::Number(val) = result.unwrap() {
            assert_eq!(val.as_i64().unwrap(), 42);
        } else {
            panic!("Expected PValue::Number");
        }
    }

    #[test]
    fn test_pvalue_to_sql_invalid_type() {
        let value = PValue::String("test".to_string());
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::BOOL, &mut buf); // Invalid type
        assert!(result.is_err());
    }

    #[test]
    fn test_rowvalue_from_sql_invalid_type() {
        let raw = &[1, 2, 3];
        let result = RowValue::from_sql(&Type::BOOL, raw); // Invalid type
        assert!(result.is_err());
    }

    #[test]
    fn test_pvalue_from_sql_null() {
        let result = PValue::from_sql_null(&Type::TEXT);
        assert!(result.is_ok());
        if let PValue::Null = result.unwrap() {
            // Expected null value
        } else {
            panic!("Expected PValue::Null");
        }
    }

    #[test]
    fn test_rowvalue_from_sql_null() {
        let result = RowValue::from_sql_null(&Type::TEXT);
        assert!(result.is_ok());
        if let RowValue::PVal(PValue::Null) = result.unwrap() {
            // Expected null value
        } else {
            panic!("Expected RowValue::PVal(PValue::Null)");
        }
    }

    #[test]
    fn test_pvalue_to_sql_array() {
        let value = PValue::Array(vec![
            PValue::Number(serde_json::Number::from(1)),
            PValue::Number(serde_json::Number::from(2)),
        ]);
        let mut buf = BytesMut::new();
        let result = value.to_sql(&Type::INT4_ARRAY, &mut buf);
        assert!(result.is_ok());
    }

    #[test]
    fn test_pvalue_from_sql_array() {
        // raw representation of INT4_ARRAY with values 1,2,3
        let raw: &[u8] = &[
            0, 0, 0, 1, // dimentions
            0, 0, 0, 0, // has nulls
            0, 0, 0, 23, // element type
            0, 0, 0, 3, // array length
            0, 0, 0, 1, // lower bound
            0, 0, 0, 4, // value length
            0, 0, 0, 1, // value
            0, 0, 0, 4, // value length
            0, 0, 0, 2, // value
            0, 0, 0, 4, // value length
            0, 0, 0, 3, // value
        ];

        let result = PValue::from_sql(&Type::INT4_ARRAY, raw);
        assert!(result.is_ok());
        if let PValue::Array(val) = result.unwrap() {
            assert_eq!(val.len(), 3);
            assert_eq!(
                val,
                vec![
                    PValue::Number(1.into()),
                    PValue::Number(2.into()),
                    PValue::Number(3.into())
                ]
            );
        } else {
            panic!("Expected PValue::Array");
        }
    }
}
