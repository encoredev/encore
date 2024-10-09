use axum::http::HeaderValue;

use crate::api;
use crate::api::jsonschema::DecodeConfig;
use crate::api::schema::{JSONPayload, ToOutgoingRequest};
use crate::api::{jsonschema, APIResult};
use http_body_util::BodyExt;
use reqwest::header::CONTENT_TYPE;

#[derive(Debug, Clone)]
pub struct Body {
    schema: jsonschema::JSONSchema,
}

impl Body {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        Self { schema }
    }

    pub async fn parse_incoming_request_body(
        &self,
        body: axum::body::Body,
    ) -> APIResult<Option<serde_json::Map<String, serde_json::Value>>> {
        // Collect the bytes of the request body.
        // Serde doesn't support async streaming reads (at least not yet).
        let bytes = body
            .collect()
            .await
            .map_err(|e| api::Error::invalid_argument("unable to read request body", e))?
            .to_bytes();

        let mut jsonde = serde_json::Deserializer::from_slice(&bytes);
        let cfg = DecodeConfig {
            coerce_strings: false,
        };
        let value = self
            .schema
            .deserialize(&mut jsonde, cfg)
            .map_err(|e| api::Error::invalid_argument("unable to decode request body", e))?;
        Ok(Some(value))
    }
}

impl ToOutgoingRequest<reqwest::Request> for Body {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
    ) -> APIResult<()> {
        if payload.is_none() {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing body payload".to_string(),
                internal_message: None,
                stack: None,
            });
        };

        let body = self.schema.to_vec(payload).map_err(api::Error::internal)?;
        if !req.headers().contains_key(CONTENT_TYPE) {
            req.headers_mut().insert(
                CONTENT_TYPE,
                reqwest::header::HeaderValue::from_static("application/json"),
            );
        }
        *req.body_mut() = Some(body.into());
        Ok(())
    }
}

impl Body {
    pub fn to_outgoing_payload(&self, payload: &JSONPayload) -> APIResult<Vec<u8>> {
        if payload.is_none() {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing body payload".to_string(),
                internal_message: None,
                stack: None,
            });
        };

        let body = self.schema.to_vec(payload).map_err(api::Error::internal)?;
        Ok(body)
    }
    pub fn to_response(
        &self,
        payload: &JSONPayload,
        resp: axum::http::response::Builder,
    ) -> APIResult<axum::http::Response<axum::body::Body>> {
        let buf = self.schema.to_vec(payload).map_err(api::Error::internal)?;
        let resp = resp
            .header(
                axum::http::header::CONTENT_TYPE,
                HeaderValue::from_static(mime::APPLICATION_JSON.as_ref()),
            )
            .body(axum::body::Body::from(buf))
            .map_err(api::Error::internal)?;

        Ok(resp)
    }
}
