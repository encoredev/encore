use chrono::TimeZone;
use encore_runtime_core::api::{self, auth, PValue, PValues};
use napi::{bindgen_prelude::*, sys, JsDate, JsObject, JsUnknown, NapiValue, Result};
use serde_json::Number;

#[allow(dead_code)]
pub fn parse_pvalue(val: JsUnknown) -> Result<PValue> {
    let val = PVal::from_unknown(val)?;
    Ok(val.0)
}

pub fn parse_pvalues(val: JsUnknown) -> Result<Option<PValues>> {
    if val
        .get_type()
        .is_ok_and(|t| t == napi::ValueType::Undefined || t == napi::ValueType::Null)
    {
        return Ok(None);
    }

    let val = PVals::from_unknown(val)?;
    Ok(Some(val.0))
}

#[allow(dead_code)]
pub fn pvalue_to_js(env: Env, val: &PValue) -> Result<JsUnknown> {
    let env = env.raw();

    unsafe {
        // Safety: the memory representation is the same for &PValue and &PVal.
        let pv = std::mem::transmute::<&PValue, &PVal>(val);
        let val = ToNapiValue::to_napi_value(env, pv)?;
        JsUnknown::from_raw(env, val)
    }
}

#[allow(dead_code)]
pub fn pvalues_to_js(env: Env, val: &PValues) -> Result<JsUnknown> {
    let env = env.raw();
    unsafe {
        let pv = std::mem::transmute::<&PValues, &PVals>(val);
        let val = ToNapiValue::to_napi_value(env, pv)?;
        JsUnknown::from_raw(env, val)
    }
}

#[derive(Clone)]
pub struct PVal(pub PValue);
#[derive(Clone)]
pub struct PVals(pub PValues);

impl ToNapiValue for PVal {
    unsafe fn to_napi_value(env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        match val.0 {
            PValue::Null => unsafe { Null::to_napi_value(env, Null) },
            PValue::Bool(b) => unsafe { bool::to_napi_value(env, b) },
            PValue::Number(n) => unsafe { Number::to_napi_value(env, n.to_owned()) },
            PValue::String(s) => unsafe { ToNapiValue::to_napi_value(env, s) },
            PValue::Array(arr) => {
                let env2 = Env::from_raw(env);
                let mut out = env2.create_array(arr.len() as u32)?;

                for (i, v) in arr.into_iter().enumerate() {
                    out.set(i as u32, PVal(v))?;
                }

                unsafe { Array::to_napi_value(env, out) }
            }
            PValue::Object(obj) => unsafe { ToNapiValue::to_napi_value(env, PVals(obj)) },
            PValue::DateTime(dt) => {
                let env2 = Env::from_raw(env);
                let ts = dt.timestamp_millis();
                let dt = env2.create_date(ts as f64)?;
                JsDate::to_napi_value(env, dt)
            }
        }
    }
}

impl ToNapiValue for &PVal {
    unsafe fn to_napi_value(env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        match &val.0 {
            PValue::Null => unsafe { Null::to_napi_value(env, Null) },
            PValue::Bool(b) => unsafe { bool::to_napi_value(env, *b) },
            PValue::Number(n) => unsafe { Number::to_napi_value(env, n.to_owned()) },
            PValue::String(s) => unsafe { ToNapiValue::to_napi_value(env, s) },
            PValue::Array(arr) => {
                let env2 = Env::from_raw(env);
                let mut out = env2.create_array(arr.len() as u32)?;

                for (i, v) in arr.iter().enumerate() {
                    // Safety: the memory representation is the same for &PValue and &PVal.
                    let pv = unsafe { std::mem::transmute::<&PValue, &PVal>(v) };
                    out.set(i as u32, pv)?;
                }

                unsafe { Array::to_napi_value(env, out) }
            }
            PValue::Object(obj) => unsafe {
                // Safety: the memory representation is the same for &PValue and &PVal.
                let pv = std::mem::transmute::<&PValues, &PVals>(obj);
                ToNapiValue::to_napi_value(env, pv)
            },
            PValue::DateTime(dt) => {
                let env2 = Env::from_raw(env);
                let ts = dt.timestamp_millis();
                let dt = env2.create_date(ts as f64)?;
                JsDate::to_napi_value(env, dt)
            }
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
                    // Is it a date?
                    let mut is_date = false;
                    check_status!(
                        unsafe { sys::napi_is_date(env, napi_val, &mut is_date) },
                        "Failed to detect whether given js is a date"
                    )?;
                    if is_date {
                        let dt = JsDate::from_napi_value(env, napi_val)?;
                        let millis = dt.value_of()?;
                        let ts = timestamp_to_dt(millis);
                        PValue::DateTime(ts)
                    } else {
                        PValue::Object(unsafe { PVals::from_napi_value(env, napi_val)?.0 })
                    }
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

impl ToNapiValue for &PVals {
    unsafe fn to_napi_value(raw_env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        let env = Env::from_raw(raw_env);
        let mut obj = env.create_object()?;

        for (k, v) in val.0.iter() {
            // Safety: the memory representation is the same for &PValue and &PVal.
            let pv = unsafe { std::mem::transmute::<&PValue, &PVal>(v) };
            obj.set(k, pv)?;
        }

        unsafe { JsObject::to_napi_value(raw_env, obj) }
    }
}

impl FromNapiValue for PVals {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        assert_type_of!(env, napi_val, ValueType::Object)?;
        let obj = JsObject::from_napi_value(env, napi_val)?;

        let mut map = PVals(PValues::new());
        for key in JsObject::keys(&obj)?.into_iter() {
            if let Some(val) = obj_get_pval(env, napi_val, &key)? {
                map.0.insert(key, val);
            }
        }

        Ok(map)
    }
}

pub fn encode_request_payload(
    env: Env,
    p: Option<&api::RequestPayload>,
) -> napi::Result<JsUnknown> {
    let Some(p) = p else {
        return Ok(env.get_null()?.into_unknown());
    };

    let mut obj = env.create_object()?;

    add_fields_to_obj(&mut obj, p.path.as_ref())?;
    add_fields_to_obj(&mut obj, p.query.as_ref())?;
    add_fields_to_obj(&mut obj, p.header.as_ref())?;

    match &p.body {
        api::Body::Typed(typed) => add_fields_to_obj(&mut obj, typed.as_ref())?,
        api::Body::Raw(_) => {}
    }

    Ok(obj.into_unknown())
}

pub fn encode_auth_payload(env: Env, p: &auth::AuthPayload) -> napi::Result<JsUnknown> {
    let mut obj = env.create_object()?;
    add_fields_to_obj(&mut obj, p.query.as_ref())?;
    add_fields_to_obj(&mut obj, p.header.as_ref())?;
    Ok(obj.into_unknown())
}

pub fn pvalues_or_null(env: Env, vals: Option<&PValues>) -> napi::Result<JsUnknown> {
    vals.map_or_else(
        || env.get_null().map(|v| v.into_unknown()),
        |v| pvalues_to_js(env, v),
    )
}

fn add_fields_to_obj<'a, I: IntoIterator<Item = (&'a String, &'a PValue)>>(
    obj: &'a mut JsObject,
    vals: Option<I>,
) -> napi::Result<()> {
    let Some(vals) = vals else {
        return Ok(());
    };

    for (k, v) in vals.into_iter() {
        let pv = unsafe { std::mem::transmute::<&PValue, &PVal>(v) };
        obj.set(k, pv)?;
    }
    Ok(())
}

fn timestamp_to_dt(millis: f64) -> chrono::DateTime<chrono::FixedOffset> {
    let millis = millis.trunc() as i64;
    let secs = millis / 1000;
    let nanos = (millis % 1000) * 1_000_000;
    let ts = chrono::Utc.timestamp_opt(secs, nanos as u32);
    ts.unwrap().fixed_offset()
}

// This is an inlined version of JsObject::get that distinguishes between null and undefined.
fn obj_get_pval<K: AsRef<str>>(
    env: sys::napi_env,
    obj: sys::napi_value,
    field: K,
) -> Result<Option<PValue>> {
    let c_field = std::ffi::CString::new(field.as_ref())?;

    unsafe {
        let mut ret = std::ptr::null_mut();

        check_status!(
            sys::napi_get_named_property(env, obj, c_field.as_ptr(), &mut ret),
            "Failed to get property with field `{}`",
            field.as_ref(),
        )?;

        let ty = type_of!(env, ret)?;

        if ty == ValueType::Undefined {
            return Ok(None);
        }

        let pval = PVal::from_napi_value(env, ret)?;
        Ok(Some(pval.0))
    }
}
