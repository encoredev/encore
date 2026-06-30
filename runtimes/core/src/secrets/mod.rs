use std::collections::HashMap;
use std::fmt::Display;
use std::sync::{Arc, OnceLock};

use std::io::Read as _;

use anyhow::Context;
use base64::{engine::general_purpose, Engine as _};
use flate2::read::GzDecoder;

use crate::encore::runtime::v1 as pb;
use encore::runtime::v1::secret_data::{Source, SubPath};
use encore::runtime::v1::SecretData;

use crate::encore;
use crate::encore::runtime::v1::secret_data::Encoding;
use crate::names::EncoreName;

mod gcpsm;
mod provider;

pub use provider::{Provider, ProviderError};

pub struct Manager {
    app_secrets: HashMap<EncoreName, Arc<Secret>>,
}

impl Manager {
    /// Build the secrets Manager.
    ///
    /// Provider-backed app secrets are eagerly resolved here so that
    /// `Secret::get` stays sync at call time. A failure for any provider-backed
    /// secret aborts startup — this matches the behavior of missing-secret in
    /// the Go runtime and surfaces backend outages immediately rather than at
    /// random request time.
    pub async fn new(
        app_secrets: Vec<pb::AppSecret>,
        provider_defs: Vec<pb::SecretProvider>,
    ) -> anyhow::Result<Self> {
        let providers = build_providers(provider_defs).await?;

        let mut app_secret_map = HashMap::with_capacity(app_secrets.len());
        for s in app_secrets {
            let Some(data) = s.data else { continue };
            let secret = Arc::new(Secret::new(data.clone()));

            if let Some(Source::Provider(p_ref)) = &data.source {
                let prov = providers.get(&p_ref.provider_rid).ok_or_else(|| {
                    anyhow::anyhow!(
                        "app secret {} references unknown provider {}",
                        s.encore_name,
                        p_ref.provider_rid
                    )
                })?;
                let raw = prov
                    .load(&p_ref.id, &p_ref.version)
                    .await
                    .with_context(|| format!("fetch app secret {}", s.encore_name))?;
                let resolved = post_process(raw, &data);
                // OnceLock::set returns Err only if already set, which can't
                // happen here since we just constructed the Secret.
                let _ = secret.resolved.set(resolved);
            }

            app_secret_map.insert(s.encore_name.into(), secret);
        }

        Ok(Self {
            app_secrets: app_secret_map,
        })
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

async fn build_providers(
    defs: Vec<pb::SecretProvider>,
) -> anyhow::Result<HashMap<String, Arc<dyn Provider>>> {
    let mut out: HashMap<String, Arc<dyn Provider>> = HashMap::with_capacity(defs.len());
    for def in defs {
        let rid = def.rid.clone();
        let name = def.encore_name.clone();
        let p: Arc<dyn Provider> = match def.provider {
            Some(pb::secret_provider::Provider::GcpSm(cfg)) => Arc::new(
                gcpsm::GcpProvider::new(cfg.project_id)
                    .await
                    .with_context(|| format!("init secret provider {name}"))?,
            ),
            None => anyhow::bail!("secret provider {name} has no provider configured"),
        };
        out.insert(rid, p);
    }
    Ok(out)
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
    /// A secret value uses the `envref:` indirection but one of the referenced
    /// chunk env vars is not set.
    EnvRefChunkNotFound,
    JsonKeyNotFound,
    JsonValueNotString,
    InvalidBase64,
    InvalidJSON,
    InvalidJSONValue,
    InvalidGzip,
    InvalidSecretSource,
    UnknownEncoding,
    /// Source is a provider reference but no value was pre-resolved.
    /// Provider-backed SecretData is only supported for app_secrets, which are
    /// fetched at Manager startup.
    ProviderNotResolved,
}

impl std::error::Error for ResolveError {}

impl Display for ResolveError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ResolveError::EnvVarNotFound => write!(f, "environment variable not found"),
            ResolveError::EnvRefChunkNotFound => {
                write!(f, "referenced envref chunk environment variable not found")
            }
            ResolveError::JsonKeyNotFound => write!(f, "JSON key not found"),
            ResolveError::JsonValueNotString => write!(f, "JSON value is not a string"),
            ResolveError::InvalidBase64 => write!(f, "invalid base64"),
            ResolveError::InvalidGzip => write!(f, "invalid gzip data"),
            ResolveError::InvalidJSON => write!(f, "invalid JSON"),
            ResolveError::InvalidJSONValue => write!(f, "invalid JSON value encoding"),
            ResolveError::InvalidSecretSource => write!(f, "invalid secret source"),
            ResolveError::UnknownEncoding => write!(f, "unknown encoding"),
            ResolveError::ProviderNotResolved => {
                write!(f, "provider-backed secret not pre-resolved")
            }
        }
    }
}

type ResolveResult<T> = Result<T, ResolveError>;

fn resolve(data: &SecretData) -> ResolveResult<Vec<u8>> {
    let value = match &data.source {
        Some(Source::Embedded(data)) => data.clone(),
        Some(Source::Env(name)) => resolve_env_source(name)?,
        Some(Source::Provider(_)) => return Err(ResolveError::ProviderNotResolved),
        None => Err(ResolveError::InvalidSecretSource)?,
    };

    post_process(value, data)
}

/// Marker prefix in an env var value indicating its content is split across
/// multiple env vars. The remainder is a comma-separated list of env var names
/// whose values are concatenated, in order, to form the secret value.
///
/// This works around per-variable size limits on some platforms (e.g. AWS
/// Lambda's 4KB total env budget). The producer splits the *encoded* value
/// (post-base64/gzip) into N chunks and sets:
///   SECRET      = "envref:SECRET_0,SECRET_1,SECRET_2"
///   SECRET_0..N = <chunk>
///
/// Reassembly happens here, before `post_process`, so decoding still operates on
/// the complete value. The indirection is non-recursive: a chunk whose value
/// itself starts with `envref:` is treated as literal content.
const ENV_REF_PREFIX: &str = "envref:";

/// Resolve an env-var-backed secret value, expanding the `envref:` indirection
/// (see [`ENV_REF_PREFIX`]) if present.
fn resolve_env_source(name: &str) -> ResolveResult<Vec<u8>> {
    let value = std::env::var(name).map_err(|_| ResolveError::EnvVarNotFound)?;

    // Fast path: plain value, no indirection.
    let Some(refs) = value.strip_prefix(ENV_REF_PREFIX) else {
        return Ok(value.into_bytes());
    };

    let mut out = Vec::new();
    for part in refs.split(',') {
        let part = part.trim();
        if part.is_empty() {
            continue; // tolerate trailing comma / stray whitespace
        }
        let chunk = std::env::var(part).map_err(|_| ResolveError::EnvRefChunkNotFound)?;
        out.extend_from_slice(chunk.as_bytes());
    }
    Ok(out)
}

/// Apply encoding decode + sub_path extraction to a freshly fetched value.
/// Shared between the sync resolve path (embedded/env) and the eager provider
/// fetch path.
fn post_process(value: Vec<u8>, data: &SecretData) -> ResolveResult<Vec<u8>> {
    let encoding = Encoding::try_from(data.encoding).map_err(|_| ResolveError::UnknownEncoding)?;
    let value = match encoding {
        Encoding::None => value,
        Encoding::Gzip => {
            let compressed = BASE64
                .decode(value)
                .map_err(|_| ResolveError::InvalidBase64)?;
            let mut decoder = GzDecoder::new(&compressed[..]);
            let mut decompressed = Vec::new();
            decoder
                .read_to_end(&mut decompressed)
                .map_err(|_| ResolveError::InvalidGzip)?;
            decompressed
        }
        Encoding::Base64 => BASE64
            .decode(&value)
            .map_err(|_| ResolveError::InvalidBase64)?,
    };

    match &data.sub_path {
        None => Ok(value),

        Some(SubPath::JsonKey(json_key)) => {
            // Escape the JSON key since we use gjson.
            let json_key = escape_gjson_key(json_key);

            let str_value = std::str::from_utf8(&value).map_err(|_| ResolveError::InvalidJSON)?;
            let value = gjson::get(str_value, &json_key);
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
        c.is_ascii_lowercase()
            || c.is_ascii_uppercase()
            || c.is_ascii_digit()
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

        // Test envref: split across multiple env vars. The chunks are
        // reassembled before decoding, so base64 split mid-string still works:
        // "aGVs" + "bG8=" == "aGVsbG8=" which base64-decodes to "hello".
        {
            std::env::set_var("SPLIT_0", "aGVs");
            std::env::set_var("SPLIT_1", "bG8=");
            std::env::set_var("SPLIT", "envref:SPLIT_0, SPLIT_1");

            let secret = Secret::new(SecretData {
                source: Some(Source::Env("SPLIT".to_string())),
                sub_path: None,
                encoding: Encoding::Base64 as i32,
            });
            assert_eq!(secret.get().unwrap(), b"hello");

            // A missing chunk reports the distinct error variant.
            std::env::set_var("SPLIT_MISSING", "envref:SPLIT_0,NOPE");
            let secret = Secret::new(SecretData {
                source: Some(Source::Env("SPLIT_MISSING".to_string())),
                sub_path: None,
                encoding: Encoding::None as i32,
            });
            assert_matches!(secret.get(), Err(ResolveError::EnvRefChunkNotFound));
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

    #[test]
    fn provider_source_without_resolution_errors() {
        use super::*;
        use encore::runtime::v1::secret_data::{ProviderRef, Source};

        let secret = Secret::new(SecretData {
            source: Some(Source::Provider(ProviderRef {
                provider_rid: "rid".into(),
                id: "id".into(),
                version: String::new(),
            })),
            sub_path: None,
            encoding: Encoding::None as i32,
        });
        assert_matches!(secret.get(), Err(ResolveError::ProviderNotResolved));
    }

    #[tokio::test]
    async fn provider_backed_secret_pre_resolves_to_cache() {
        use super::*;
        use async_trait::async_trait;
        use encore::runtime::v1::secret_data::{ProviderRef, Source};
        use std::sync::atomic::{AtomicUsize, Ordering};

        #[derive(Debug, Default)]
        struct FakeProvider {
            calls: AtomicUsize,
        }
        #[async_trait]
        impl Provider for FakeProvider {
            async fn load(&self, _id: &str, _version: &str) -> Result<Vec<u8>, ProviderError> {
                self.calls.fetch_add(1, Ordering::SeqCst);
                Ok(b"resolved-value".to_vec())
            }
        }

        let fake = Arc::new(FakeProvider::default());

        let data = SecretData {
            source: Some(Source::Provider(ProviderRef {
                provider_rid: "rid".into(),
                id: "id".into(),
                version: String::new(),
            })),
            sub_path: None,
            encoding: Encoding::None as i32,
        };
        let secret = Arc::new(Secret::new(data.clone()));

        // Simulate what Manager::new does for provider-backed app_secrets:
        // call the provider once, pre-populate the OnceLock.
        let raw = fake.load("id", "").await.unwrap();
        let _ = secret.resolved.set(post_process(raw, &data));

        // get() returns the pre-resolved value synchronously and never
        // touches the provider again.
        assert_eq!(secret.get().unwrap(), b"resolved-value");
        assert_eq!(secret.get().unwrap(), b"resolved-value");
        assert_eq!(fake.calls.load(Ordering::SeqCst), 1);
    }
}
