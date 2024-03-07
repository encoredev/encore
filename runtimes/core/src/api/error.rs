use std::fmt::{Debug, Display};
use std::str::FromStr;

use crate::error::{AppError, StackTrace};
use serde::{Deserialize, Serialize};
use serde_with::{DeserializeFromStr, SerializeDisplay};

/// Represents an API Error.
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Error {
    pub code: ErrCode,
    pub message: String,
    pub internal_message: Option<String>,

    #[serde(skip_serializing)]
    pub stack: Option<StackTrace>,
}

impl Error {
    pub fn internal<E>(cause: E) -> Self
    where
        E: Into<anyhow::Error>,
    {
        Self {
            code: ErrCode::Internal,
            message: ErrCode::Internal.default_public_message().into(),
            internal_message: Some(format!("{:#?}", cause.into())),
            stack: None,
        }
    }

    pub fn invalid_argument<S, E>(public_msg: S, cause: E) -> Self
    where
        S: Into<String>,
        E: Into<anyhow::Error>,
    {
        Self {
            code: ErrCode::InvalidArgument,
            message: public_msg.into(),
            internal_message: Some(format!("{:#?}", cause.into())),
            stack: None,
        }
    }

    pub fn not_found<S>(public_msg: S) -> Self
    where
        S: Into<String>,
    {
        Self {
            code: ErrCode::NotFound,
            message: public_msg.into(),
            internal_message: None,
            stack: None,
        }
    }
}

impl Into<AppError> for Error {
    fn into(self) -> AppError {
        AppError::new(self.message)
    }
}

impl Into<AppError> for &Error {
    fn into(self) -> AppError {
        // TODO: capture the JS stack trace for this error
        AppError {
            message: self.message.clone(),
            stack: vec![],
            cause: None,
        }
    }
}

impl std::error::Error for Error {}

impl Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match &self.internal_message {
            Some(msg) => write!(f, "{}", msg),
            None => write!(f, "{}", self.message),
        }
    }
}

/// Represents an API Error.
#[derive(SerializeDisplay, DeserializeFromStr, Debug, Copy, Clone, PartialEq, Eq)]
pub enum ErrCode {
    /// Canceled indicates the operation was canceled (typically by the caller).
    ///
    /// Encore will generate this error code when cancellation is requested.
    Canceled,

    /// Unknown error. An example of where this error may be returned is
    /// if a Status value received from another address space belongs to
    /// an error-space that is not known in this address space. Also
    /// errors raised by APIs that do not return enough error information
    /// may be converted to this error.
    ///
    /// Encore will generate this error code in the above two mentioned cases.
    Unknown,

    /// InvalidArgument indicates client specified an invalid argument.
    /// Note that this differs from FailedPrecondition. It indicates arguments
    /// that are problematic regardless of the state of the system
    /// (e.g., a malformed file name).
    ///
    /// Encore will generate this error code if the request data cannot be parsed.
    InvalidArgument,

    /// DeadlineExceeded means operation expired before completion.
    /// For operations that change the state of the system, this error may be
    /// returned even if the operation has completed successfully. For
    /// example, a successful response from a server could have been delayed
    /// long enough for the deadline to expire.
    ///
    /// Encore will generate this error code when the deadline is exceeded.
    DeadlineExceeded,

    /// NotFound means some requested entity (e.g., file or directory) was
    /// not found.
    ///
    /// Encore will not generate this error code.
    NotFound,

    /// AlreadyExists means an attempt to create an entity failed because one
    /// already exists.
    ///
    /// Encore will not generate this error code.
    AlreadyExists,

    /// PermissionDenied indicates the caller does not have permission to
    /// execute the specified operation. It must not be used for rejections
    /// caused by exhausting some resource (use ResourceExhausted
    /// instead for those errors). It must not be
    /// used if the caller cannot be identified (use Unauthenticated
    /// instead for those errors).
    ///
    /// Encore will not generate this error code.
    PermissionDenied,

    /// ResourceExhausted indicates some resource has been exhausted, perhaps
    /// a per-user quota, or perhaps the entire file system is out of space.
    ///
    /// Encore will generate this error code in out-of-memory and server overload
    /// situations, or when a message is larger than the configured maximum size.
    ResourceExhausted,

    /// FailedPrecondition indicates operation was rejected because the
    /// system is not in a state required for the operation's execution.
    /// For example, directory to be deleted may be non-empty, an rmdir
    /// operation is applied to a non-directory, etc.
    ///
    /// A litmus test that may help a service implementor in deciding
    /// between FailedPrecondition, Aborted, and Unavailable:
    ///  (a) Use Unavailable if the client can retry just the failing call.
    ///  (b) Use Aborted if the client should retry at a higher-level
    ///      (e.g., restarting a read-modify-write sequence).
    ///  (c) Use FailedPrecondition if the client should not retry until
    ///      the system state has been explicitly fixed. E.g., if an "rmdir"
    ///      fails because the directory is non-empty, FailedPrecondition
    ///      should be returned since the client should not retry unless
    ///      they have first fixed up the directory by deleting files from it.
    ///  (d) Use FailedPrecondition if the client performs conditional
    ///      Get/Update/Delete on a resource and the resource on the
    ///      server does not match the condition. E.g., conflicting
    ///      read-modify-write on the same resource.
    ///
    /// Encore will not generate this error code.
    FailedPrecondition,

    /// Aborted indicates the operation was aborted, typically due to a
    /// concurrency issue like sequencer check failures, transaction aborts,
    /// etc.
    ///
    /// See litmus test above for deciding between FailedPrecondition,
    /// Aborted, and Unavailable.
    Aborted,

    /// OutOfRange means operation was attempted past the valid range.
    /// E.g., seeking or reading past end of file.
    ///
    /// Unlike InvalidArgument, this error indicates a problem that may
    /// be fixed if the system state changes. For example, a 32-bit file
    /// system will generate InvalidArgument if asked to read at an
    /// offset that is not in the range [0,2^32-1], but it will generate
    /// OutOfRange if asked to read from an offset past the current
    /// file size.
    ///
    /// There is a fair bit of overlap between FailedPrecondition and
    /// OutOfRange. We recommend using OutOfRange (the more specific
    /// error) when it applies so that callers who are iterating through
    /// a space can easily look for an OutOfRange error to detect when
    /// they are done.
    ///
    /// Encore will not generate this error code.
    OutOfRange,

    /// Unimplemented indicates operation is not implemented or not
    /// supported/enabled in this service.
    ///
    /// Encore will generate this error code when an endpoint does not exist.
    Unimplemented,

    /// Internal errors. Means some invariants expected by underlying
    /// system has been broken. If you see one of these errors,
    /// something is very broken.
    ///
    /// Encore will generate this error code in several internal error conditions.
    Internal,

    /// Unavailable indicates the service is currently unavailable.
    /// This is a most likely a transient condition and may be corrected
    /// by retrying with a backoff. Note that it is not always safe to retry
    /// non-idempotent operations.
    ///
    /// See litmus test above for deciding between FailedPrecondition,
    /// Aborted, and Unavailable.
    ///
    /// Encore will generate this error code in aubrupt shutdown of a server process
    /// or network connection.
    Unavailable,

    /// DataLoss indicates unrecoverable data loss or corruption.
    ///
    /// Encore will not generate this error code.
    DataLoss,

    /// Unauthenticated indicates the request does not have valid
    /// authentication credentials for the operation.
    ///
    /// Encore will generate this error code when the authentication metadata
    /// is invalid or missing, and expects auth handlers to return errors with
    /// this code when the auth token is not valid.
    Unauthenticated,
}

impl ErrCode {
    pub fn default_public_message(&self) -> &'static str {
        match self {
            ErrCode::Canceled => "The operation was canceled.",
            ErrCode::Unknown => "An unknown error occurred.",
            ErrCode::InvalidArgument => "The request is invalid.",
            ErrCode::DeadlineExceeded => "The operation timed out.",
            ErrCode::NotFound => "The requested resource was not found.",
            ErrCode::AlreadyExists => "The resource already exists.",
            ErrCode::PermissionDenied => "The caller does not have permission to execute the specified operation.",
            ErrCode::ResourceExhausted => "The resource has been exhausted.",
            ErrCode::FailedPrecondition => "The operation was rejected because the system is not in a state required for the operation's execution.",
            ErrCode::Aborted => "The operation was aborted.",
            ErrCode::OutOfRange => "The operation was attempted past the valid range.",
            ErrCode::Unimplemented => "The operation is not implemented or not supported/enabled in this service.",
            ErrCode::Internal => "An internal error occurred.",
            ErrCode::Unavailable => "The service is currently unavailable.",
            ErrCode::DataLoss => "Unrecoverable data loss or corruption occurred.",
            ErrCode::Unauthenticated => "The request does not have valid authentication credentials for the operation.",
        }
    }

    pub fn status_code(&self) -> axum::http::StatusCode {
        match self {
            ErrCode::Canceled => axum::http::StatusCode::from_u16(499).unwrap(),
            ErrCode::Unknown => axum::http::StatusCode::INTERNAL_SERVER_ERROR,
            ErrCode::InvalidArgument => axum::http::StatusCode::BAD_REQUEST,
            ErrCode::DeadlineExceeded => axum::http::StatusCode::GATEWAY_TIMEOUT,
            ErrCode::NotFound => axum::http::StatusCode::NOT_FOUND,
            ErrCode::AlreadyExists => axum::http::StatusCode::CONFLICT,
            ErrCode::PermissionDenied => axum::http::StatusCode::FORBIDDEN,
            ErrCode::ResourceExhausted => axum::http::StatusCode::TOO_MANY_REQUESTS,
            ErrCode::FailedPrecondition => axum::http::StatusCode::BAD_REQUEST,
            ErrCode::Aborted => axum::http::StatusCode::CONFLICT,
            ErrCode::OutOfRange => axum::http::StatusCode::BAD_REQUEST,
            ErrCode::Unimplemented => axum::http::StatusCode::NOT_IMPLEMENTED,
            ErrCode::Internal => axum::http::StatusCode::INTERNAL_SERVER_ERROR,
            ErrCode::Unavailable => axum::http::StatusCode::SERVICE_UNAVAILABLE,
            ErrCode::DataLoss => axum::http::StatusCode::INTERNAL_SERVER_ERROR,
            ErrCode::Unauthenticated => axum::http::StatusCode::UNAUTHORIZED,
        }
    }
}

impl Display for ErrCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ErrCode::Canceled => write!(f, "canceled"),
            ErrCode::Unknown => write!(f, "unknown"),
            ErrCode::InvalidArgument => write!(f, "invalid_argument"),
            ErrCode::DeadlineExceeded => write!(f, "deadline_exceeded"),
            ErrCode::NotFound => write!(f, "not_found"),
            ErrCode::AlreadyExists => write!(f, "already_exists"),
            ErrCode::PermissionDenied => write!(f, "permission_denied"),
            ErrCode::ResourceExhausted => write!(f, "resource_exhausted"),
            ErrCode::FailedPrecondition => write!(f, "failed_precondition"),
            ErrCode::Aborted => write!(f, "aborted"),
            ErrCode::OutOfRange => write!(f, "out_of_range"),
            ErrCode::Unimplemented => write!(f, "unimplemented"),
            ErrCode::Internal => write!(f, "internal"),
            ErrCode::Unavailable => write!(f, "unavailable"),
            ErrCode::DataLoss => write!(f, "data_loss"),
            ErrCode::Unauthenticated => write!(f, "unauthenticated"),
        }
    }
}

#[derive(Debug)]
pub struct UnknownErrCode {
    pub code: String,
}

impl Display for UnknownErrCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{} (unknown)", self.code)
    }
}

impl std::error::Error for UnknownErrCode {}

impl FromStr for ErrCode {
    type Err = UnknownErrCode;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "canceled" => Ok(ErrCode::Canceled),
            "unknown" => Ok(ErrCode::Unknown),
            "invalid_argument" => Ok(ErrCode::InvalidArgument),
            "deadline_exceeded" => Ok(ErrCode::DeadlineExceeded),
            "not_found" => Ok(ErrCode::NotFound),
            "already_exists" => Ok(ErrCode::AlreadyExists),
            "permission_denied" => Ok(ErrCode::PermissionDenied),
            "resource_exhausted" => Ok(ErrCode::ResourceExhausted),
            "failed_precondition" => Ok(ErrCode::FailedPrecondition),
            "aborted" => Ok(ErrCode::Aborted),
            "out_of_range" => Ok(ErrCode::OutOfRange),
            "unimplemented" => Ok(ErrCode::Unimplemented),
            "internal" => Ok(ErrCode::Internal),
            "unavailable" => Ok(ErrCode::Unavailable),
            "data_loss" => Ok(ErrCode::DataLoss),
            "unauthenticated" => Ok(ErrCode::Unauthenticated),
            other => Err(UnknownErrCode {
                code: other.to_owned(),
            }),
        }
    }
}

impl Into<axum::http::status::StatusCode> for ErrCode {
    fn into(self) -> axum::http::status::StatusCode {
        self.status_code()
    }
}
