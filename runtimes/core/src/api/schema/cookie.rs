use http::header::{COOKIE, SET_COOKIE};

use crate::api::{self, jsonschema, schema::ToResponse, APIResult, PValue, PValues};

#[derive(Debug, Clone)]
pub struct Cookie {
    schema: jsonschema::JSONSchema,
}

impl Cookie {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        Self { schema }
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
            for raw_part in String::from_utf8_lossy(raw.as_bytes()).split(';') {
                let c = cookie::Cookie::parse(raw_part).map_err(api::Error::internal)?;
                jar.add_original(c.into_owned());
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
            for raw_part in String::from_utf8_lossy(raw.as_bytes()).split(';') {
                let c = cookie::Cookie::parse(raw_part).map_err(api::Error::internal)?;
                jar.add_original(c.into_owned());
            }
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
            let value = to_cookie_value(value)?;

            let c = cookie::Cookie::new(name, value);
            resp = resp.header(SET_COOKIE, c.to_string());
        }

        Ok(resp)
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
    }
}
