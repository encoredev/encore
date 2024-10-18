use encore_runtime_core::api::{PValue, PValues};
use napi::{bindgen_prelude::*, sys, JsObject, JsUnknown, NapiValue, Result};
use serde_json::Number;

#[allow(dead_code)]
pub fn parse_pvalue(val: JsUnknown) -> Result<PValue> {
    let val = PVal::from_unknown(val)?;
    Ok(val.0)
}

pub fn parse_pvalues(val: JsUnknown) -> Result<PValues> {
    let val = PVals::from_unknown(val)?;
    Ok(val.0)
}

#[allow(dead_code)]
pub fn pvalue_to_js(env: Env, val: PValue) -> Result<JsUnknown> {
    let env = env.raw();
    unsafe {
        let val = PVal::to_napi_value(env, PVal(val))?;
        JsUnknown::from_raw(env, val)
    }
}

#[allow(dead_code)]
pub fn pvalues_to_js(env: Env, val: PValues) -> Result<JsUnknown> {
    let env = env.raw();
    unsafe {
        let val = PVals::to_napi_value(env, PVals(val))?;
        JsUnknown::from_raw(env, val)
    }
}

pub struct PVal(pub PValue);
pub struct PVals(pub PValues);

impl ToNapiValue for PVal {
    unsafe fn to_napi_value(env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        match val.0 {
            PValue::Null => unsafe { Null::to_napi_value(env, Null) },
            PValue::Bool(b) => unsafe { bool::to_napi_value(env, b) },
            PValue::Number(n) => unsafe { Number::to_napi_value(env, n) },
            PValue::String(s) => unsafe { String::to_napi_value(env, s) },
            PValue::Array(arr) => unsafe {
                let vals = std::mem::transmute::<Vec<PValue>, Vec<PVal>>(arr);
                Vec::<PVal>::to_napi_value(env, vals)
            },
            PValue::Object(obj) => unsafe { PVals::to_napi_value(env, PVals(obj)) },
        }
    }
}

impl FromNapiValue for PVal {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        let ty = type_of!(env, napi_val)?;
        let val = PVal(match ty {
            ValueType::Boolean => PValue::Bool(unsafe { bool::from_napi_value(env, napi_val)? }),
            ValueType::Number => PValue::Number(unsafe { Number::from_napi_value(env, napi_val)? }),
            ValueType::String => PValue::String(unsafe { String::from_napi_value(env, napi_val)? }),
            ValueType::Object => {
                let mut is_arr = false;
                check_status!(
                    unsafe { sys::napi_is_array(env, napi_val, &mut is_arr) },
                    "Failed to detect whether given js is an array"
                )?;

                if is_arr {
                    PValue::Array(unsafe {
                        let vals = Vec::<PVal>::from_napi_value(env, napi_val)?;
                        // Transmute Vec<PVal> to Vec<PValue> since that's what PValue::Array expects.
                        // This is safe because PVal is a newtype around PValue.
                        std::mem::transmute::<Vec<PVal>, Vec<PValue>>(vals)
                    })
                } else {
                    PValue::Object(unsafe { PVals::from_napi_value(env, napi_val)?.0 })
                }
            }
            ValueType::BigInt => todo!(),
            ValueType::Null => PValue::Null,
            ValueType::Function => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "JS functions cannot be represented as a serde_json::Value".to_owned(),
                ))
            }
            ValueType::Undefined => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "undefined cannot be represented as a serde_json::Value".to_owned(),
                ))
            }
            ValueType::Symbol => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "JS symbols cannot be represented as a serde_json::Value".to_owned(),
                ))
            }
            ValueType::External => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "External JS objects cannot be represented as a serde_json::Value".to_owned(),
                ))
            }
            _ => {
                return Err(Error::new(
                    Status::InvalidArg,
                    "Unknown JS variables cannot be represented as a serde_json::Value".to_owned(),
                ))
            }
        });

        Ok(val)
    }
}

impl ToNapiValue for PVals {
    unsafe fn to_napi_value(raw_env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        let env = Env::from_raw(raw_env);
        let mut obj = env.create_object()?;

        for (k, v) in val.0.into_iter() {
            obj.set(k, PVal(v))?;
        }

        unsafe { JsObject::to_napi_value(raw_env, obj) }
    }
}

impl FromNapiValue for PVals {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        let obj = JsObject::from_napi_value(env, napi_val)?;

        let mut map = PVals(PValues::new());
        for key in JsObject::keys(&obj)?.into_iter() {
            if let Some(val) = obj.get::<_, PVal>(&key)? {
                map.0.insert(key, val.0);
            }
        }

        Ok(map)
    }
}
