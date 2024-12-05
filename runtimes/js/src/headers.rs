use axum::http::{HeaderMap, HeaderName, HeaderValue};
use napi::{
    bindgen_prelude::{assert_type_of, check_status, type_of, FromNapiValue},
    sys, Error, JsObject, JsUnknown, Result, ValueType,
};

pub fn parse_header_map(val: JsUnknown) -> Result<Option<HeaderMap>> {
    if val
        .get_type()
        .is_ok_and(|t| matches!(t, napi::ValueType::Undefined | napi::ValueType::Null))
    {
        return Ok(None);
    }

    let val = WrappedHeaderMap::from_unknown(val)?;
    Ok(Some(val.0))
}

pub struct WrappedHeaderMap(pub HeaderMap);

impl FromNapiValue for WrappedHeaderMap {
    unsafe fn from_napi_value(env: sys::napi_env, napi_val: sys::napi_value) -> Result<Self> {
        assert_type_of!(env, napi_val, ValueType::Object)?;
        let obj = JsObject::from_napi_value(env, napi_val)?;

        let mut map = WrappedHeaderMap(HeaderMap::new());
        for key in JsObject::keys(&obj)?.into_iter() {
            if let Some(vals) = obj_get_header_val(env, napi_val, &key)? {
                let hname = HeaderName::from_bytes(key.as_bytes()).map_err(|e| {
                    Error::new(
                        napi::Status::InvalidArg,
                        format!("invalid header name: {e}"),
                    )
                })?;

                for val in vals {
                    let hval = HeaderValue::from_bytes(val.as_bytes()).map_err(|e| {
                        Error::new(
                            napi::Status::InvalidArg,
                            format!("invalid header value: {e}"),
                        )
                    })?;

                    map.0.append(&hname, hval);
                }
            }
        }

        Ok(map)
    }
}

fn obj_get_header_val<K: AsRef<str>>(
    env: sys::napi_env,
    obj: sys::napi_value,
    field: K,
) -> Result<Option<Vec<String>>> {
    let c_field = std::ffi::CString::new(field.as_ref())?;

    unsafe {
        let mut ret = std::ptr::null_mut();

        check_status!(
            sys::napi_get_named_property(env, obj, c_field.as_ptr(), &mut ret),
            "Failed to get property with field `{}`",
            field.as_ref()
        )?;

        let ty = type_of!(env, ret)?;

        match ty {
            ValueType::Undefined => Ok(None),
            ValueType::String => {
                let val = String::from_napi_value(env, ret)?;
                Ok(Some(vec![val]))
            }
            ValueType::Object => {
                let mut is_arr = false;
                check_status!(
                    sys::napi_is_array(env, ret, &mut is_arr),
                    "Failed to detect whether given js is an array"
                )?;

                if is_arr {
                    let vals = Vec::<String>::from_napi_value(env, ret).map_err(|_e| {
                        Error::new(
                            napi::Status::InvalidArg,
                            "unable to parse header values array as strings",
                        )
                    })?;

                    Ok(Some(vals))
                } else {
                    Err(Error::new(
                        napi::Status::InvalidArg,
                        "invalid header value type",
                    ))
                }
            }
            _ => Err(Error::new(
                napi::Status::InvalidArg,
                "header map value must be string or array of strings",
            )),
        }
    }
}
