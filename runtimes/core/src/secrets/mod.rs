use std::collections::HashMap;
use std::fmt::Display;
use std::sync::{Arc, OnceLock};

use base64::{engine::general_purpose, Engine as _};

use crate::encore::runtime::v1 as pb;
use encore::runtime::v1::secret_data::{Source, SubPath};
use encore::runtime::v1::SecretData;

use crate::encore;
use crate::encore::runtime::v1::secret_data::Encoding;
use crate::names::EncoreName;

pub struct Manager {
    app_secrets: HashMap<EncoreName, Arc<Secret>>,
}

impl Manager {
    pub fn new(app_secrets: Vec<pb::AppSecret>) -> Self {
        let app_secrets = app_secrets
            .into_iter()
            .filter_map(|s| match s.data {
                Some(data) => Some((s.encore_name.into(), Arc::new(Secret::new(data)))),
                None => None,
            })
            .collect();
        Self { app_secrets }
    }

    pub fn load(&self, data: SecretData) -> Secret {
        Secret::new(data)
    }

    /// Retrieve the secret for the given encore name.
    /// If the secret is not found, returns None.
    pub fn app_secret(&self, name: EncoreName) -> Option<Arc<Secret>> {
        self.app_secrets.get(&name).cloned()
    }
}

pub struct Secret {
    data: SecretData,
    resolved: OnceLock<ResolveResult<Vec<u8>>>,
}

impl Secret {
    fn new(data: SecretData) -> Self {
        Self {
            data,
            resolved: OnceLock::new(),
        }
    }

    pub fn new_for_test(plaintext: &'static str) -> Self {
        Self::new(SecretData {
            source: Some(Source::Embedded(plaintext.as_bytes().to_vec())),
            sub_path: None,
            encoding: Encoding::None as i32,
        })
    }

    pub fn get(&self) -> Result<&[u8], ResolveError> {
        let result = self.resolved.get_or_init(|| resolve(&self.data)).as_deref();
        match result {
            Ok(bytes) => Ok(bytes),
            Err(err) => Err(*err),
        }
    }
}

const BASE64: general_purpose::GeneralPurpose = general_purpose::STANDARD;

#[derive(Debug, Copy, Clone)]
pub enum ResolveError {
    EnvVarNotFound,
    JsonKeyNotFound,
    JsonValueNotString,
    InvalidBase64,
    InvalidJSON,
    InvalidJSONValue,
    InvalidSecretSource,
    UnknownEncoding,
}

impl std::error::Error for ResolveError {}

impl Display for ResolveError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ResolveError::EnvVarNotFound => write!(f, "environment variable not found"),
            ResolveError::JsonKeyNotFound => write!(f, "JSON key not found"),
            ResolveError::JsonValueNotString => write!(f, "JSON value is not a string"),
            ResolveError::InvalidBase64 => write!(f, "invalid base64"),
            ResolveError::InvalidJSON => write!(f, "invalid JSON"),
            ResolveError::InvalidJSONValue => write!(f, "invalid JSON value encoding"),
            ResolveError::InvalidSecretSource => write!(f, "invalid secret source"),
            ResolveError::UnknownEncoding => write!(f, "unknown encoding"),
        }
    }
}

type ResolveResult<T> = Result<T, ResolveError>;

fn resolve(data: &SecretData) -> ResolveResult<Vec<u8>> {
    let value = match &data.source {
        Some(Source::Embedded(data)) => data.clone(),
        Some(Source::Env(name)) => {
            let value = std::env::var(name).map_err(|_| ResolveError::EnvVarNotFound)?;
            value.into_bytes()
        }
        None => Err(ResolveError::InvalidSecretSource)?,
    };

    // Shall we decode this?
    let encoding = Encoding::try_from(data.encoding).map_err(|_| ResolveError::UnknownEncoding)?;
    let value = match encoding {
        Encoding::None => value,
        Encoding::Base64 => BASE64
            .decode(&value)
            .map_err(|_| ResolveError::InvalidBase64)?,
    };

    // Is there a subpath?
    match &data.sub_path {
        None => Ok(value),

        Some(SubPath::JsonKey(json_key)) => {
            // Escape the JSON key since we use gjson.
            let json_key = escape_gjson_key(&json_key);

            let str_value = std::str::from_utf8(&value).map_err(|_| ResolveError::InvalidJSON)?;
            let value = gjson::get(&str_value, &json_key);
            match value.kind() {
                gjson::Kind::String => {
                    // Use the string as-is.
                    Ok(value.str().as_bytes().to_vec())
                }

                gjson::Kind::Object => {
                    // Iterate over the keys to find the first "bytes" or "string" key.
                    let mut result: Option<ResolveResult<Vec<u8>>> = None;
                    let iter = |key: gjson::Value, value: gjson::Value| {
                        match key.str() {
                            "bytes" => {
                                // Decode the bytes from base64.
                                let res = BASE64
                                    .decode(value.str())
                                    .map_err(|_| ResolveError::InvalidBase64);
                                result = Some(res);
                            }
                            "string" => {
                                // Use the string as-is.
                                result = Some(Ok(value.str().as_bytes().to_vec()));
                            }
                            _ => {}
                        }
                        result.is_some()
                    };
                    value.each(iter);
                    result.unwrap_or(Err(ResolveError::InvalidJSONValue))
                }

                gjson::Kind::Null => Err(ResolveError::JsonKeyNotFound),
                _ => Err(ResolveError::JsonValueNotString),
            }
        }
    }
}

fn escape_gjson_key(key: &str) -> String {
    fn is_safe_path_key_char(c: char) -> bool {
        (c >= 'a' && c <= 'z')
            || (c >= 'A' && c <= 'Z')
            || (c >= '0' && c <= '9')
            || c <= ' '
            || c > '~'
            || c == '_'
            || c == '-'
            || c == ':'
    }

    let mut escaped = String::with_capacity(key.len());
    for c in key.chars() {
        if is_safe_path_key_char(c) {
            escaped.push(c);
        } else {
            escaped.push('\\');
            escaped.push(c);
        }
    }
    escaped
}

#[cfg(test)]
mod tests {
    use assert_matches::assert_matches;

    #[test]
    fn test_resolve() {
        use super::*;
        use encore::runtime::v1::{secret_data::Source, SecretData};

        let secret = Secret::new(SecretData {
            source: Some(Source::Embedded(b"hello".to_vec())),
            sub_path: None,
            encoding: Encoding::None as i32,
        });
        assert_eq!(secret.get().unwrap(), b"hello");

        let secret = Secret::new(SecretData {
            source: Some(Source::Embedded(b"aGVsbG8=".to_vec())),
            sub_path: None,
            encoding: Encoding::Base64 as i32,
        });
        assert_eq!(secret.get().unwrap(), b"hello");

        let secret = Secret::new(SecretData {
            source: Some(Source::Embedded(b"aGVsbG8=".to_vec())),
            sub_path: None,
            encoding: Encoding::None as i32,
        });
        assert_eq!(secret.get().unwrap(), b"aGVsbG8=");

        {
            let data = SecretData {
                source: Some(Source::Env("TEST_SECRET".to_string())),
                sub_path: None,
                encoding: Encoding::None as i32,
            };
            let secret = Secret::new(data.clone());
            assert_matches!(secret.get(), Err(ResolveError::EnvVarNotFound));

            std::env::set_var("TEST_SECRET", "hello");
            let secret = Secret::new(data);
            assert_eq!(secret.get().unwrap(), b"hello");
        }

        // Test json_key.
        {
            let secret = Secret::new(SecretData {
                source: Some(Source::Embedded(br#"{"foo": "hello"}"#.to_vec())),
                sub_path: Some(SubPath::JsonKey("foo".to_string())),
                encoding: Encoding::None as i32,
            });
            assert_eq!(secret.get().unwrap(), b"hello");

            let secret = Secret::new(SecretData {
                source: Some(Source::Embedded(
                    br#"{"foo": {"bytes": "aGVsbG8="}}"#.to_vec(),
                )),
                sub_path: Some(SubPath::JsonKey("bar".to_string())),
                encoding: Encoding::None as i32,
            });
            assert_matches!(secret.get(), Err(ResolveError::JsonKeyNotFound));

            let secret = Secret::new(SecretData {
                source: Some(Source::Embedded(
                    br#"{"foo": {"bytes": "aGVsbG8="}}"#.to_vec(),
                )),
                sub_path: Some(SubPath::JsonKey("foo".to_string())),
                encoding: Encoding::None as i32,
            });
            assert_matches!(secret.get().unwrap(), b"hello");
        }
    }
}
