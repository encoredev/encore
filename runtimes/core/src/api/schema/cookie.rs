use cookie::time::OffsetDateTime;
use http::header::{COOKIE, SET_COOKIE};

use crate::api::{self, jsonschema, schema::ToResponse, APIResult, DateTime, PValue, PValues};

use super::{HTTPHeaders, ToHeaderStr};

#[derive(Debug, Clone)]
pub struct Cookie {
    schema: jsonschema::JSONSchema,
}

impl Cookie {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        Self { schema }
    }

    pub fn contains_any(&self, headers: &impl HTTPHeaders) -> bool {
        let mut jar = cookie::CookieJar::new();
        for raw in headers.get_all(axum::http::header::COOKIE.as_str()) {
            if let Ok(raw) = raw.to_str() {
                for c in cookie::Cookie::split_parse(raw).flatten() {
                    jar.add_original(c.into_owned());
                }
            }
        }

        for (name, field) in self.schema.root().fields.iter() {
            let cookie_name = field.name_override.as_deref().unwrap_or(name.as_str());

            if let Some(c) = jar.get(cookie_name) {
                // Only consider non-empty values to be present.
                if !c.value().is_empty() {
                    return true;
                }
            }
        }
        false
    }

    pub fn fields(&self) -> impl Iterator<Item = (&String, &jsonschema::Field)> {
        self.schema.root().fields.iter()
    }

    pub fn parse_incoming_request_parts(
        &self,
        req: &axum::http::request::Parts,
    ) -> APIResult<Option<PValues>> {
        self.parse_req(&req.headers)
    }

    pub fn parse_req(&self, headers: &axum::http::HeaderMap) -> APIResult<Option<PValues>> {
        if self.schema.root().is_empty() {
            return Ok(None);
        }

        let mut jar = cookie::CookieJar::new();
        for raw in headers.get_all(COOKIE) {
            if let Ok(raw) = raw.to_str() {
                for c in cookie::Cookie::split_parse(raw).flatten() {
                    jar.add_original(c.into_owned());
                }
            }
        }

        match self.schema.parse(jar) {
            Ok(decoded) => Ok(Some(decoded)),
            Err(err) => Err(err),
        }
    }

    pub fn parse_resp(&self, headers: &axum::http::HeaderMap) -> APIResult<Option<PValues>> {
        if self.schema.root().is_empty() {
            return Ok(None);
        }

        let mut jar = cookie::CookieJar::new();
        for raw in headers.get_all(SET_COOKIE) {
            let c = cookie::Cookie::parse(String::from_utf8_lossy(raw.as_bytes()))
                .map_err(api::Error::internal)?;
            jar.add_original(c.into_owned());
        }

        match self.schema.parse(jar) {
            Ok(decoded) => Ok(Some(decoded)),
            Err(err) => Err(err),
        }
    }
}

impl ToResponse for Cookie {
    fn to_response(
        &self,
        payload: &super::JSONPayload,
        mut resp: axum::http::response::Builder,
    ) -> APIResult<axum::http::response::Builder> {
        if self.schema.root().is_empty() {
            return Ok(resp);
        }

        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing cookie parameters".to_string(),
                internal_message: Some("missing cookie parameters".to_string()),
                stack: None,
                details: None,
            });
        };

        let schema = self.schema.root();
        for (key, value) in payload.iter() {
            let key = key.as_str();
            let Some(field) = schema.fields.get(key) else {
                continue;
            };

            let name = field.name_override.as_deref().unwrap_or(key);
            let c = to_cookie(name, value)?;
            resp = resp.header(SET_COOKIE, c.to_string());
        }

        Ok(resp)
    }
}

struct RawCookie<'a> {
    inner: &'a PValues,
}

impl<'a> RawCookie<'a> {
    fn new(inner: &'a PValues) -> Self {
        Self { inner }
    }

    fn value(&self) -> APIResult<String> {
        to_cookie_value(self.inner.get("value").ok_or(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "missing cookie value".to_string(),
            internal_message: None,
            stack: None,
            details: None,
        })?)
    }

    fn max_age(&self) -> APIResult<Option<cookie::time::Duration>> {
        match self.inner.get("maxAge") {
            Some(PValue::Number(nr)) => {
                let dur = if nr.is_i64() {
                    cookie::time::Duration::seconds(nr.as_i64().unwrap())
                } else if nr.is_u64() {
                    cookie::time::Duration::seconds(i64::MAX)
                } else if nr.is_f64() {
                    cookie::time::Duration::seconds_f64(nr.as_f64().unwrap())
                } else {
                    return Err(api::Error {
                        code: api::ErrCode::InvalidArgument,
                        message: format!("unable to parse duration for maxAge, value: {}", nr),
                        internal_message: None,
                        stack: None,
                        details: None,
                    });
                };
                Ok(Some(dur))
            }
            None => Ok(None),
            Some(v) => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!(
                    "invalid type for cookie maxAge field, expected number, got {}",
                    v.type_name()
                ),
                internal_message: None,
                stack: None,
                details: None,
            }),
        }
    }

    fn expires(&self) -> APIResult<Option<DateTime>> {
        match self.inner.get("expires") {
            Some(PValue::DateTime(dt)) => Ok(Some(*dt)),
            None => Ok(None),
            Some(v) => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!(
                    "invalid type for cookie expires field, expected datetime, got {}",
                    v.type_name()
                ),
                internal_message: None,
                stack: None,
                details: None,
            }),
        }
    }

    fn secure(&self) -> APIResult<Option<bool>> {
        self.get_bool("secure")
    }

    fn http_only(&self) -> APIResult<Option<bool>> {
        self.get_bool("http_only")
    }

    fn partitioned(&self) -> APIResult<Option<bool>> {
        self.get_bool("partitioned")
    }

    fn same_site(&self) -> APIResult<Option<cookie::SameSite>> {
        self.get_string("sameSite")?
            .map_or(Ok(None), |same_site| match same_site.as_str() {
                "Strict" => Ok(Some(cookie::SameSite::Strict)),
                "Lax" => Ok(Some(cookie::SameSite::Lax)),
                "None" => Ok(Some(cookie::SameSite::None)),
                _ => Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("invalid sameSite value: {}", same_site),
                    internal_message: None,
                    stack: None,
                    details: None,
                }),
            })
    }

    fn domain(&self) -> APIResult<Option<String>> {
        self.get_string("domain")
    }

    fn path(&self) -> APIResult<Option<String>> {
        self.get_string("path")
    }

    fn get_bool(&self, key: &str) -> APIResult<Option<bool>> {
        match self.inner.get(key) {
            Some(PValue::Bool(b)) => Ok(Some(*b)),
            None => Ok(None),
            Some(v) => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!(
                    "invalid type for cookie {} field, expected bool, got {}",
                    key,
                    v.type_name()
                ),
                internal_message: None,
                stack: None,
                details: None,
            }),
        }
    }

    fn get_string(&self, key: &str) -> APIResult<Option<String>> {
        match self.inner.get(key) {
            Some(PValue::String(s)) => Ok(Some(s.clone())),
            None => Ok(None),
            Some(v) => Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: format!(
                    "invalid type for cookie {} field, expected string, got {}",
                    key,
                    v.type_name()
                ),
                internal_message: None,
                stack: None,
                details: None,
            }),
        }
    }
}

fn to_cookie(name: &str, value: &PValue) -> APIResult<cookie::Cookie<'static>> {
    if let PValue::Object(obj) = value {
        let cookie = RawCookie::new(obj);
        let value = cookie.value()?;
        let mut cb = cookie::CookieBuilder::new(name.to_string(), value);

        if let Some(dt) = cookie.expires()? {
            let system_time: std::time::SystemTime = dt.into();
            let expire = OffsetDateTime::from(system_time);
            cb = cb.expires(expire);
        }
        if let Some(same_site) = cookie.same_site()? {
            cb = cb.same_site(same_site);
        }
        if let Some(domain) = cookie.domain()? {
            cb = cb.domain(domain);
        }
        if let Some(path) = cookie.path()? {
            cb = cb.path(path);
        }
        if let Some(max_age) = cookie.max_age()? {
            cb = cb.max_age(max_age);
        }
        if let Some(secure) = cookie.secure()? {
            cb = cb.secure(secure);
        }
        if let Some(http_only) = cookie.http_only()? {
            cb = cb.http_only(http_only);
        }
        if let Some(partitioned) = cookie.partitioned()? {
            cb = cb.partitioned(partitioned);
        }

        Ok(cb.build())
    } else {
        Ok(cookie::Cookie::new(
            name.to_string(),
            to_cookie_value(value)?,
        ))
    }
}

fn to_cookie_value(value: &PValue) -> APIResult<String> {
    use PValue::*;

    match value {
        Null => Ok("null".to_string()),
        Bool(b) => Ok(b.to_string()),
        String(s) => Ok(s.clone()),
        Number(n) => Ok(n.to_string()),
        DateTime(dt) => Ok(dt.to_rfc3339()),
        Array(_) => Err(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "array type unsupported as cookie value".to_string(),
            internal_message: None,
            stack: None,
            details: None,
        }),
        Object(_) => Err(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "map type unsupported as cookie value".to_string(),
            internal_message: None,
            stack: None,
            details: None,
        }),
        Cookie(_) => Err(api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "cookie type unsupported as cookie value".to_string(),
            internal_message: None,
            stack: None,
            details: None,
        }),
    }
}
