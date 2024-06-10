use crate::{encore, secrets};
use anyhow::Context;
use base64::engine::general_purpose;
use base64::Engine;
use encore::runtime::v1 as pb;
use hmac::Mac;
use std::fmt::Display;
use std::time::SystemTime;

pub struct RequestValidator {
    keys: Box<[SigningKey]>,
}

impl std::fmt::Debug for RequestValidator {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("RequestValidator").finish()
    }
}

pub struct ValidationData<'a> {
    pub request_path: &'a str,
    pub date_header: &'a str,
    pub x_encore_auth_header: &'a str,
}

/// A seal of approval is a record that the request originated from the Encore Platform.
#[derive(Debug)]
pub struct SealOfApproval;

impl RequestValidator {
    pub fn new(secrets: &secrets::Manager, keys: Vec<pb::EncoreAuthKey>) -> Self {
        let keys = keys
            .into_iter()
            .filter_map(|k| match k.data {
                Some(data) => Some(SigningKey {
                    id: k.id,
                    data: secrets.load(data),
                }),
                None => None,
            })
            .collect();
        Self { keys }
    }

    pub fn validate_platform_request(
        &self,
        req: &ValidationData,
    ) -> Result<SealOfApproval, ValidationError> {
        let decoded_auth_header = BASE64
            .decode(req.x_encore_auth_header.as_bytes())
            .map_err(|_| ValidationError::InvalidMac)?;

        // Pull out key ID from hmac prefix
        const KEY_ID_LEN: usize = 4;
        if decoded_auth_header.len() < KEY_ID_LEN {
            return Err(ValidationError::InvalidMac);
        }

        let key_id = u32::from_be_bytes(decoded_auth_header[..KEY_ID_LEN].try_into().unwrap());
        let received_mac = &decoded_auth_header[KEY_ID_LEN..];
        for k in self.keys.iter() {
            if k.id == key_id {
                let secret_data = k.data.get().map_err(ValidationError::SecretResolve)?;
                return check_auth_key(secret_data, req, received_mac);
            }
        }

        Err(ValidationError::UnknownMacKey)
    }

    pub fn sign_outgoing_request(&self, req: &mut reqwest::Request) -> anyhow::Result<()> {
        let date_str = req
            .headers_mut()
            .entry(reqwest::header::DATE)
            .or_insert_with(|| {
                let date_str = httpdate::fmt_http_date(SystemTime::now());
                date_str.parse().unwrap()
            });

        let key = &self.keys[0];
        let key_data = key.data.get().context("unable to resolve signing key")?;
        let mut mac = hmac::Hmac::<sha2::Sha256>::new_from_slice(key_data).unwrap();
        mac.update(date_str.as_bytes());
        mac.update(b"\x00");
        mac.update(req.url().path().as_bytes());

        let mac_bytes = mac.finalize().into_bytes();
        let combined = [key.id.to_be_bytes().as_slice(), mac_bytes.as_slice()].concat();
        let auth_header = BASE64.encode(combined);
        req.headers_mut().insert(
            reqwest::header::HeaderName::from_static("x-encore-auth"),
            reqwest::header::HeaderValue::from_str(&auth_header).context("invalid auth header")?,
        );
        Ok(())
    }
}

struct SigningKey {
    id: u32,
    data: secrets::Secret,
}

const BASE64: general_purpose::GeneralPurpose = general_purpose::STANDARD_NO_PAD;

fn check_auth_key(
    decryption_key: &[u8],
    req: &ValidationData,
    received_mac: &[u8],
) -> Result<SealOfApproval, ValidationError> {
    let request_date = httpdate::parse_http_date(req.date_header)
        .map_err(|_| ValidationError::InvalidDateHeader)?;

    let now = SystemTime::now();
    let diff = now
        .duration_since(request_date)
        .unwrap_or_else(|e| e.duration());

    const THRESHOLD: u64 = 15 * 60;
    if diff.as_secs() > THRESHOLD {
        return Err(ValidationError::TimeSkew);
    }

    // Compute the MAC.
    type HmacSha256 = hmac::Hmac<sha2::Sha256>;
    let mut computed_mac =
        HmacSha256::new_from_slice(decryption_key).map_err(|_| ValidationError::InvalidMacKey)?;
    computed_mac.update(req.date_header.as_bytes());
    computed_mac.update(b"\x00");
    computed_mac.update(req.request_path.as_bytes());

    computed_mac
        .verify_slice(received_mac)
        .map_err(|_| ValidationError::InvalidMac)?;

    Ok(SealOfApproval)
}

#[derive(Debug)]
pub enum ValidationError {
    InvalidMac,
    UnknownMacKey,
    InvalidMacKey,
    InvalidDateHeader,
    TimeSkew,
    SecretResolve(secrets::ResolveError),
}

impl Display for ValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ValidationError::InvalidMac => write!(f, "invalid mac"),
            ValidationError::UnknownMacKey => write!(f, "unknown mac key"),
            ValidationError::InvalidMacKey => write!(f, "invalid mac key"),
            ValidationError::InvalidDateHeader => write!(f, "invalid or missing date header"),
            ValidationError::TimeSkew => write!(f, "time skew"),
            ValidationError::SecretResolve(e) => {
                write!(f, "resolve secret: {}", e)
            }
        }
    }
}

impl std::error::Error for ValidationError {}
