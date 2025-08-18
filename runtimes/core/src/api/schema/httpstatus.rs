use crate::api::{APIResult, PValues};
use axum::http::response::Builder as ResponseBuilder;

/// HTTP status code parameter specification.
#[derive(Debug, Clone)]
pub struct HttpStatus {
    pub src_name: String,
}

impl HttpStatus {
    pub fn new(src_name: String) -> Self {
        Self { src_name }
    }

    /// Extract HTTP status code from response payload and apply it to the response builder.
    pub fn to_response(
        &self,
        payload: &Option<PValues>,
        mut resp_builder: ResponseBuilder,
    ) -> APIResult<ResponseBuilder> {
        if let Some(payload) = payload {
            if let Some(status_value) = payload.get(&self.src_name) {
                // Extract the status code from the PValue
                if let Some(status_code) = match status_value {
                    crate::api::PValue::Number(n) => n.as_u64(),
                    _ => None,
                } {
                    if (100..=599).contains(&status_code) {
                        resp_builder = resp_builder.status(status_code as u16);
                    } else {
                        return Err(crate::api::Error::invalid_argument(format!("invalid HTTP status code: {}. Status code must be between 100 and 599.", status_code), anyhow::anyhow!("invalid HTTP status code")));
                    }
                }
            }
        }
        Ok(resp_builder)
    }
}

impl super::ToResponse for HttpStatus {
    fn to_response(
        &self,
        payload: &super::JSONPayload,
        resp: axum::http::response::Builder,
    ) -> APIResult<axum::http::response::Builder> {
        self.to_response(payload, resp)
    }
}
