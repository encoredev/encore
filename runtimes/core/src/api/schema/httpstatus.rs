use crate::api::{APIResult, PValues};
use axum::http::response::Builder as ResponseBuilder;

/// HTTP status code parameter specification.
#[derive(Debug, Clone)]
pub struct HttpStatus {
    pub name: String,
}

impl HttpStatus {
    pub fn new(name: String) -> Self {
        Self { name }
    }

    /// Extract HTTP status code from response payload and apply it to the response builder.
    pub fn to_response(
        &self,
        payload: &Option<PValues>,
        mut resp_builder: ResponseBuilder,
    ) -> APIResult<ResponseBuilder> {
        if let Some(payload) = payload {
            if let Some(status_value) = payload.get(&self.name) {
                let status_code = self.extract_status_code(status_value)?;
                resp_builder = resp_builder.status(status_code);
            }
        }
        Ok(resp_builder)
    }

    /// Extract and validate HTTP status code from a PValue.
    fn extract_status_code(&self, status_value: &crate::api::PValue) -> APIResult<u16> {
        let status_code = match status_value {
            crate::api::PValue::Number(n) => n.as_u64().ok_or_else(|| {
                crate::api::Error::invalid_argument(
                    "invalid http status code",
                    anyhow::anyhow!(
                        "HTTP status field '{}' must be a positive integer",
                        self.name
                    ),
                )
            })?,
            _ => {
                return Err(crate::api::Error::invalid_argument(
                    "invalid http status code",
                    anyhow::anyhow!(
                        "HTTP status field '{}' must be a number, got: {}",
                        self.name,
                        status_value.type_name()
                    ),
                ));
            }
        };

        if !(100..=599).contains(&status_code) {
            return Err(crate::api::Error::invalid_argument(
                "invalid http status code",
                anyhow::anyhow!(
                    "HTTP status field '{}' must be a between 100 and 599, got: {status_code}",
                    self.name,
                ),
            ));
        }

        Ok(status_code as u16)
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
