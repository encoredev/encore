//! Conversions between JavaScript values and BSON for the MongoDB bindings.
//!
//! Documents are converted directly, not through `PValue`, because `PValue`
//! stores object keys in a sorted map, and MongoDB depends on key order:
//! in compound index keys, in multi-field sorts, and in stored documents.
//!
//! The rules follow the official Node.js driver's defaults: integers that
//! fit in 32 bits become Int32, other numbers Double, `Date` becomes a BSON
//! date, `Buffer` binary, `bigint` Int64, and `undefined` fields are skipped.

use std::cell::Cell;

use encore_runtime_core::mongodb::bson::{self, spec::BinarySubtype, Bson, Document};
use napi::bindgen_prelude::*;
use napi::{sys, Env, JsDate, JsObject, NapiRaw, ValueType};

/// A JavaScript object converted to a BSON document.
pub struct BsonDoc(pub Document);

/// A BSON value converted to a JavaScript value.
pub struct BsonOut(pub Bson);

// Matches PVal's limit: objects can't nest deeper than this.
const MAX_DEPTH: usize = 128;

thread_local! {
    static DEPTH: Cell<usize> = const { Cell::new(0) };
}

struct DepthGuard;

impl DepthGuard {
    fn enter() -> Result<Self> {
        DEPTH.with(|d| {
            if d.get() >= MAX_DEPTH {
                return Err(Error::new(
                    Status::InvalidArg,
                    "document is nested too deeply".to_owned(),
                ));
            }
            d.set(d.get() + 1);
            Ok(DepthGuard)
        })
    }
}

impl Drop for DepthGuard {
    fn drop(&mut self) {
        DEPTH.with(|d| d.set(d.get() - 1));
    }
}

impl FromNapiValue for BsonDoc {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        match unsafe { js_to_bson(env, napi_val)? } {
            Some(Bson::Document(doc)) => Ok(BsonDoc(doc)),
            _ => Err(Error::new(
                Status::InvalidArg,
                "expected an object".to_owned(),
            )),
        }
    }
}

/// Converts a JavaScript value to BSON. It returns None for `undefined`,
/// so that object fields set to `undefined` are left out.
unsafe fn js_to_bson(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Option<Bson>> {
    let _guard = DepthGuard::enter()?;
    let ty = type_of!(env, napi_val)?;
    Ok(Some(match ty {
        ValueType::Undefined => return Ok(None),
        ValueType::Null => Bson::Null,
        ValueType::Boolean => Bson::Boolean(unsafe { bool::from_napi_value(env, napi_val)? }),
        ValueType::String => Bson::String(unsafe { String::from_napi_value(env, napi_val)? }),
        ValueType::Number => {
            let n = unsafe { f64::from_napi_value(env, napi_val)? };
            if n.fract() == 0.0 && n >= i32::MIN as f64 && n <= i32::MAX as f64 {
                Bson::Int32(n as i32)
            } else {
                Bson::Double(n)
            }
        }
        ValueType::BigInt => {
            let bi = unsafe { BigInt::from_napi_value(env, napi_val)? };
            let (n, lossless) = bi.get_i64();
            if !lossless {
                return Err(Error::new(
                    Status::InvalidArg,
                    "bigint does not fit in a 64-bit integer".to_owned(),
                ));
            }
            Bson::Int64(n)
        }
        ValueType::Object => unsafe { object_to_bson(env, napi_val)? },
        other => {
            return Err(Error::new(
                Status::InvalidArg,
                format!("cannot store a value of type {other} in MongoDB"),
            ))
        }
    }))
}

unsafe fn object_to_bson(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Bson> {
    let mut is_array = false;
    check_status!(unsafe { sys::napi_is_array(env, napi_val, &mut is_array) })?;
    if is_array {
        let obj = unsafe { JsObject::from_napi_value(env, napi_val)? };
        let len = obj.get_array_length()?;
        let mut arr = Vec::with_capacity(len as usize);
        for i in 0..len {
            let item: napi::JsUnknown = obj.get_element(i)?;
            // undefined in an array becomes null, like the Node.js driver.
            arr.push(unsafe { js_to_bson(env, item.raw())? }.unwrap_or(Bson::Null));
        }
        return Ok(Bson::Array(arr));
    }

    let mut is_date = false;
    check_status!(unsafe { sys::napi_is_date(env, napi_val, &mut is_date) })?;
    if is_date {
        let millis = unsafe { JsDate::from_napi_value(env, napi_val)? }.value_of()?;
        return Ok(Bson::DateTime(bson::DateTime::from_millis(millis as i64)));
    }

    let mut is_buffer = false;
    check_status!(unsafe { sys::napi_is_buffer(env, napi_val, &mut is_buffer) })?;
    if is_buffer {
        let buf = unsafe { Buffer::from_napi_value(env, napi_val)? };
        return Ok(Bson::Binary(bson::Binary {
            subtype: BinarySubtype::Generic,
            bytes: buf.to_vec(),
        }));
    }

    let obj = unsafe { JsObject::from_napi_value(env, napi_val)? };

    // Encore's Decimal type (encore.dev/types).
    if obj.has_property("__encore_decimal")? {
        let value: String = obj.get("value")?.unwrap_or_default();
        let d = value
            .parse::<bson::Decimal128>()
            .map_err(|e| Error::new(Status::InvalidArg, format!("invalid decimal {value}: {e}")))?;
        return Ok(Bson::Decimal128(d));
    }

    // A plain object: keep the keys in their JavaScript order.
    let mut doc = Document::new();
    for key in JsObject::keys(&obj)? {
        let val: napi::JsUnknown = obj.get_named_property(&key)?;
        if let Some(v) = unsafe { js_to_bson(env, val.raw())? } {
            doc.insert(key, v);
        }
    }
    Ok(Bson::Document(doc))
}

impl FromNapiValue for BsonOut {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        Ok(BsonOut(
            unsafe { js_to_bson(env, napi_val)? }.unwrap_or(Bson::Null),
        ))
    }
}

impl ToNapiValue for BsonOut {
    unsafe fn to_napi_value(raw_env: sys::napi_env, val: Self) -> Result<sys::napi_value> {
        let env = Env::from_raw(raw_env);
        let v = bson_to_js(&env, val.0)?;
        Ok(v.raw())
    }
}

/// Converts a BSON value to a JavaScript value, keeping document key order.
fn bson_to_js(env: &Env, val: Bson) -> Result<napi::JsUnknown> {
    Ok(match val {
        Bson::Null | Bson::Undefined => env.get_null()?.into_unknown(),
        Bson::Boolean(b) => env.get_boolean(b)?.into_unknown(),
        Bson::Int32(i) => env.create_int32(i)?.into_unknown(),
        Bson::Int64(i) => env.create_int64(i)?.into_unknown(),
        Bson::Double(f) => env.create_double(f)?.into_unknown(),
        Bson::String(s) => env.create_string(&s)?.into_unknown(),
        Bson::DateTime(dt) => env
            .create_date(dt.timestamp_millis() as f64)?
            .into_unknown(),
        // No JavaScript equivalent without the Node.js driver's classes:
        // an ObjectId becomes its hex string, a Decimal128 its decimal string.
        Bson::ObjectId(oid) => env.create_string(&oid.to_hex())?.into_unknown(),
        Bson::Decimal128(d) => env.create_string(&d.to_string())?.into_unknown(),
        Bson::Binary(b) => env
            .create_buffer_with_data(b.bytes)?
            .into_raw()
            .into_unknown(),
        Bson::Array(arr) => {
            let mut out = env.create_array_with_length(arr.len())?;
            for (i, v) in arr.into_iter().enumerate() {
                out.set_element(i as u32, bson_to_js(env, v)?)?;
            }
            out.into_unknown()
        }
        Bson::Document(doc) => {
            let mut out = env.create_object()?;
            for (k, v) in doc {
                out.set_named_property(&k, bson_to_js(env, v)?)?;
            }
            out.into_unknown()
        }
        other => env.create_string(&other.to_string())?.into_unknown(),
    })
}
