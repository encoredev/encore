use std::fmt::{Debug, Display};
use std::time::SystemTime;

use anyhow::Context;
use sha3::digest::Digest;
use subtle::ConstantTimeEq;

use crate::api::reqauth::encoreauth;
use crate::api::reqauth::encoreauth::{OperationHash, SignatureComponents};
use crate::api::reqauth::meta::{MetaKey, MetaMap, MetaMapMut};
use crate::secrets;
use crate::secrets::Secret;

pub trait ServiceAuthMethod: Debug + Send + Sync + 'static {
    fn name(&self) -> &'static str;
    fn sign(&self, headers: &mut reqwest::header::HeaderMap, now: SystemTime)
        -> anyhow::Result<()>;
    fn verify(
        &self,
        headers: &axum::http::header::HeaderMap,
        now: SystemTime,
    ) -> Result<(), VerifyError>;
}

#[derive(Debug)]
pub struct Noop;

impl ServiceAuthMethod for Noop {
    fn name(&self) -> &'static str {
        "noop"
    }

    fn sign(
        &self,
        _headers: &mut reqwest::header::HeaderMap,
        _now: SystemTime,
    ) -> anyhow::Result<()> {
        Ok(())
    }

    fn verify(
        &self,
        _headers: &axum::http::header::HeaderMap,
        _now: SystemTime,
    ) -> Result<(), VerifyError> {
        Ok(())
    }
}

pub struct EncoreAuthKey {
    pub key_id: u32,
    pub data: Secret,
}

pub struct EncoreAuth {
    app_slug: String,
    env_name: String,
    keys: Vec<EncoreAuthKey>,
    latest_idx: usize, // index into keys
}

impl Debug for EncoreAuth {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("EncoreAuth")
            .field("app_slug", &self.app_slug)
            .field("env_name", &self.env_name)
            .finish()
    }
}

impl EncoreAuth {
    pub fn new(app_slug: String, env_name: String, keys: Vec<EncoreAuthKey>) -> Self {
        if keys.is_empty() {
            panic!("auth keys must not be empty");
        }

        let latest_idx = {
            let mut max_id = keys[0].key_id;
            let mut max_idx = 0;
            for (idx, k) in keys.iter().enumerate() {
                if k.key_id > max_id {
                    max_idx = idx;
                    max_id = k.key_id;
                }
            }
            max_idx
        };

        Self {
            app_slug,
            env_name,
            keys,
            latest_idx,
        }
    }
}

#[derive(Debug)]
pub enum VerifyError {
    NoAuthorizationHeader,
    NoDateHeader,
    InvalidHeader(encoreauth::InvalidSignature),
    SignatureMismatch,
    DateSkew,
    UnknownKey,
    ResolveKeyData(secrets::ResolveError),
}

impl Display for VerifyError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        use VerifyError::*;
        match self {
            NoAuthorizationHeader => write!(f, "no authorization header"),
            NoDateHeader => write!(f, "no date header"),
            InvalidHeader(e) => write!(f, "invalid header: {}", e),
            SignatureMismatch => write!(f, "signature mismatch"),
            DateSkew => write!(f, "date skew"),
            UnknownKey => write!(f, "unknown key"),
            ResolveKeyData(e) => write!(f, "unable to resolve secret key data: {}", e),
        }
    }
}

impl std::error::Error for VerifyError {}

impl ServiceAuthMethod for EncoreAuth {
    fn name(&self) -> &'static str {
        "encore-auth"
    }

    fn sign(
        &self,
        headers: &mut reqwest::header::HeaderMap,
        now: SystemTime,
    ) -> anyhow::Result<()> {
        let op_hash = self.build_op_hash(headers);

        let key = &self.keys[self.latest_idx];
        let key_data = key.data.get().context("unable to resolve auth key data")?;

        let hash = encoreauth::sign(
            (key.key_id, key_data),
            &self.app_slug,
            &self.env_name,
            now,
            &op_hash,
        );

        headers
            .set(MetaKey::SvcAuthEncoreAuthHash, hash)
            .context("set auth hash header")?;
        headers
            .set(MetaKey::SvcAuthEncoreAuthDate, httpdate::fmt_http_date(now))
            .context("set auth date header")?;

        Ok(())
    }

    fn verify(
        &self,
        headers: &axum::http::header::HeaderMap,
        now: SystemTime,
    ) -> Result<(), VerifyError> {
        let auth_header = headers
            .get_meta(MetaKey::SvcAuthEncoreAuthHash)
            .ok_or(VerifyError::NoAuthorizationHeader)?;
        let date_header = headers
            .get_meta(MetaKey::SvcAuthEncoreAuthDate)
            .ok_or(VerifyError::NoDateHeader)?;

        let components = SignatureComponents::parse(auth_header, date_header)
            .map_err(|e| VerifyError::InvalidHeader(e))?;

        let diff = now
            .duration_since(components.timestamp)
            .unwrap_or_else(|e| e.duration());
        if diff.as_secs() > 120 {
            return Err(VerifyError::DateSkew);
        }

        let key = self
            .keys
            .iter()
            .find(|k| k.key_id == components.key_id)
            .ok_or(VerifyError::UnknownKey)?;

        let key_data = key.data.get().map_err(|e| VerifyError::ResolveKeyData(e))?;
        let expected_signature = encoreauth::sign_for_verification(
            (key.key_id, key_data),
            &components.app_slug,
            &components.env_name,
            components.timestamp,
            &components.operation_hash,
        );

        let signature_match: bool = expected_signature
            .as_bytes()
            .ct_eq(auth_header.as_bytes())
            .into();
        if !signature_match {
            return Err(VerifyError::SignatureMismatch);
        }

        let expected_op_hash = self.build_op_hash(headers);
        if !expected_op_hash.ct_eq(&components.operation_hash) {
            return Err(VerifyError::SignatureMismatch);
        }

        Ok(())
    }
}

impl EncoreAuth {
    fn build_op_hash<R: MetaMap>(&self, req: &R) -> OperationHash {
        // Build a deterministic hash of the meta keys and values.
        let mut hash = <sha3::Sha3_256 as Digest>::new();
        for key in req.sorted_meta_keys() {
            use MetaKey::*;
            match key {
                SvcAuthMethod | SvcAuthEncoreAuthHash | SvcAuthEncoreAuthDate => {
                    // Skip these headers, as they are part of the auth mechanism itself.
                }

                TraceParent | TraceState => {
                    // Skip these headers, as they are part of the tracing mechanism and could be changed
                    // by things like load balancers.
                }

                XCorrelationId | Version | UserId | UserData | Caller | Callee => {
                    // Read all values for this key, and sort them.
                    let mut values = req.meta_values(key).collect::<Vec<_>>();
                    values.sort();

                    for value in values {
                        hash.update(key.header_key());
                        hash.update(b"=");
                        hash.update(value.as_bytes());
                        hash.update(b"\n");
                    }
                }
            }
        }

        let payload = hash.finalize();
        OperationHash::new(
            "internal-api".as_bytes(),
            "call".as_bytes(),
            Some(payload.as_slice()),
            std::iter::empty(),
        )
    }
}

#[cfg(test)]
mod tests {
    use crate::api::schema::AsStr;

    use super::*;

    fn metas<R: MetaMap>(req: &R) -> Vec<(MetaKey, Vec<String>)> {
        let keys = req.sorted_meta_keys();
        keys.into_iter()
            .map(|k| {
                (
                    k,
                    req.meta_values(k)
                        .map(|s| s.to_string())
                        .collect::<Vec<_>>(),
                )
            })
            .collect()
    }

    fn convert_header_map(src: reqwest::header::HeaderMap) -> axum::http::HeaderMap {
        let mut dst = axum::http::HeaderMap::new();
        for entry in src.iter() {
            let key: axum::http::HeaderName = entry.0.as_str().parse().unwrap();
            let value = entry.1.to_str().unwrap().parse().unwrap();
            dst.insert(key, value);
        }
        dst
    }

    #[test]
    fn test_encore_auth() -> anyhow::Result<()> {
        let mut headers = reqwest::header::HeaderMap::new();
        let auth = EncoreAuth {
            app_slug: "app".into(),
            env_name: "env".into(),
            keys: vec![EncoreAuthKey {
                key_id: 123,
                data: Secret::new_for_test("secret data"),
            }],
            latest_idx: 0,
        };

        let now = SystemTime::UNIX_EPOCH + std::time::Duration::from_secs(1234567890);
        auth.sign(&mut headers, now)
            .context("unable to sign request")?;

        let out_headers = convert_header_map(headers);
        auth.verify(&out_headers, now)
            .context("unable to verify request")?;

        assert_eq!(
            metas(&out_headers),
            vec![
                (
                    MetaKey::SvcAuthEncoreAuthDate,
                    vec!["Fri, 13 Feb 2009 23:31:30 GMT".to_string()]
                ),
                (MetaKey::SvcAuthEncoreAuthHash, vec![r#"ENCORE1-HMAC-SHA3-256 cred="20090213/app/env/123", op=f3c70a419394ce9d56efafad2208154b92c8596d7396b3a2b4ea7fd925d28dc2, sig=fc0c88b47c13d999353ecc8681d91d9c03209a1f05583b92d84e429fedfe387a"#.to_string()])
            ]
        );

        Ok(())
    }
}
