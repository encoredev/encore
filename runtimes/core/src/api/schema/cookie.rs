use http::{
    header::{COOKIE, SET_COOKIE},
    HeaderValue,
};

use crate::api::{self, jsonschema, schema::ToResponse, APIResult, PValue, PValues};

use super::{HTTPHeaders, JSONPayload, ToHeaderStr, ToOutgoingRequest};

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
        headers
            .get_all(COOKIE)
            .iter()
            .filter_map(|raw| raw.to_str().ok())
            .flat_map(cookie::Cookie::split_parse)
            .flatten()
            .for_each(|c| jar.add_original(c.into_owned()));

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
        headers
            .get_all(SET_COOKIE)
            .iter()
            .filter_map(|raw| raw.to_str().ok())
            .flat_map(cookie::Cookie::parse)
            .for_each(|c| jar.add_original(c.into_owned()));

        match self.schema.parse(jar) {
            Ok(decoded) => Ok(Some(decoded)),
            Err(err) => Err(err),
        }
    }
}

impl ToOutgoingRequest<http::HeaderMap> for Cookie {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        headers: &mut http::HeaderMap,
    ) -> APIResult<()> {
        if self.schema.root().is_empty() {
            return Ok(());
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

        for (key, field) in self.schema.root().fields.iter() {
            let Some(PValue::Cookie(c)) = payload.get(key) else {
                if field.optional {
                    continue;
                }
                return Err(api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: format!("missing cookie parameter: {key}"),
                    internal_message: Some(format!("missing cookie parameter: {key}")),
                    stack: None,
                    details: None,
                });
            };

            let header_value =
                HeaderValue::from_str(&c.to_string()).map_err(api::Error::internal)?;

            headers.append(http::header::COOKIE, header_value);
        }

        Ok(())
    }
}

impl ToOutgoingRequest<reqwest::Request> for Cookie {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
    ) -> APIResult<()> {
        let headers = req.headers_mut();
        self.to_outgoing_request(payload, headers)
    }
}

impl<B> ToOutgoingRequest<http::Request<B>> for Cookie {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut http::Request<B>,
    ) -> APIResult<()> {
        let headers = req.headers_mut();
        self.to_outgoing_request(payload, headers)
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
            if !schema.fields.contains_key(key) {
                continue;
            };
            let PValue::Cookie(c) = value else {
                continue;
            };
            resp = resp.header(SET_COOKIE, c.to_string());
        }
        Ok(resp)
    }
}
