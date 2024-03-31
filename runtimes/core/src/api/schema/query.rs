use crate::api;
use crate::api::schema::{JSONPayload, ToOutgoingRequest};
use crate::api::{jsonschema, APIResult};
use serde::de::DeserializeSeed;
use serde::Serialize;

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

        let decoded = DeserializeSeed::deserialize(&self.schema, de)
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
        for (key, _) in parsed {
            if schema.contains_name(key.as_ref()) {
                return true;
            }
        }

        return false;
    }

    pub fn len(&self) -> usize {
        self.schema.root().fields.len()
    }

    pub fn fields(&self) -> impl Iterator<Item = (&String, &jsonschema::Field)> {
        self.schema.root().fields.iter()
    }
}

impl ToOutgoingRequest for Query {
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
