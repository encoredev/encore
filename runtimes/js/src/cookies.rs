use crate::pvalue::PVal;

use encore_runtime_core::api::{self, Cookie, DateTime, PValue, PValues};
use napi::{bindgen_prelude::*, JsObject, Result};

// Helper struct to parse a PValue::Object from javascript into a PValue::Cookie
pub(crate) struct JsCookie;

impl JsCookie {
    fn get_bool(vals: &PValues, key: &str) -> Result<Option<bool>> {
        match vals.get(key) {
            None => Ok(None),
            Some(PValue::Bool(b)) => Ok(Some(*b)),
            Some(_) => Err(Error::new(
                Status::InvalidArg,
                format!("cookie field {key} must be a boolean"),
            )),
        }
    }
    fn get_string(vals: &PValues, key: &str) -> Result<Option<String>> {
        match vals.get(key) {
            None => Ok(None),
            Some(PValue::String(s)) => Ok(Some(s.clone())),
            Some(_) => Err(Error::new(
                Status::InvalidArg,
                format!("cookie field {key} must be a string"),
            )),
        }
    }
    fn get_datetime(vals: &PValues, key: &str) -> Result<Option<DateTime>> {
        match vals.get(key) {
            None => Ok(None),
            Some(PValue::DateTime(d)) => Ok(Some(*d)),
            Some(_) => Err(Error::new(
                Status::InvalidArg,
                format!("cookie field {key} must be a datetime"),
            )),
        }
    }
    fn get_max_age(vals: &PValues, key: &str) -> Result<Option<u64>> {
        match vals.get(key) {
            None => Ok(None),
            Some(PValue::Number(n)) => {
                if n.is_i64() {
                    let n = n.as_i64().unwrap();
                    if n < 0 {
                        Ok(Some(0u64))
                    } else {
                        Ok(Some(n as u64))
                    }
                } else if n.is_u64() {
                    Ok(Some(n.as_u64().unwrap()))
                } else {
                    Err(Error::new(
                        Status::InvalidArg,
                        "cookie field {} must be an integer",
                    ))
                }
            }
            Some(_) => Err(Error::new(
                Status::InvalidArg,
                format!("cookie field {key} must be an integer"),
            )),
        }
    }

    pub fn parse_cookie(obj: &PValues, name: &str, value: &PValue) -> Result<Cookie> {
        Ok(Cookie {
            name: name.to_string(),
            value: Box::new(value.clone()),
            path: Self::get_string(obj, "path")?,
            domain: Self::get_string(obj, "domain")?,
            secure: Self::get_bool(obj, "secure")?,
            http_only: Self::get_bool(obj, "httpOnly")?,
            expires: Self::get_datetime(obj, "expires")?,
            max_age: Self::get_max_age(obj, "maxAge")?,
            same_site: Self::get_string(obj, "sameSite")?
                .map(|s| match s.as_str() {
                    "Lax" => Ok(api::SameSite::Lax),
                    "Strict" => Ok(api::SameSite::Strict),
                    "None" => Ok(api::SameSite::None),
                    _ => Err(Error::new(
                        Status::InvalidArg,
                        "cookie field sameSite must be one of Lax, Strict or None",
                    )),
                })
                .transpose()?,
            partitioned: Self::get_bool(obj, "partitioned")?,
        })
    }
}

pub(crate) unsafe fn cookie_to_napi_value(
    env: sys::napi_env,
    c: api::Cookie,
) -> Result<sys::napi_value> {
    let env2 = Env::from_raw(env);
    let mut cookie = env2.create_object()?;

    cookie.set("name", &c.name)?;
    cookie.set("value", PVal(*c.value))?;

    if let Some(secure) = c.secure {
        cookie.set("secure", secure)?;
    }
    if let Some(http_only) = c.http_only {
        cookie.set("httpOnly", http_only)?;
    }

    if let Some(domain) = &c.domain {
        cookie.set("domain", domain)?;
    }
    if let Some(path) = &c.path {
        cookie.set("path", path)?;
    }
    if let Some(expires) = c.expires {
        cookie.set("expires", PVal(PValue::DateTime(expires)))?;
    }
    if let Some(same_site) = c.same_site {
        cookie.set("sameSite", same_site.to_string())?;
    }
    JsObject::to_napi_value(env, cookie)
}
