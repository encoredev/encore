use crate::api::{ErrCode, Error};
use axum::http::StatusCode;

/// Maps an error and HTTP status code to a human-readable status code string
/// This mirrors the Go implementation's Code function in util.go
pub fn status_code_string(err: Option<&Error>, http_status: StatusCode) -> String {
    if let Some(error) = err {
        // If we have an error, use the error code directly
        return error.code.to_human_readable();
    }

    // If no error, map from HTTP status code
    match http_status {
        StatusCode::OK => "ok".to_string(),
        StatusCode::BAD_REQUEST => "invalid_argument".to_string(),
        StatusCode::UNAUTHORIZED => "unauthenticated".to_string(),
        StatusCode::FORBIDDEN => "permission_denied".to_string(),
        StatusCode::NOT_FOUND => "not_found".to_string(),
        StatusCode::METHOD_NOT_ALLOWED => "invalid_argument".to_string(),
        StatusCode::REQUEST_TIMEOUT => "deadline_exceeded".to_string(),
        StatusCode::CONFLICT => "already_exists".to_string(),
        StatusCode::GONE => "not_found".to_string(),
        StatusCode::PRECONDITION_FAILED => "failed_precondition".to_string(),
        StatusCode::PAYLOAD_TOO_LARGE => "invalid_argument".to_string(),
        StatusCode::UNPROCESSABLE_ENTITY => "invalid_argument".to_string(),
        StatusCode::TOO_MANY_REQUESTS => "resource_exhausted".to_string(),
        StatusCode::INTERNAL_SERVER_ERROR => "internal".to_string(),
        StatusCode::NOT_IMPLEMENTED => "unimplemented".to_string(),
        StatusCode::BAD_GATEWAY => "unavailable".to_string(),
        StatusCode::SERVICE_UNAVAILABLE => "unavailable".to_string(),
        StatusCode::GATEWAY_TIMEOUT => "deadline_exceeded".to_string(),
        _ => "unknown".to_string(),
    }
}

impl ErrCode {
    pub fn to_human_readable(&self) -> String {
        match self {
            ErrCode::Canceled => "canceled".to_string(),
            ErrCode::Unknown => "unknown".to_string(),
            ErrCode::InvalidArgument => "invalid_argument".to_string(),
            ErrCode::DeadlineExceeded => "deadline_exceeded".to_string(),
            ErrCode::NotFound => "not_found".to_string(),
            ErrCode::AlreadyExists => "already_exists".to_string(),
            ErrCode::PermissionDenied => "permission_denied".to_string(),
            ErrCode::ResourceExhausted => "resource_exhausted".to_string(),
            ErrCode::FailedPrecondition => "failed_precondition".to_string(),
            ErrCode::Aborted => "aborted".to_string(),
            ErrCode::OutOfRange => "out_of_range".to_string(),
            ErrCode::Unimplemented => "unimplemented".to_string(),
            ErrCode::Internal => "internal".to_string(),
            ErrCode::Unavailable => "unavailable".to_string(),
            ErrCode::DataLoss => "data_loss".to_string(),
            ErrCode::Unauthenticated => "unauthenticated".to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_status_code_mapping() {
        // Test successful case
        assert_eq!(
            status_code_string(None, StatusCode::OK),
            "ok"
        );

        // Test error cases
        assert_eq!(
            status_code_string(None, StatusCode::NOT_FOUND),
            "not_found"
        );

        assert_eq!(
            status_code_string(None, StatusCode::INTERNAL_SERVER_ERROR),
            "internal"
        );

        // Test unknown status code
        assert_eq!(
            status_code_string(None, StatusCode::IM_A_TEAPOT),
            "unknown"
        );
    }

    #[test]
    fn test_error_code_to_string() {
        assert_eq!(ErrCode::Canceled.to_human_readable(), "canceled");
        assert_eq!(ErrCode::InvalidArgument.to_human_readable(), "invalid_argument");
        assert_eq!(ErrCode::Internal.to_human_readable(), "internal");
        assert_eq!(ErrCode::NotFound.to_human_readable(), "not_found");
    }
}