use http::header::COOKIE;

use crate::api::{self, jsonschema, APIResult, PValues};

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
        self.parse(&req.headers)
    }

    pub fn parse(&self, headers: &axum::http::HeaderMap) -> APIResult<Option<PValues>> {
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
}
