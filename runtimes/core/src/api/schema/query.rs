use std::str::FromStr;

use crate::api;
use crate::api::jsonschema::DecodeConfig;
use crate::api::schema::{JSONPayload, ToOutgoingRequest};
use crate::api::{jsonschema, APIResult};
use serde::Serialize;
use url::Url;

#[derive(Debug, Clone)]
pub struct Query {
    schema: jsonschema::JSONSchema,
}

impl Query {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        Self { schema }
    }

    pub fn parse_incoming_request_parts(
        &self,
        req: &axum::http::request::Parts,
    ) -> APIResult<Option<serde_json::Map<String, serde_json::Value>>> {
        self.parse(req.uri.query())
    }

    pub fn parse(
        &self,
        query: Option<&str>,
    ) -> APIResult<Option<serde_json::Map<String, serde_json::Value>>> {
        let parsed = form_urlencoded::parse(query.unwrap_or_default().as_bytes());
        let de = serde_urlencoded::Deserializer::new(parsed);
        let cfg = DecodeConfig {
            coerce_strings: true,
        };

        let decoded = self
            .schema
            .deserialize(de, cfg)
            .map_err(|e| api::Error::invalid_argument("unable to decode query string", e))?;

        Ok(Some(decoded))
    }

    pub fn contains_name(&self, name: &str) -> bool {
        self.schema.root().contains_name(name)
    }

    pub fn contains_any(&self, query: &[u8]) -> bool {
        let schema = &self.schema.root();
        if schema.is_empty() {
            return false;
        }

        let parsed = form_urlencoded::parse(query);
        for (key, val) in parsed {
            // Only consider non-empty values to be present.
            if !val.is_empty() && schema.contains_name(key.as_ref()) {
                return true;
            }
        }

        false
    }

    pub fn len(&self) -> usize {
        self.schema.root().fields.len()
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn fields(&self) -> impl Iterator<Item = (&String, &jsonschema::Field)> {
        self.schema.root().fields.iter()
    }
}

impl ToOutgoingRequest<http::Request<()>> for Query {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut http::Request<()>,
    ) -> APIResult<()> {
        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing query parameters".to_string(),
                internal_message: Some("missing query parameters".to_string()),
                stack: None,
            });
        };

        // Serialize the payload.
        let mut url = Url::parse(&req.uri().to_string()).map_err(api::Error::internal)?;

        {
            let mut pairs = url.query_pairs_mut();
            let serializer = serde_urlencoded::Serializer::new(&mut pairs);

            payload
                .serialize(serializer)
                .map_err(api::Error::internal)?;
        }

        *req.uri_mut() = http::Uri::from_str(url.as_str()).map_err(api::Error::internal)?;

        Ok(())
    }
}

impl ToOutgoingRequest<reqwest::Request> for Query {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
    ) -> APIResult<()> {
        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing query parameters".to_string(),
                internal_message: Some("missing query parameters".to_string()),
                stack: None,
            });
        };

        // Serialize the payload.
        {
            let url = req.url_mut();
            let mut pairs = url.query_pairs_mut();
            let serializer = serde_urlencoded::Serializer::new(&mut pairs);

            payload
                .serialize(serializer)
                .map_err(api::Error::internal)?;
        }

        // If the query string is now empty, set it to None.
        if let Some("") = req.url().query() {
            req.url_mut().set_query(None);
        }

        Ok(())
    }
}
