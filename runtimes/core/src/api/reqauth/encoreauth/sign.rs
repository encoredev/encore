use std::fmt::{Display, Formatter};
use std::str::FromStr;
use std::time::SystemTime;

use bytes::{BufMut, BytesMut};
use chrono::{DateTime, SecondsFormat, Utc};
use hmac::{Hmac, Mac};

use crate::api::reqauth::encoreauth::ophash::OperationHash;

const SIGNATURE_VERSION: &str = "ENCORE1";
const _HASH_IMPL: &str = "HMAC-SHA3-256";

// This must match the values of the constants above.
const AUTH_SCHEME: &str = "ENCORE1-HMAC-SHA3-256";

/// Sign creates the authorization headers for a new request.
///
/// The signature algorithm is based on the AWS Signature Version 4 signing process and is valid for 2 minutes
/// from the time the request is signed.
pub fn sign(
    key: (u32, &[u8]),
    app_slug: &str,
    env_name: &str,
    timestamp: SystemTime,
    operation: &OperationHash,
) -> String {
    sign_for_verification(key, app_slug, env_name, timestamp, operation)
}

pub fn sign_for_verification(
    key: (u32, &[u8]),
    app_slug: &str,
    env_name: &str,
    timestamp: SystemTime,
    operation: &OperationHash,
) -> String {
    let credentials = create_credential_string(timestamp, app_slug, env_name, key.0);
    let request_digest = build_request_digest(timestamp, &credentials, operation);
    let signing_key = derive_signing_key(key.1, timestamp, app_slug, env_name).into_bytes();

    let signature = hash_hmac(&signing_key, request_digest.as_bytes()).into_bytes();
    let signature = hex::encode(signature);

    format!(
        "{} cred=\"{}\", op={}, sig={}",
        AUTH_SCHEME,
        credentials,
        operation.as_hex(),
        signature
    )
}

pub struct SignatureComponents {
    pub key_id: u32,
    pub app_slug: String,
    pub env_name: String,
    pub timestamp: SystemTime,
    pub operation_hash: OperationHash,
}

#[derive(Debug)]
pub enum InvalidSignature {
    InvalidAuthorizationHeader,
    InvalidDateHeader,

    InvalidAuthScheme,
    InvalidCredentialString,
    InvalidOperationHash,
    UnknownParameter(String),
}

impl Display for InvalidSignature {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        use InvalidSignature::*;
        match self {
            InvalidAuthorizationHeader => write!(f, "invalid authorization header"),
            InvalidDateHeader => write!(f, "invalid date header"),
            InvalidAuthScheme => write!(f, "invalid auth scheme"),
            InvalidCredentialString => write!(f, "invalid credential string"),
            InvalidOperationHash => write!(f, "invalid operation hash"),
            UnknownParameter(name) => write!(f, "unknown parameter: {}", name),
        }
    }
}

impl std::error::Error for InvalidSignature {}

impl SignatureComponents {
    pub fn parse(authorization: &str, date: &str) -> Result<Self, InvalidSignature> {
        let http_date =
            httpdate::parse_http_date(date).map_err(|_| InvalidSignature::InvalidDateHeader)?;
        let date_str = <DateTime<Utc>>::from(http_date)
            .format("%Y%m%d")
            .to_string();

        let mut auth_components = authorization.splitn(2, ' ');
        let scheme = auth_components
            .next()
            .ok_or(InvalidSignature::InvalidAuthorizationHeader)?;
        if scheme != AUTH_SCHEME {
            return Err(InvalidSignature::InvalidAuthScheme);
        }

        let parameters = auth_components
            .next()
            .ok_or(InvalidSignature::InvalidAuthorizationHeader)?;

        let mut op_hash = None;
        let mut creds = None;
        for param in parameters.split(", ") {
            let (name, value) = param
                .split_once('=')
                .ok_or(InvalidSignature::InvalidAuthorizationHeader)?;
            match name {
                "cred" => {
                    if creds.is_some() {
                        return Err(InvalidSignature::InvalidAuthorizationHeader);
                    }

                    // Unquote the value.
                    let value = value
                        .strip_prefix('"')
                        .and_then(|v| v.strip_suffix('"'))
                        .ok_or(InvalidSignature::InvalidCredentialString)?;

                    let parsed = parse_credential_string(value)?;
                    if parsed.date != date_str {
                        return Err(InvalidSignature::InvalidDateHeader);
                    }
                    creds = Some(parsed);
                }
                "op" => {
                    if op_hash.is_some() {
                        return Err(InvalidSignature::InvalidAuthorizationHeader);
                    }
                    op_hash = Some(
                        OperationHash::from_str(value)
                            .map_err(|_| InvalidSignature::InvalidOperationHash)?,
                    );
                }
                "sig" => {
                    // No need to do anything with the signature
                }
                _ => {
                    return Err(InvalidSignature::UnknownParameter(name.to_string()));
                }
            }
        }

        let Some(creds) = creds else {
            return Err(InvalidSignature::InvalidAuthorizationHeader);
        };

        Ok(Self {
            key_id: creds.key_id,
            app_slug: creds.app_slug,
            env_name: creds.env_name,
            timestamp: http_date,
            operation_hash: op_hash.ok_or(InvalidSignature::InvalidAuthorizationHeader)?,
        })
    }
}

fn create_credential_string(
    timestamp: SystemTime,
    app_slug: &str,
    env_name: &str,
    key_id: u32,
) -> String {
    let dt: DateTime<Utc> = timestamp.into();
    let date = dt.format("%Y%m%d");
    format!("{}/{}/{}/{}", date, app_slug, env_name, key_id)
}

struct CredentialComponents {
    key_id: u32,
    app_slug: String,
    env_name: String,
    date: String,
}

fn parse_credential_string(s: &str) -> Result<CredentialComponents, InvalidSignature> {
    let mut parts = s.split('/');
    let date = parts
        .next()
        .ok_or(InvalidSignature::InvalidCredentialString)?
        .to_string();
    let app_slug = parts
        .next()
        .ok_or(InvalidSignature::InvalidCredentialString)?
        .to_string();
    let env_name = parts
        .next()
        .ok_or(InvalidSignature::InvalidCredentialString)?
        .to_string();
    let key_id = parts
        .next()
        .ok_or(InvalidSignature::InvalidCredentialString)?
        .parse::<u32>()
        .map_err(|_| InvalidSignature::InvalidCredentialString)?;

    if parts.next().is_some() {
        return Err(InvalidSignature::InvalidCredentialString);
    }

    Ok(CredentialComponents {
        key_id,
        app_slug,
        env_name,
        date,
    })
}

/// The request digest represents the request that we want to make
/// and is the data we will sign.
///
/// It is a newline separated string of the following:
///
/// - The auth scheme being used.
/// - Timestamp in RFC3339 format.
/// - App slug and environment name.
/// - The operation hash.
fn build_request_digest(
    timestamp: SystemTime,
    credentials: &str,
    operation: &OperationHash,
) -> String {
    let dt: DateTime<Utc> = timestamp.into();
    let timestamp = dt.to_rfc3339_opts(SecondsFormat::Secs, true);
    format!(
        "{}\n{}\n{}\n{}",
        AUTH_SCHEME,
        timestamp,
        credentials,
        operation.as_hex(),
    )
}

/// The signing key is a HMAC-SHA3-256 hash of the following, where each component is hashed in order,
/// and the result of each hash is used as the key for the next hash:
/// - Signature version.
/// - The shared secret between the app and Encore.
/// - The date in YYYYMMDD format.
/// - The application slug.
/// - The environment name.
/// - The string "encore_request".
fn derive_signing_key(
    key_data: &[u8],
    timestamp: SystemTime,
    app_slug: &str,
    env_name: &str,
) -> hmac::digest::CtOutput<HmacSha3_256> {
    let base_key = {
        let mut bytes = BytesMut::with_capacity(SIGNATURE_VERSION.len() + key_data.len());
        bytes.put_slice(SIGNATURE_VERSION.as_bytes());
        bytes.put_slice(key_data);
        bytes.to_vec()
    };

    let date_key = {
        let dt: DateTime<Utc> = timestamp.into();
        let timestamp = dt.format("%Y%m%d").to_string();
        hash_hmac(&base_key, timestamp.as_bytes()).into_bytes()
    };

    let app_key = hash_hmac(&date_key, app_slug.as_bytes()).into_bytes();
    let env_key = hash_hmac(&app_key, env_name.as_bytes()).into_bytes();

    hash_hmac(&env_key, b"encore_request")
}

type HmacSha3_256 = Hmac<sha3::Sha3_256>;

fn hash_hmac(key: &[u8], data: &[u8]) -> hmac::digest::CtOutput<HmacSha3_256> {
    HmacSha3_256::new_from_slice(key)
        .expect("hmac can accept keys of any size")
        .chain_update(data)
        .finalize()
}
